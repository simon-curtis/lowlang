package amd64

import (
	"fmt"
	"strings"

	"lowlang/internal/ir"
	"lowlang/internal/target"
)

const sysvVectorRegs = 8

var sysvIntRegs = []string{"rdi", "rsi", "rdx", "rcx", "r8", "r9"}

type Platform interface {
	OS() target.OS
	entry() string
	header() string
	runtime() string
	classify(sig *ir.CSig) *cPlan
	intRegs() []string
	vectorRegs() int
	registerSlots() int
	preservedVectors() []string
	thunkSaved() []string
	positional() bool
	syscall(g *Generator)
	variadicCount(g *Generator, plan *cPlan)
	reserve(g *Generator, size int)
	externalTarget(symbol string) string
	symbolAddress(symbol string) string
	imports(out *strings.Builder, prog *ir.Program)
	dynamic(prog *ir.Program) bool
	shims(g *Generator, symbols []string)
}

func NewPlatform(os target.OS) Platform {
	if os == target.Windows {
		return win64{}
	}
	return sysv{}
}

type sysv struct{}

func (sysv) OS() target.OS {
	return target.Linux
}

func (sysv) entry() string {
	return ELFEntrySymbol
}

func (sysv) header() string {
	return linuxHeader
}

func (sysv) runtime() string {
	return linuxRuntime
}

func (sysv) classify(sig *ir.CSig) *cPlan {
	return sysvPlan(sig)
}

func (sysv) intRegs() []string {
	return sysvIntRegs
}

func (sysv) vectorRegs() int {
	return sysvVectorRegs
}

func (sysv) registerSlots() int {
	return 0
}

func (sysv) preservedVectors() []string {
	return nil
}

func (sysv) thunkSaved() []string {
	return abi.saved
}

func (sysv) positional() bool {
	return false
}

func (sysv) syscall(g *Generator) {
	g.emit("syscall")
}

func (sysv) variadicCount(g *Generator, plan *cPlan) {
	g.emit("mov eax, %d", plan.vectors)
}

func (sysv) reserve(g *Generator, size int) {
	if size > 0 {
		g.emit("sub rsp, %d", size)
	}
}

func (sysv) externalTarget(symbol string) string {
	return symbol
}

func (sysv) symbolAddress(symbol string) string {
	return fmt.Sprintf("qword ptr [rip+%s@GOTPCREL]", symbol)
}

func (sysv) imports(*strings.Builder, *ir.Program) {}

func (sysv) dynamic(prog *ir.Program) bool {
	return len(prog.Libraries()) > 0
}

func (sysv) shims(g *Generator, symbols []string) {
	for _, symbol := range symbols {
		switch symbol {
		case "atexit":
			g.shim(symbol, "__cxa_atexit", "xor esi, esi", "xor edx, edx")
		case "at_quick_exit":
			g.shim(symbol, "__cxa_at_quick_exit", "xor esi, esi")
		}
	}
}
