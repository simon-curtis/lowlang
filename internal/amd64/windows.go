package amd64

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"lowlang/internal/ir"
	"lowlang/internal/target"
)

const (
	gHi          = 48
	gLo          = 56
	gLimit       = 96
	mCurG        = 8
	mOSBase      = 104
	mOSLimit     = 112
	mOSDealloc   = 120
	tebBase      = 0x8
	tebLimit     = 0x10
	tebDealloc   = 0x1478
	pageSize     = 4096
	cRuntimeDLL  = "msvcrt.dll"
	kernelDLL    = "kernel32.dll"
	synchDLL     = "api-ms-win-core-synch-l1-2-0.dll"
	importPrefix = "__imp_"
)

func (g *Generator) syscall() {
	if g.os == target.Windows {
		g.emit("call rt_panic_syscall")
		return
	}
	g.emit("syscall")
}

var winArgRegs = []string{"rcx", "rdx", "r8", "r9"}

var runtimeImports = map[string][]string{
	kernelDLL: {
		"AddVectoredExceptionHandler", "CloseHandle", "CreateEventW", "CreateThread", "ExitProcess",
		"GetStdHandle", "GetSystemInfo", "GetSystemTimePreciseAsFileTime", "QueryPerformanceCounter", "QueryPerformanceFrequency",
		"RegisterWaitForSingleObject", "SetEvent", "Sleep", "SwitchToThread", "TlsAlloc",
		"UnregisterWait", "VirtualAlloc", "WaitForSingleObject", "WriteFile",
	},
	synchDLL: {"WaitOnAddress", "WakeByAddressAll", "WakeByAddressSingle"},
}

func importSymbol(symbol string) string {
	return fmt.Sprintf("%s%s", importPrefix, symbol)
}

func dllName(library string) string {
	switch library {
	case "c", "m":
		return cRuntimeDLL
	}
	if strings.HasSuffix(strings.ToLower(library), ".dll") {
		return strings.ToLower(library)
	}
	return fmt.Sprintf("%s.dll", strings.ToLower(library))
}

func (g *Generator) cIntRegs() []string {
	if g.os == target.Windows {
		return winArgRegs
	}
	return argRegs
}

func (g *Generator) cVectorRegs() int {
	if g.os == target.Windows {
		return len(winArgRegs)
	}
	return 8
}

func (g *Generator) cRegisterSlots() int {
	if g.os == target.Windows {
		return len(winArgRegs)
	}
	return 0
}

func (g *Generator) thunkSaved() []string {
	if g.os == target.Windows {
		return append(slices.Clone(calleeSaved), "rdi", "rsi")
	}
	return calleeSaved
}

func (g *Generator) tls(reg string, slot int) {
	if g.os != target.Windows {
		g.emit("mov %s, qword ptr fs:[%d]", reg, slot)
		return
	}
	g.emit("mov %s, %s", reg, g.tlsOperand(reg, slot))
}

func (g *Generator) tlsOperand(scratch string, slot int) string {
	if g.os != target.Windows {
		return fmt.Sprintf("fs:[%d]", slot)
	}
	g.emit("mov %s, qword ptr [rip+rt_tls_slot]", scratch)
	g.emit("mov %s, qword ptr gs:[%s]", scratch, scratch)
	return fmt.Sprintf("[%s%+d]", scratch, slot)
}

func (g *Generator) reserve(size int) {
	if size == 0 {
		return
	}
	if g.os != target.Windows || size < pageSize {
		g.emit("sub rsp, %d", size)
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

func (g *Generator) tebToOS(gReg, mReg string) {
	g.emit("mov r11, qword ptr gs:[%d]", tebLimit)
	g.emit("mov [%s+%d], r11", gReg, gLimit)
	for _, f := range []struct{ teb, m int }{{tebBase, mOSBase}, {tebLimit, mOSLimit}, {tebDealloc, mOSDealloc}} {
		g.emit("mov r11, [%s+%d]", mReg, f.m)
		g.emit("mov qword ptr gs:[%d], r11", f.teb)
	}
}

func (g *Generator) tebEnterG(gReg, mReg string) {
	g.emit("mov r11, qword ptr gs:[%d]", tebLimit)
	g.emit("mov [%s+%d], r11", mReg, mOSLimit)
	for _, f := range []struct{ teb, g int }{{tebBase, gHi}, {tebLimit, gLimit}, {tebDealloc, gLo}} {
		g.emit("mov r11, [%s+%d]", gReg, f.g)
		g.emit("mov qword ptr gs:[%d], r11", f.teb)
	}
}

func (g *Generator) tebToG(mReg string) {
	g.emit("mov r10, [%s+%d]", mReg, mCurG)
	g.tebEnterG("r10", mReg)
}

func (g *Generator) win64Args(fr *frame, plan *ir.CPlan, args []*vreg, moves []pmove) []pmove {
	first := 0
	if plan.Memory {
		first = 1
	}
	for i, a := range args {
		slot, src := first+i, fr.loc(a)
		if slot >= len(winArgRegs) {
			g.stackArg(slot, src)
			continue
		}
		if plan.Args[i] == ir.Vector {
			g.toXmm(fmt.Sprintf("xmm%d", slot), src)
		}
		moves = append(moves, pmove{dst: winArgRegs[slot], src: src})
	}
	return moves
}

func importTable(prog *ir.Program) map[string][]string {
	table := map[string][]string{}
	for dll, symbols := range runtimeImports {
		table[dll] = slices.Clone(symbols)
	}
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
		fmt.Fprintf(out, "\n\t.section .idata$2,\"dr\"\n\t.long .Lilt%d@IMGREL, 0, 0, .Ldll%d@IMGREL, .Liat%d@IMGREL\n", i, i, i)
	}
	out.WriteString("\n\t.section .idata$3,\"dr\"\n\t.long 0, 0, 0, 0, 0\n")
	for i, dll := range dlls {
		writeThunks(out, "4", fmt.Sprintf(".Lilt%d", i), i, table[dll], false)
		writeThunks(out, "5", fmt.Sprintf(".Liat%d", i), i, table[dll], true)
		out.WriteString("\n\t.section .idata$6,\"dr\"\n")
		for j, symbol := range table[dll] {
			fmt.Fprintf(out, "\t.p2align 1\n.Lhint%d_%d:\n\t.short 0\n\t.asciz %q\n", i, j, symbol)
		}
		fmt.Fprintf(out, "\n\t.section .idata$7,\"dr\"\n.Ldll%d:\n\t.asciz %q\n", i, dll)
	}
}

func writeThunks(out *strings.Builder, section, label string, dll int, symbols []string, named bool) {
	fmt.Fprintf(out, "\n\t.section .idata$%s,\"dr\"\n\t.p2align 3\n%s:\n", section, label)
	for j, symbol := range symbols {
		if named {
			fmt.Fprintf(out, "%s:\n", importSymbol(symbol))
		}
		fmt.Fprintf(out, "\t.long .Lhint%d_%d@IMGREL, 0\n", dll, j)
	}
	out.WriteString("\t.long 0, 0\n")
}
