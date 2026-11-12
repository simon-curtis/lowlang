package hot

import (
	"context"
	"debug/elf"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"lowlang/internal/amd64"
	"lowlang/internal/ast"
	"lowlang/internal/compiler"
	"lowlang/internal/ir"
	"lowlang/internal/target"
)

type child struct {
	cmd    *exec.Cmd
	done   chan error
	exited atomic.Bool
}

type Session struct {
	path       string
	sources    []string
	stdout     io.Writer
	stderr     io.Writer
	dir        string
	child      *child
	binary     string
	symbols    map[string]uint64
	table      map[string]int
	funcs      map[string]string
	signatures map[string]string
	layout     string
	libraries  []string
	cSymbols   []string
	next       uint64
	builds     int
}

func New(path string, stdout, stderr io.Writer) (*Session, error) {
	if err := checkHost(); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "low-dev-")
	if err != nil {
		return nil, err
	}
	return &Session{path: path, stdout: stdout, stderr: stderr, dir: dir}, nil
}

func (s *Session) Close() error {
	s.stop()
	return os.RemoveAll(s.dir)
}

func (s *Session) logf(format string, args ...any) {
	fmt.Fprintf(s.stderr, "low: %s\n", fmt.Sprintf(format, args...))
}

func (s *Session) Run(ctx context.Context) error {
	if err := s.Reload(ctx); err != nil {
		return err
	}
	last := s.stamp()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-s.exited():
			s.child = nil
			s.logf("program exited: %s", describeExit(err))
		case <-ticker.C:
			if stamp := s.stamp(); stamp != last {
				last = stamp
				if err := s.Reload(ctx); err != nil {
					return err
				}
			}
		}
	}
}

func describeExit(err error) string {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.String()
	}
	if err != nil {
		return err.Error()
	}
	return "exit status 0"
}

func (s *Session) exited() <-chan error {
	if s.child == nil {
		return nil
	}
	return s.child.done
}

func (s *Session) stamp() uint64 {
	h := fnv.New64a()
	dirs := map[string]bool{}
	for _, path := range s.watched() {
		h.Write([]byte(path))
		if src, err := os.ReadFile(path); err == nil {
			h.Write(src)
		}
		dirs[filepath.Dir(path)] = true
	}
	for _, dir := range slices.Sorted(maps.Keys(dirs)) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			h.Write([]byte(e.Name()))
		}
	}
	return h.Sum64()
}

func (s *Session) watched() []string {
	if len(s.sources) > 0 {
		return s.sources
	}
	entries, err := os.ReadDir(s.path)
	if err != nil {
		return []string{s.path}
	}
	var paths []string
	for _, e := range entries {
		paths = append(paths, filepath.Join(s.path, e.Name()))
	}
	return paths
}

func (s *Session) Reload(ctx context.Context) error {
	unit, err := compiler.AnalyseDev(s.path, target.Linux)
	if err != nil {
		s.logf("%v", err)
		return nil
	}
	s.sources = unit.Sources
	if s.child == nil {
		return s.start(ctx, unit, "started")
	}
	if reason := s.restartReason(unit); reason != "" {
		return s.restart(ctx, unit, reason)
	}
	table := s.extend(unit.Program)
	if len(table) > amd64.TableSize {
		return s.restart(ctx, unit, "function table full")
	}
	out := amd64.Generate(unit.Program, unit.Platform, table)
	if out.Funcs["main"] != s.funcs["main"] {
		return s.restart(ctx, unit, "main changed")
	}
	changed := changedFuncs(s.funcs, out.Funcs)
	if len(changed) == 0 {
		s.logf("no code changes")
		return nil
	}
	began := time.Now()
	if err := s.patch(ctx, unit, table, changed); err != nil {
		return s.restart(ctx, unit, fmt.Sprintf("reload failed: %v", err))
	}
	s.table, s.funcs = table, out.Funcs
	s.signatures = signatures(unit.Files)
	s.logf("reloaded %s in %s", strings.Join(changed, ", "), time.Since(began).Round(time.Millisecond))
	return nil
}

func (s *Session) restartReason(unit *compiler.Unit) string {
	if !slices.Equal(unit.Program.Libraries(), s.libraries) {
		return "linked libraries changed"
	}
	if !slices.Equal(unit.Program.CSymbols, s.cSymbols) {
		return "called C functions changed"
	}
	if layout(unit.Files) != s.layout {
		return "type layout changed"
	}
	for name, sig := range signatures(unit.Files) {
		if old, ok := s.signatures[name]; ok && old != sig {
			return fmt.Sprintf("signature of %s changed", name)
		}
	}
	return ""
}

func (s *Session) extend(prog *ir.Program) map[string]int {
	table := maps.Clone(s.table)
	for _, f := range prog.Funcs {
		if _, ok := table[f.Name]; !ok {
			table[f.Name] = len(table)
		}
	}
	return table
}

func changedFuncs(old, current map[string]string) []string {
	var changed []string
	for name, asm := range current {
		if old[name] != asm {
			changed = append(changed, name)
		}
	}
	slices.Sort(changed)
	return changed
}

func (s *Session) start(ctx context.Context, unit *compiler.Unit, reason string) error {
	s.builds++
	table := map[string]int{}
	for i, f := range unit.Program.Funcs {
		table[f.Name] = i
	}
	out := amd64.Generate(unit.Program, unit.Platform, table)
	binary := filepath.Join(s.dir, fmt.Sprintf("app%d", s.builds))
	if err := compiler.Link(ctx, out.Asm, binary, unit); err != nil {
		return err
	}
	symbols, err := readSymbols(binary)
	if err != nil {
		return err
	}
	cmd := exec.Command(binary)
	cmd.Stdout, cmd.Stderr = s.stdout, s.stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	c := &child{cmd: cmd, done: make(chan error, 1)}
	go c.wait()
	s.child = c
	s.binary, s.symbols = binary, symbols
	s.table, s.funcs = table, out.Funcs
	s.signatures, s.layout = signatures(unit.Files), layout(unit.Files)
	s.libraries = unit.Program.Libraries()
	s.cSymbols = unit.Program.CSymbols
	s.next = 0
	s.logf("%s", reason)
	return nil
}

func (c *child) wait() {
	err := c.cmd.Wait()
	c.exited.Store(true)
	c.done <- err
}

func (s *Session) restart(ctx context.Context, unit *compiler.Unit, reason string) error {
	s.stop()
	return s.start(ctx, unit, fmt.Sprintf("restarted: %s", reason))
}

func (s *Session) stop() {
	if s.child == nil {
		return
	}
	s.child.cmd.Process.Kill()
	<-s.child.done
	s.child = nil
}

func readSymbols(path string) (map[string]uint64, error) {
	f, err := elf.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	syms, err := f.Symbols()
	if err != nil {
		return nil, err
	}
	out := map[string]uint64{}
	for _, sym := range syms {
		out[sym.Name] = sym.Value
	}
	return out, nil
}

func (s *Session) patch(ctx context.Context, unit *compiler.Unit, table map[string]int, changed []string) error {
	suffix := fmt.Sprintf("_r%d_%d", s.builds, s.next)
	address := amd64.HotBase + s.next
	output := filepath.Join(s.dir, fmt.Sprintf("patch%d_%d", s.builds, s.next))
	asm := amd64.GeneratePatch(unit.Program, unit.Platform, changed, table, suffix)
	if err := compiler.LinkAt(ctx, asm, output, s.binary, address); err != nil {
		return err
	}
	section, symbols, err := readPatch(output)
	if err != nil {
		return err
	}
	end := section.address + uint64(len(section.code)) - amd64.HotBase
	if end > amd64.HotSize {
		return errors.New("hot code space exhausted")
	}
	entries := make([]byte, 0, 16*len(changed))
	for _, name := range changed {
		entries = binary.LittleEndian.AppendUint64(entries, uint64(table[name]))
		entries = binary.LittleEndian.AppendUint64(entries, symbols[amd64.Symbol(name, suffix)])
	}
	if err := s.apply(ctx, section.code, section.address, entries, len(changed)); err != nil {
		return err
	}
	s.next = (end + 15) &^ 15
	return nil
}

type patchSection struct {
	address uint64
	code    []byte
}

func readPatch(path string) (patchSection, map[string]uint64, error) {
	f, err := elf.Open(path)
	if err != nil {
		return patchSection{}, nil, err
	}
	defer f.Close()
	section := f.Section(".hot")
	if section == nil {
		return patchSection{}, nil, errors.New("patch has no .hot section")
	}
	code, err := section.Data()
	if err != nil {
		return patchSection{}, nil, err
	}
	symbols, err := readSymbols(path)
	return patchSection{address: section.Addr, code: code}, symbols, err
}

func (s *Session) apply(ctx context.Context, code []byte, address uint64, entries []byte, count int) error {
	pid := s.child.cmd.Process.Pid
	if err := waitForHotRegion(ctx, pid); err != nil {
		return err
	}
	mem, err := os.OpenFile(fmt.Sprintf("/proc/%d/mem", pid), os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer mem.Close()
	if _, err := mem.WriteAt(code, int64(address)); err != nil {
		return fmt.Errorf("writing code at %#x: %w", address, err)
	}
	if _, err := mem.WriteAt(entries, int64(s.symbols[amd64.PatchListSymbol])); err != nil {
		return fmt.Errorf("writing patch list: %w", err)
	}
	counter := int64(s.symbols[amd64.PatchCountSymbol])
	if _, err := mem.WriteAt(binary.LittleEndian.AppendUint64(nil, uint64(count)), counter); err != nil {
		return fmt.Errorf("writing patch count: %w", err)
	}
	if err := notify(s.child.cmd.Process); err != nil {
		return err
	}
	if err := waitForZero(ctx, mem, counter); err != nil && s.awaitExit(ctx) != nil {
		return fmt.Errorf("waiting for the program to apply the patch: %w", err)
	}
	return nil
}

func (s *Session) awaitExit(ctx context.Context) error {
	return poll(ctx, func() (bool, error) { return s.child.exited.Load(), nil })
}

func waitForHotRegion(ctx context.Context, pid int) error {
	region := fmt.Sprintf("%x-", amd64.HotBase)
	return poll(ctx, func() (bool, error) {
		maps, err := os.ReadFile(fmt.Sprintf("/proc/%d/maps", pid))
		if err != nil {
			return false, err
		}
		return strings.Contains(string(maps), region), nil
	})
}

func waitForZero(ctx context.Context, mem *os.File, address int64) error {
	word := make([]byte, 8)
	return poll(ctx, func() (bool, error) {
		if _, err := mem.ReadAt(word, address); err != nil {
			return false, err
		}
		return binary.LittleEndian.Uint64(word) == 0, nil
	})
}

func poll(ctx context.Context, ready func() (bool, error)) error {
	deadline := time.Now().Add(2 * time.Second)
	for {
		ok, err := ready()
		if err != nil || ok {
			return err
		}
		if time.Now().After(deadline) {
			return errors.New("program did not respond")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond):
		}
	}
}

func layout(files []*ast.File) string {
	var b strings.Builder
	for _, file := range files {
		for _, d := range file.Enums {
			fmt.Fprintf(&b, "enum %s", d.Type.Name)
			for _, v := range d.Type.Variants {
				fmt.Fprintf(&b, " %s%v", v.Name, v.Fields)
			}
			b.WriteString(";")
		}
		for _, d := range file.Structs {
			fmt.Fprintf(&b, "struct %s", d.Type.Name)
			for _, f := range d.Type.Fields {
				fmt.Fprintf(&b, " %s:%s", f.Name, f.Type)
			}
			b.WriteString(";")
		}
	}
	return b.String()
}

func signatures(files []*ast.File) map[string]string {
	out := map[string]string{}
	for _, file := range files {
		for _, fn := range file.Funcs {
			var b strings.Builder
			for i, p := range fn.Params {
				fmt.Fprintf(&b, "%t %s,", p.Mut, fn.ParamTypes[i])
			}
			fmt.Fprintf(&b, "-> %s", fn.RetType)
			if fn.Closure != nil {
				for _, l := range fn.Closure.Captures {
					fmt.Fprintf(&b, " %s %s %t", l.Name, l.Type, ir.Boxed(l))
				}
			}
			out[fmt.Sprintf("%s.%s", file.Module, fn.Name)] = b.String()
		}
	}
	return out
}
