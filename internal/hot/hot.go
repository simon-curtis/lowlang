package hot

import (
	"context"
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
	"lowlang/internal/compiler"
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
	built      *compiler.Built
	table      amd64.FuncTable
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
	table := s.table.Extend(unit.Program)
	if table.Full() {
		return s.restart(ctx, unit, "function table full")
	}
	out, err := compiler.Generate(unit, compiler.Hot(table))
	if err != nil {
		return err
	}
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
	s.signatures = signatures(unit.Program)
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
	for name, sig := range signatures(unit.Program) {
		if old, ok := s.signatures[name]; ok && old != sig {
			return fmt.Sprintf("signature of %s changed", name)
		}
	}
	return ""
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
	table := amd64.NewFuncTable(unit.Program)
	binary := filepath.Join(s.dir, fmt.Sprintf("app%d", s.builds))
	built, err := compiler.Build(ctx, unit, compiler.Hot(table), binary)
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
	s.built = built
	s.table, s.funcs = table, built.Funcs
	s.signatures, s.layout = signatures(unit.Program), layout(unit.Files)
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

func (s *Session) patch(ctx context.Context, unit *compiler.Unit, table amd64.FuncTable, changed []string) error {
	suffix := fmt.Sprintf("_r%d_%d", s.builds, s.next)
	patch, err := s.built.Patch(unit, table, changed, suffix, amd64.HotBase+s.next)
	if err != nil {
		return err
	}
	end := patch.Address + uint64(len(patch.Code)) - amd64.HotBase
	if end > amd64.HotSize {
		return errors.New("hot code space exhausted")
	}
	entries := table.PatchEntries(changed, suffix, patch.Symbols)
	if err := s.apply(ctx, patch.Code, patch.Address, entries, len(changed)); err != nil {
		return err
	}
	s.next = (end + 15) &^ 15
	return nil
}

func (s *Session) mailbox() (list, count int64, err error) {
	listAddress, ok := s.built.Symbols[amd64.PatchListSymbol]
	if !ok {
		return 0, 0, fmt.Errorf("program has no %s", amd64.PatchListSymbol)
	}
	countAddress, ok := s.built.Symbols[amd64.PatchCountSymbol]
	if !ok {
		return 0, 0, fmt.Errorf("program has no %s", amd64.PatchCountSymbol)
	}
	return int64(listAddress), int64(countAddress), nil
}

func (s *Session) apply(ctx context.Context, code []byte, address uint64, entries []byte, count int) error {
	list, counter, err := s.mailbox()
	if err != nil {
		return err
	}
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
	if _, err := mem.WriteAt(entries, list); err != nil {
		return fmt.Errorf("writing patch list: %w", err)
	}
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
