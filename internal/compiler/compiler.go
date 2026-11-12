package compiler

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"lowlang/internal/asm"
	"lowlang/internal/ast"
	"lowlang/internal/check"
	"lowlang/internal/ir"
	"lowlang/internal/lexer"
	"lowlang/internal/link"
	"lowlang/internal/lint"
	"lowlang/internal/parser"
	"lowlang/internal/target"
)

type Unit struct {
	Files    []*ast.File
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
	unit.Program = ir.Build(unit.Files, platform, opts)
	return unit, nil
}

func Check(path string, read func(name string) ([]byte, error)) (*Unit, error) {
	prelude, err := parsePrelude()
	if err != nil {
		return nil, err
	}
	modules, err := load(path, read)
	if err != nil {
		return nil, err
	}
	files := append([]*ast.File{prelude}, modules.files...)
	embeds, err := check.Check(files, stdFS)
	if err != nil {
		return nil, err
	}
	unit := &Unit{Files: files, Sources: append(modules.sources, embeds...)}
	unit.Warnings = lint.Run(modules.userFiles())
	return unit, nil
}

func parsePrelude() (*ast.File, error) {
	tokens, err := lexer.LexPrelude(preludeSource)
	if err != nil {
		return nil, err
	}
	return parser.ParsePrelude(tokens)
}

func Assemble(ctx context.Context, asm, object string, flags ...string) error {
	source := fmt.Sprintf("%s.s", object)
	if err := os.WriteFile(source, []byte(asm), 0o644); err != nil {
		return err
	}
	args := append(flags, "-o", object, source)
	return tool(ctx, "as", args...)
}

func Link(ctx context.Context, source, output string, prog *ir.Program) error {
	if prog.OS == target.Windows {
		return linkWindows(ctx, source, output)
	}
	object, err := asm.Assemble(source, asm.Options{AlignBranches: true})
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	image, err := linkImage(object, prog.Libraries)
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

func linkImage(object *asm.Object, libraries []string) ([]byte, error) {
	if len(libraries) == 0 {
		return link.Static(object)
	}
	resolved, err := libraryResolver.Resolve(libraries)
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

const windowsStack = 8 << 20

func linkWindows(ctx context.Context, asm, output string) error {
	dir, err := os.MkdirTemp("", "low-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	source := filepath.Join(dir, "main.s")
	if err := os.WriteFile(source, []byte(asm), 0o644); err != nil {
		return err
	}
	object := filepath.Join(dir, "main.obj")
	if err := tool(ctx, "clang", "--target=x86_64-pc-windows-msvc", "-c", "-o", object, source); err != nil {
		return err
	}
	return tool(ctx, "lld-link",
		"/nologo", "/brepro", "/nodefaultlib", "/entry:start", "/subsystem:console",
		fmt.Sprintf("/stack:%d", windowsStack), fmt.Sprintf("/out:%s", output), object)
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
