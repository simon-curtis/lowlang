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

type Profile struct {
	platform target.OS
	optimise bool
	hot      bool
}

func Release(platform target.OS) Profile {
	return Profile{platform: platform, optimise: true}
}

var Dev = Profile{platform: target.Linux, hot: true}

type Unit struct {
	Files    []*ast.File
	Types    *types.Universe
	profile  Profile
	Platform amd64.Platform
	Program  *ir.Program
	Sources  []string
	Embeds   map[string][]byte
	Warnings []lint.Warning
}

//go:embed prelude.low
var preludeSource []byte

func Analyse(path string, profile Profile) (*Unit, error) {
	unit, err := Check(path, profile.platform, Overlay(nil))
	if err != nil {
		return nil, err
	}
	unit.lower(profile)
	return unit, nil
}

func (u *Unit) lower(profile Profile) {
	u.profile = profile
	u.Platform = amd64.NewPlatform(profile.platform)
	u.Program = ir.Build(u.Files, profile.optimise)
}

func (u *Unit) newTable() amd64.FuncTable {
	if !u.profile.hot {
		return nil
	}
	return amd64.NewFuncTable(u.Program)
}

func Check(path string, platform target.OS, disk fs.FS) (*Unit, error) {
	file, err := parsePrelude()
	if err != nil {
		return nil, err
	}
	modules, err := load(path, platform, disk)
	if err != nil {
		return nil, err
	}
	files := append([]*ast.File{file}, modules.files...)
	embeds := &embedder{disk: disk, std: stdFS, data: map[string][]byte{}}
	universe, err := check.Check(files, embeds.resolve)
	if err != nil {
		return nil, err
	}
	unit := &Unit{Files: files, Types: universe, Sources: append(modules.sources, embeds.paths...), Embeds: embeds.data}
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
