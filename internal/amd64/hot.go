package amd64

import (
	"encoding/binary"
	"fmt"
	"maps"
	"slices"
	"strings"

	"lowlang/internal/ir"
)

type FuncTable map[string]int

func NewFuncTable(prog *ir.Program) FuncTable {
	return FuncTable{}.Extend(prog)
}

func (t FuncTable) Extend(prog *ir.Program) FuncTable {
	extended := maps.Clone(t)
	for _, f := range prog.Funcs {
		if _, ok := extended[f.Name]; !ok {
			extended[f.Name] = len(extended)
		}
	}
	return extended
}

func (t FuncTable) Full() bool {
	return len(t) > TableSize
}

func (t FuncTable) PatchEntries(changed []string, suffix string, symbols map[string]uint64) []byte {
	entries := make([]byte, 0, PatchEntryBytes*len(changed))
	for _, name := range changed {
		entries = binary.LittleEndian.AppendUint64(entries, uint64(t[name]))
		entries = binary.LittleEndian.AppendUint64(entries, symbols[Symbol(name, suffix)])
	}
	return entries
}

func (t FuncTable) writeRuntime(out *strings.Builder) {
	writeSymbols(out, []symbol{
		{"HOT_BASE", HotBase},
		{"HOT_SIZE", HotSize},
		{"PATCH_BYTES", PatchEntryBytes * TableSize},
		{"PATCH_ENTRY", PatchEntryBytes},
		{"PATCH_SIGNAL", int(PatchSignal)},
	})
	out.WriteString(hotRuntime)
	entries := make([]string, TableSize)
	for i := range entries {
		entries[i] = "0"
	}
	for name, i := range t {
		entries[i] = Symbol(name, "")
	}
	out.WriteString("\n\t.data\n\t.p2align 3\n\t.globl fn_table\nfn_table:\n")
	for _, e := range entries {
		fmt.Fprintf(out, "\t.quad %s\n", e)
	}
}

func GenerateHot(prog *ir.Program, platform Platform, table FuncTable) Output {
	return generate(prog, platform, table)
}

func GeneratePatch(prog *ir.Program, platform Platform, names []string, table FuncTable, suffix string) string {
	g := newGenerator(platform, table, suffix)
	for _, f := range prog.Funcs {
		if slices.Contains(names, f.Name) {
			g.function(f)
		}
	}
	var out strings.Builder
	out.WriteString("\t.intel_syntax noprefix\n")
	out.WriteString(layout)
	out.WriteString(platform.header())
	out.WriteString("\t.text\n")
	out.WriteString(g.text.String())
	out.WriteString("\n\t.section .rodata\n")
	out.WriteString(g.data.String())
	return out.String()
}
