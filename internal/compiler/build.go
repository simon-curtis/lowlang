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

type Built struct {
	Funcs   map[string]string
	Symbols map[string]uint64
	Table   amd64.FuncTable
}

type toolchain struct {
	assembler asm.Options
	link      func(object *asm.Object, libraries []string) (link.Image, error)
}

var toolchains = map[target.OS]toolchain{
	target.Linux:   {assembler: asm.Options{AlignBranches: true}, link: linkELF},
	target.Windows: {assembler: asm.Options{ClangLayout: true}, link: linkPE},
}

var libraryResolver = link.Resolver{Dirs: link.SystemDirs}

func Assembler(platform target.OS) asm.Options {
	return toolchains[platform].assembler
}

func (u *Unit) assembler() asm.Options {
	options := Assembler(u.Platform.OS())
	options.Embeds = u.Embeds
	return options
}

func Generate(unit *Unit) (amd64.Output, error) {
	return GenerateWith(unit, unit.newTable())
}

func GenerateWith(unit *Unit, table amd64.FuncTable) (amd64.Output, error) {
	return amd64.Generate(unit.Program, unit.Platform, table)
}

func Build(ctx context.Context, unit *Unit, output string) (*Built, error) {
	table := unit.newTable()
	out, err := GenerateWith(unit, table)
	if err != nil {
		return nil, err
	}
	object, err := asm.Assemble(out.Asm, unit.assembler())
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	image, err := toolchains[unit.Platform.OS()].link(object, unit.Program.Libraries())
	if err != nil {
		return nil, err
	}
	if err := writeExecutable(output, image.Bytes); err != nil {
		return nil, err
	}
	return &Built{Funcs: out.Funcs, Symbols: image.Symbols, Table: table}, nil
}

func (b *Built) Patch(unit *Unit, table amd64.FuncTable, changed []string, suffix string, base uint64) (*link.PatchImage, error) {
	source := amd64.GeneratePatch(unit.Program, unit.Platform, changed, table, suffix)
	object, err := asm.Assemble(source, unit.assembler())
	if err != nil {
		return nil, err
	}
	return link.Patch(object, base, b.Symbols)
}

func linkELF(object *asm.Object, libraries []string) (link.Image, error) {
	if len(libraries) == 0 {
		return link.Static(object)
	}
	resolved, err := libraryResolver.Resolve(libraries)
	if err != nil {
		return link.Image{}, err
	}
	return link.Dynamic(object, resolved)
}

func linkPE(object *asm.Object, _ []string) (link.Image, error) {
	bytes, err := link.PE(object)
	return link.Image{Bytes: bytes}, err
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
