package compiler

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"

	"lowlang/internal/amd64"
	"lowlang/internal/asm"
	"lowlang/internal/link"
	"lowlang/internal/target"
)

type Mode struct {
	hot   bool
	table amd64.FuncTable
}

var Release = Mode{}

func Hot(table amd64.FuncTable) Mode {
	return Mode{hot: true, table: table}
}

type Built struct {
	Funcs   map[string]string
	Symbols map[string]uint64
}

type toolchain struct {
	assembler asm.Options
	link      func(object *asm.Object, libraries []string) ([]byte, error)
}

var toolchains = map[target.OS]toolchain{
	target.Linux:   {assembler: asm.Options{AlignBranches: true}, link: linkELF},
	target.Windows: {assembler: asm.Options{ClangLayout: true}, link: linkPE},
}

var libraryResolver = link.Resolver{Dirs: link.SystemDirs}

func Generate(unit *Unit, mode Mode) (amd64.Output, error) {
	platform := unit.Platform
	switch {
	case !mode.hot:
		return amd64.Generate(unit.Program, platform), nil
	case platform.OS() != target.Linux:
		return amd64.Output{}, fmt.Errorf("hot reload needs the %s target, not %s", target.Linux, platform.OS())
	}
	return amd64.GenerateHot(unit.Program, platform, mode.table), nil
}

func Build(ctx context.Context, unit *Unit, mode Mode, output string) (*Built, error) {
	out, err := Generate(unit, mode)
	if err != nil {
		return nil, err
	}
	chain := toolchains[unit.Platform.OS()]
	object, err := asm.Assemble(out.Asm, chain.assembler)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	image, err := chain.link(object, unit.Program.Libraries())
	if err != nil {
		return nil, err
	}
	if err := writeExecutable(output, image); err != nil {
		return nil, err
	}
	built := &Built{Funcs: out.Funcs}
	if mode.hot {
		built.Symbols, err = link.ImageSymbols(image)
	}
	return built, err
}

func (b *Built) Patch(unit *Unit, table amd64.FuncTable, changed []string, suffix string, base uint64) (*link.PatchImage, error) {
	source := amd64.GeneratePatch(unit.Program, unit.Platform, changed, table, suffix)
	object, err := asm.Assemble(source, toolchains[target.Linux].assembler)
	if err != nil {
		return nil, err
	}
	return link.Patch(object, base, b.Symbols)
}

func linkELF(object *asm.Object, libraries []string) ([]byte, error) {
	if len(libraries) == 0 {
		return link.Static(object)
	}
	resolved, err := libraryResolver.Resolve(libraries)
	if err != nil {
		return nil, err
	}
	return link.Dynamic(object, resolved)
}

func linkPE(object *asm.Object, _ []string) ([]byte, error) {
	return link.PE(object)
}

func writeExecutable(output string, image []byte) error {
	if err := os.Remove(output); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("replace %s: %w", output, err)
	}
	if err := writeWhileForksWait(output, image); err != nil {
		return fmt.Errorf("write %s: %w", output, err)
	}
	return nil
}

func writeWhileForksWait(path string, data []byte) error {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	return os.WriteFile(path, data, 0o755)
}
