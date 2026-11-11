package asm

import "fmt"

type symState uint8

const (
	symUndefined symState = iota
	symLabel
	symAbsolute
)

type symbol struct {
	name    string
	state   symState
	section *secBuilder
	value   int64
	frag    int32
	global  bool
}

func (a *assembler) lookup(name string) *symbol {
	if sym, ok := a.syms[name]; ok {
		return sym
	}
	sym := &symbol{name: name}
	a.syms[name] = sym
	a.symOrder = append(a.symOrder, sym)
	return sym
}

func (a *assembler) defineLabel(name string) {
	sym := a.lookup(name)
	if sym.state != symUndefined {
		a.fail("symbol %s is already defined", name)
	}
	sym.state = symLabel
	sym.section = a.cur
	sym.value, sym.frag = a.cur.mark()
}

func (a *assembler) defineNumericLabel(n int) {
	a.numericLabels[n]++
	name := fmt.Sprintf(".L%d\x02%d", n, a.numericLabels[n])
	a.defineLabel(name)
}

func (a *assembler) defineConstant(name string, value int64) {
	sym := a.lookup(name)
	if sym.state == symLabel {
		a.fail("symbol %s is already defined as a label", name)
	}
	sym.state = symAbsolute
	sym.value = value
}
