package amd64

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"lowlang/internal/ir"
	"lowlang/internal/target"
)

const (
	pageSize     = 4096
	cRuntimeDLL  = "msvcrt.dll"
	mathDLL      = "api-ms-win-crt-math-l1-1-0.dll"
	kernelDLL    = "kernel32.dll"
	synchDLL     = "api-ms-win-core-synch-l1-2-0.dll"
	randomDLL    = "bcryptprimitives.dll"
	importPrefix = "__imp_"
)

var win64IntRegs = []string{"rcx", "rdx", "r8", "r9"}

var winPreservedVectors = []string{
	"xmm6", "xmm7", "xmm8", "xmm9", "xmm10", "xmm11", "xmm12", "xmm13", "xmm14", "xmm15",
}

type win64 struct{}

func (win64) OS() target.OS {
	return target.Windows
}

func (win64) entry() string {
	return PEEntrySymbol
}

func (win64) header() string {
	return windowsHeader
}

func (win64) runtime() string {
	return windowsRuntime
}

func (win64) classify(sig *ir.CSig) *cPlan {
	return win64Plan(sig)
}

func (win64) intRegs() []string {
	return win64IntRegs
}

func (win64) vectorRegs() int {
	return len(win64IntRegs)
}

func (win64) registerSlots() int {
	return len(win64IntRegs)
}

func (win64) preservedVectors() []string {
	return winPreservedVectors
}

func (win64) thunkSaved() []string {
	return append(slices.Clone(abi.saved), "rdi", "rsi")
}

func (win64) positional() bool {
	return true
}

func (win64) syscall(g *Generator) {
	g.emit("call rt_panic_syscall")
}

func (win64) variadicCount(*Generator, *cPlan) {}

func (win64) trampolines(*Generator, []string) {}

func (win64) reserve(g *Generator, size int) {
	if size < pageSize {
		if size > 0 {
			g.emit("sub rsp, %d", size)
		}
		return
	}
	probe := g.newLabel()
	g.emit("lea r11, [rsp-%d]", size)
	g.label(probe)
	g.emit("sub rsp, %d", pageSize)
	g.emit("test qword ptr [rsp], rsp")
	g.emit("cmp rsp, r11")
	g.emit("ja %s", probe)
	g.emit("mov rsp, r11")
}

func (win64) externalTarget(symbol string, trampolines bool) string {
	return fmt.Sprintf("qword ptr [rip+%s]", importSymbol(symbol))
}

func (win64) symbolAddress(symbol string) string {
	return fmt.Sprintf("qword ptr [rip+%s]", importSymbol(symbol))
}

func (win64) imports(out *strings.Builder, prog *ir.Program) {
	writeImports(out, prog)
}

func (win64) dynamic(*ir.Program) bool {
	return false
}

func (win64) shims(*Generator, []string) {}

var runtimeImportDLLs = map[string]string{
	"WaitOnAddress":       synchDLL,
	"WakeByAddressAll":    synchDLL,
	"WakeByAddressSingle": synchDLL,
	"ProcessPrng":         randomDLL,
}

var winCall = regexp.MustCompile(`(?m)^\s*WIN\s+(\w+)\s*$`)

func runtimeImports() map[string][]string {
	imports := map[string][]string{}
	for _, match := range winCall.FindAllStringSubmatch(windowsRuntime, -1) {
		symbol := match[1]
		dll, ok := runtimeImportDLLs[symbol]
		if !ok {
			dll = kernelDLL
		}
		imports[dll] = append(imports[dll], symbol)
	}
	return imports
}

func importSymbol(symbol string) string {
	return fmt.Sprintf("%s%s", importPrefix, symbol)
}

func dllName(library string) string {
	switch library {
	case "c":
		return cRuntimeDLL
	case "m":
		return mathDLL
	}
	if strings.HasSuffix(strings.ToLower(library), ".dll") {
		return strings.ToLower(library)
	}
	return fmt.Sprintf("%s.dll", strings.ToLower(library))
}

func importTable(prog *ir.Program) map[string][]string {
	table := runtimeImports()
	for library, symbols := range prog.Imports {
		dll := dllName(library)
		table[dll] = append(table[dll], symbols...)
	}
	for dll, symbols := range table {
		slices.Sort(symbols)
		table[dll] = slices.Compact(symbols)
	}
	return table
}

func writeImports(out *strings.Builder, prog *ir.Program) {
	table := importTable(prog)
	dlls := slices.Sorted(maps.Keys(table))
	for i := range dlls {
		fmt.Fprintf(out, "\n\t.section %s,\"dr\"\n\t.long .Lilt%d@IMGREL, 0, 0, .Ldll%d@IMGREL, .Liat%d@IMGREL\n", ImportDescriptors, i, i, i)
	}
	fmt.Fprintf(out, "\n\t.section %s,\"dr\"\n\t.long 0, 0, 0, 0, 0\n", ImportNullDescriptor)
	for i, dll := range dlls {
		writeThunks(out, ImportLookups, fmt.Sprintf(".Lilt%d", i), i, table[dll], false)
		writeThunks(out, ImportAddresses, fmt.Sprintf(".Liat%d", i), i, table[dll], true)
		fmt.Fprintf(out, "\n\t.section %s,\"dr\"\n", ImportHints)
		for j, symbol := range table[dll] {
			fmt.Fprintf(out, "\t.p2align 1\n.Lhint%d_%d:\n\t.short 0\n\t.asciz %q\n", i, j, symbol)
		}
		fmt.Fprintf(out, "\n\t.section %s,\"dr\"\n.Ldll%d:\n\t.asciz %q\n", ImportNames, i, dll)
	}
}

func writeThunks(out *strings.Builder, section, label string, dll int, symbols []string, named bool) {
	fmt.Fprintf(out, "\n\t.section %s,\"dr\"\n\t.p2align 3\n%s:\n", section, label)
	for j, symbol := range symbols {
		if named {
			fmt.Fprintf(out, "%s:\n", importSymbol(symbol))
		}
		fmt.Fprintf(out, "\t.long .Lhint%d_%d@IMGREL, 0\n", dll, j)
	}
	out.WriteString("\t.long 0, 0\n")
}
