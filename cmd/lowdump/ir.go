package main

import (
	"fmt"
	"io"
	"slices"

	"lowlang/internal/compiler"
	"lowlang/internal/ir"
	"lowlang/internal/target"
)

func dumpIRCommand(out io.Writer, path string, flags []string) error {
	unit, err := compiler.Check(path, platformOf(flags), compiler.Overlay(nil))
	if err != nil {
		return err
	}
	program := ir.Build(unit.Files, !slices.Contains(flags, "-unoptimised"))
	for _, f := range program.Funcs {
		fmt.Fprint(out, f)
	}
	return nil
}

func dumpAsmCommand(out io.Writer, path string, flags []string) error {
	unit, err := compiler.Analyse(path, compiler.Release(platformOf(flags)))
	if err != nil {
		return err
	}
	generated, err := compiler.Generate(unit)
	if err != nil {
		return err
	}
	fmt.Fprint(out, generated.Asm)
	return nil
}

func platformOf(flags []string) target.OS {
	if slices.Contains(flags, "-windows") {
		return target.Windows
	}
	return target.Linux
}
