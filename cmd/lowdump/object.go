package main

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"lowlang/internal/asm"
	"lowlang/internal/lexer"
)

func dumpObjectCommand(out io.Writer, src []byte, flags []string) error {
	opts := asm.Options{Embeds: map[string][]byte{}}
	for i := 0; i < len(flags); i++ {
		switch flags[i] {
		case "-clang":
			opts.ClangLayout = true
		case "-align-branches":
			opts.AlignBranches = true
		case "-embed":
			i++
			if i == len(flags) {
				return fmt.Errorf("%s", usage)
			}
			if err := embedFlag(&opts, flags[i]); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s", usage)
		}
	}
	object, err := asm.Assemble(string(src), opts)
	if err != nil {
		return err
	}
	dumpObject(out, object)
	return nil
}

func embedFlag(opts *asm.Options, value string) error {
	name, path, ok := strings.Cut(value, "=")
	if !ok {
		return fmt.Errorf("%s", usage)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	opts.Embeds[name] = data
	return nil
}

func dumpObject(out io.Writer, object *asm.Object) {
	for _, sec := range object.Sections {
		fmt.Fprintf(out, "section %s flags=%s align=%d nobits=%t size=%d\n", lexer.Quote(sec.Name), sec.Flags, sec.Align, sec.NoBits, sec.Size)
		fmt.Fprintf(out, "  data %s\n", hex.EncodeToString(sec.Data))
		for _, f := range sec.Fixups {
			fmt.Fprintf(out, "  fixup offset=%d kind=%s symbol=%s addend=%d line=%d\n", f.Offset, f.Kind, lexer.Quote(f.Symbol), f.Addend, f.Line)
		}
	}
	for _, sym := range object.Symbols {
		fmt.Fprintf(out, "symbol %s section=%d value=%d global=%t\n", lexer.Quote(sym.Name), sym.Section, sym.Value, sym.Global)
	}
}
