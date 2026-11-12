package compiler

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"syscall"

	"lowlang/internal/amd64"
	"lowlang/internal/asm"
	"lowlang/internal/ast"
	"lowlang/internal/check"
	"lowlang/internal/diag"
	"lowlang/internal/ir"
	"lowlang/internal/link"
	"lowlang/internal/lint"
	"lowlang/internal/parser"
	"lowlang/internal/prelude"
	"lowlang/internal/target"
	"lowlang/internal/types"
)

type Unit struct {
	Files    []*ast.File
	Types    *types.Universe
	Platform amd64.Platform
	Program  *ir.Program
	Sources  []string
	Warnings []lint.Warning
}

//go:embed prelude.low
var preludeSource []byte

func Analyse(path string, platform target.OS) (*Unit, error) {
	return analyse(path, platform, ir.Options{Inline: true, Hoist: true})
}

func AnalyseDev(path string, platform target.OS) (*Unit, error) {
	return analyse(path, platform, ir.Options{})
}

func analyse(path string, platform target.OS, opts ir.Options) (*Unit, error) {
	unit, err := Check(path, os.ReadFile)
	if err != nil {
		return nil, err
	}
	unit.Platform = amd64.NewPlatform(platform)
	unit.Program = ir.Build(unit.Files, opts)
	return unit, nil
}

func Check(path string, read func(name string) ([]byte, error)) (*Unit, error) {
	file, err := parsePrelude()
	if err != nil {
		return nil, err
	}
	modules, err := load(path, read)
	if err != nil {
		return nil, err
	}
	files := append([]*ast.File{file}, modules.files...)
	universe := types.NewUniverse()
	embeds, err := check.Check(files, stdFS, universe)
	if err != nil {
		return nil, err
	}
	unit := &Unit{Files: files, Types: universe, Sources: append(modules.sources, embeds...)}
	unit.Warnings = lint.Run(modules.userFiles())
	return unit, nil
}

func parsePrelude() (*ast.File, error) {
	file, err := parser.ParseSource(preludeSource, diag.Start("prelude.low"), true)
	if err != nil {
		return nil, err
	}
	if err := prelude.Validate(file); err != nil {
		return nil, err
	}
	return file, nil
}

func Assemble(ctx context.Context, asm, object string, flags ...string) error {
	source := fmt.Sprintf("%s.s", object)
	if err := os.WriteFile(source, []byte(asm), 0o644); err != nil {
		return err
	}
	args := append(flags, "-o", object, source)
	return tool(ctx, "as", args...)
}

func Link(ctx context.Context, source, output string, unit *Unit) error {
	windows := unit.Platform.OS() == target.Windows
	object, err := asm.Assemble(source, asm.Options{AlignBranches: !windows, ClangLayout: windows})
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	image, err := linkImage(object, unit)
	if err != nil {
		return err
	}
	if err := os.Remove(output); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("replace %s: %w", output, err)
	}
	if err := writeWhileForksWait(output, image); err != nil {
		return fmt.Errorf("write %s: %w", output, err)
	}
	return nil
}

var libraryResolver = link.Resolver{Dirs: link.SystemDirs}

func linkImage(object *asm.Object, unit *Unit) ([]byte, error) {
	if unit.Platform.OS() == target.Windows {
		return link.PE(object)
	}
	if len(unit.Program.Libraries()) == 0 {
		return link.Static(object)
	}
	resolved, err := libraryResolver.Resolve(unit.Program.Libraries())
	if err != nil {
		return nil, err
	}
	return link.Dynamic(object, resolved)
}

func writeWhileForksWait(path string, data []byte) error {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	return os.WriteFile(path, data, 0o755)
}

func LinkAt(ctx context.Context, asm, output, against string, address uint64) error {
	object := fmt.Sprintf("%s.o", output)
	if err := Assemble(ctx, asm, object); err != nil {
		return err
	}
	script := fmt.Sprintf("%s.ld", output)
	layout := fmt.Sprintf("SECTIONS { . = %#x; .hot : { *(.text) *(.rodata) } }\n", address)
	if err := os.WriteFile(script, []byte(layout), 0o644); err != nil {
		return err
	}
	return tool(ctx, "ld", "-e", "0", "-T", script, "-R", against, "-o", output, object)
}

func tool(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s failed: %w\n%s", name, err, out)
	}
	return nil
}
