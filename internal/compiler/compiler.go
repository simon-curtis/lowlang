package compiler

import (
	_ "embed"
	"io/fs"

	"lowlang/internal/amd64"
	"lowlang/internal/ast"
	"lowlang/internal/check"
	"lowlang/internal/ir"
	"lowlang/internal/lint"
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
	unit, err := Check(path, Overlay(nil))
	if err != nil {
		return nil, err
	}
	unit.Platform = amd64.NewPlatform(platform)
	unit.Program = ir.Build(unit.Files, opts)
	return unit, nil
}

func Check(path string, disk fs.FS) (*Unit, error) {
	file, err := parsePrelude()
	if err != nil {
		return nil, err
	}
	modules, err := load(path, disk)
	if err != nil {
		return nil, err
	}
	files := append([]*ast.File{file}, modules.files...)
	universe := types.NewUniverse()
	embeds := &embedder{disk: disk, std: stdFS}
	if err := check.Check(files, embeds.resolve, universe); err != nil {
		return nil, err
	}
	unit := &Unit{Files: files, Types: universe, Sources: append(modules.sources, embeds.paths...)}
	unit.Warnings = lint.Run(modules.userFiles())
	return unit, nil
}

func parsePrelude() (*ast.File, error) {
	file, err := parseSource(preludeSource, "prelude.low", true)
	if err != nil {
		return nil, err
	}
	if err := prelude.Validate(file); err != nil {
		return nil, err
	}
	return file, nil
}
