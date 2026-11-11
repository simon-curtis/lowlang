package asm_test

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"lowlang/internal/asm"
)

type census struct {
	mu         sync.Mutex
	mnemonics  map[string]int
	shapes     map[string]int
	directives map[string]int
}

func newCensus() *census {
	return &census{
		mnemonics:  map[string]int{},
		shapes:     map[string]int{},
		directives: map[string]int{},
	}
}

type censusCounts struct {
	mnemonics  map[string]int
	shapes     map[string]int
	directives map[string]int
}

func collectCensus(source string, object *asm.Object) censusCounts {
	counts := censusCounts{mnemonics: map[string]int{}, shapes: map[string]int{}, directives: map[string]int{}}
	for _, section := range object.Sections {
		for _, item := range section.Items {
			if item.Kind == asm.ItemInst {
				counts.mnemonics[item.Inst.Mnemonic]++
				counts.shapes[instShape(item.Inst)]++
			}
		}
	}
	for line := range strings.SplitSeq(source, "\n") {
		directive := directiveOf(line)
		if directive != "" {
			counts.directives[directive]++
		}
	}
	return counts
}

func directiveOf(line string) string {
	word := strings.TrimLeft(line, " \t")
	if end := strings.IndexAny(word, " \t"); end >= 0 {
		word = word[:end]
	}
	if len(word) < 2 || word[0] != '.' || strings.HasSuffix(word, ":") {
		return ""
	}
	return word
}

func (c *census) merge(counts censusCounts) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range counts.mnemonics {
		c.mnemonics[k] += v
	}
	for k, v := range counts.shapes {
		c.shapes[k] += v
	}
	for k, v := range counts.directives {
		c.directives[k] += v
	}
}

func instShape(inst *asm.Inst) string {
	var b strings.Builder
	if inst.Prefix&asm.PrefixLock != 0 {
		b.WriteString("lock ")
	}
	if inst.Prefix&asm.PrefixRep != 0 {
		b.WriteString("rep ")
	}
	if inst.Prefix&asm.PrefixRepne != 0 {
		b.WriteString("repne ")
	}
	b.WriteString(inst.Mnemonic)
	for i, op := range inst.Operands {
		if i == 0 {
			b.WriteByte(' ')
		} else {
			b.WriteString(", ")
		}
		shape := operandShape(op)
		b.WriteString(shape)
	}
	return b.String()
}

func operandShape(op asm.Operand) string {
	switch op.Kind {
	case asm.OpReg:
		return regShape(op.Reg)
	case asm.OpMem:
		return memShape(op)
	}
	return immShape(op)
}

func regShape(reg asm.Reg) string {
	switch reg.Class() {
	case asm.ClassGP8:
		return "r8"
	case asm.ClassGP8High:
		return "r8h"
	case asm.ClassGP16:
		return "r16"
	case asm.ClassGP32:
		return "r32"
	case asm.ClassGP64:
		return "r64"
	case asm.ClassXMM:
		return "xmm"
	case asm.ClassYMM:
		return "ymm"
	case asm.ClassSegment:
		return "seg"
	}
	return "reg"
}

func valueShape(v asm.Value) string {
	var parts []string
	if v.Sym != "" {
		parts = append(parts, "sym")
	}
	if v.Neg != "" {
		parts = append(parts, "-sym")
	}
	if v.Add != 0 || len(parts) == 0 {
		parts = append(parts, "const")
	}
	shape := strings.Join(parts, "+")
	switch v.Reloc {
	case asm.RelocGOTPCREL:
		shape += "@GOTPCREL"
	case asm.RelocImgRel:
		shape += "@IMGREL"
	case asm.RelocPLT:
		shape += "@PLT"
	}
	return shape
}

func immShape(op asm.Operand) string {
	shape := valueShape(op.Value)
	if op.Size != 0 {
		return fmt.Sprintf("%d ptr %s", op.Size*8, shape)
	}
	return shape
}

func memShape(op asm.Operand) string {
	var parts []string
	if op.Base != asm.RegNone {
		base := regShape(op.Base)
		if op.Base.Class() == asm.ClassIP {
			base = "rip"
		}
		parts = append(parts, base)
	}
	if op.Index != asm.RegNone {
		index := regShape(op.Index)
		parts = append(parts, fmt.Sprintf("%s*%d", index, op.Scale))
	}
	if disp := op.Value; disp != (asm.Value{}) {
		shape := valueShape(disp)
		parts = append(parts, shape)
	}
	seg := ""
	if op.Seg != asm.RegNone {
		seg = fmt.Sprintf("%s:", op.Seg)
	}
	return fmt.Sprintf("%sm%d[%s]", seg, op.Size*8, strings.Join(parts, "+"))
}

func (c *census) summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "census: %d mnemonics, %d distinct operand forms, %d directives\n", len(c.mnemonics), len(c.shapes), len(c.directives))
	directives := rankedList(c.directives)
	mnemonics := rankedList(c.mnemonics)
	fmt.Fprintf(&b, "directives: %s\n", directives)
	fmt.Fprintf(&b, "mnemonics: %s\n", mnemonics)
	b.WriteString("operand forms:\n")
	for _, name := range rankedKeys(c.shapes) {
		fmt.Fprintf(&b, "  %9d  %s\n", c.shapes[name], name)
	}
	return b.String()
}

func rankedKeys(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	return keys
}

func rankedList(counts map[string]int) string {
	var parts []string
	for _, k := range rankedKeys(counts) {
		parts = append(parts, fmt.Sprintf("%s:%d", k, counts[k]))
	}
	return strings.Join(parts, " ")
}
