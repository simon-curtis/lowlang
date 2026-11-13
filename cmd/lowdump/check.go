package main

import (
	"fmt"
	"io"

	"lowlang/internal/compiler"
	"lowlang/internal/lexer"
	"lowlang/internal/target"
)

func dumpCheckCommand(out io.Writer, path string, platform target.OS) error {
	unit, err := compiler.Check(path, platform, compiler.Overlay(nil))
	if err != nil {
		return err
	}
	d := astDumper{out}
	for _, f := range unit.Files {
		d.line(0, "", fmt.Sprintf("File %s%s", lexer.Quote(f.Path), named("module", f.Module)))
		d.file(f)
	}
	for _, source := range unit.Sources {
		d.line(0, "", fmt.Sprintf("Source %s", lexer.Quote(source)))
	}
	for _, w := range unit.Warnings {
		d.line(0, "", fmt.Sprintf("Warning %s rule=%s msg=%s", at(w.Pos), w.Rule, lexer.Quote(w.Msg)))
	}
	return nil
}
