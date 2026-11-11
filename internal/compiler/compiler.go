package compiler

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/check"
	"lowlang/internal/ir"
	"lowlang/internal/lexer"
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

func Link(ctx context.Context, asm, output string, prog *ir.Program) error {
	dir, err := os.MkdirTemp("", "low-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if prog.OS == target.Windows {
		return linkWindows(ctx, dir, asm, output)
	}
	object := filepath.Join(dir, "main.o")
	if err := Assemble(ctx, asm, object, "-mbranches-within-32B-boundaries"); err != nil {
		return err
	}
	if len(prog.Libraries) == 0 {
		return tool(ctx, "ld", "-static", "-o", output, object)
	}
	args, err := dynamicArgs(output, object, prog.Libraries)
	if err != nil {
		return err
	}
	return tool(ctx, "ld", args...)
}

const windowsStack = 8 << 20

func linkWindows(ctx context.Context, dir, asm, output string) error {
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

const dynamicLinker = "/lib64/ld-linux-x86-64.so.2"

var startFileDirs = []string{"/usr/lib", "/usr/lib64", "/usr/lib/x86_64-linux-gnu", "/lib/x86_64-linux-gnu"}

func startFiles() (string, error) {
	for _, dir := range startFileDirs {
		if _, err := os.Stat(filepath.Join(dir, "crt1.o")); err == nil {
			return dir, nil
		}
	}
	return "", fmt.Errorf("cannot find the C start files (crt1.o) in %s; install the C library development files", strings.Join(startFileDirs, ", "))
}

func dynamicArgs(output, object string, libraries []string) ([]string, error) {
	dir, err := startFiles()
	if err != nil {
		return nil, err
	}
	args := []string{
		"-o", output, "-dynamic-linker", dynamicLinker, "-L", dir,
		filepath.Join(dir, "crt1.o"), filepath.Join(dir, "crti.o"), object,
	}
	for _, lib := range libraries {
		if lib != "c" {
			args = append(args, fmt.Sprintf("-l%s", lib))
		}
	}
	return append(args, "-lc", filepath.Join(dir, "crtn.o")), nil
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
