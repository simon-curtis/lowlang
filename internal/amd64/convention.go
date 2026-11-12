package amd64

import (
	"fmt"
	"slices"
)

const stackAlignment = 16

type convention struct {
	args     []string
	results  []string
	caller   []string
	saved    []string
	syscall  []string
	grow     []string
	frameTop int
}

var abi = convention{
	args:     []string{"rdi", "rsi", "rdx", "rcx", "r8", "r9"},
	results:  []string{"rax", "rdx"},
	caller:   []string{"rcx", "rsi", "rdi", "r8", "r9"},
	saved:    []string{"rbx", "r12", "r13", "r14", "r15"},
	syscall:  []string{"rax", "rdi", "rsi", "rdx", "r10", "r8", "r9"},
	grow:     []string{"rax", "rdx", "r10"},
	frameTop: 16,
}

func (c convention) integers() []string {
	return slices.Concat(c.caller, c.saved)
}

func (c convention) stackWords(params int) int {
	return max(0, params-len(c.args))
}

func (c convention) argument(k int) (string, bool) {
	if k < len(c.args) {
		return c.args[k], true
	}
	return "", false
}

func (c convention) incoming(k int) string {
	return fmt.Sprintf("qword ptr [rbp+%d]", c.frameTop+8*(k-len(c.args)))
}

func (c convention) outgoing(k int) string {
	return fmt.Sprintf("qword ptr [rsp+%d]", 8*(k-len(c.args)))
}

func (c convention) stackResult(params, k int) string {
	return fmt.Sprintf("qword ptr [rsp+%d]", 8*(c.stackWords(params)+k))
}

func (c convention) returnsInRegisters(words int) bool {
	return words <= len(c.results)
}

func (c convention) alignedBytes(words int) int {
	return (8*words + stackAlignment - 1) / stackAlignment * stackAlignment
}

func (c convention) savedBytes() int {
	return 8 * len(c.saved)
}

func (c convention) resultsOffset(params int) int {
	return c.frameTop + 8*c.stackWords(params)
}
