package amd64

import (
	"fmt"
	"math"
)

type jumpTable struct {
	label   string
	targets []string
}

func (g *Generator) jumpTable(fr *frame, mb *mblock) {
	table := mb.table
	fallback := fr.final(mb.succs[0])
	g.move("r10", fr.loc(mb.control))
	g.rebase("r10", table.Low)
	g.emit("cmp r10, %d", len(table.Cases)-1)
	g.emit("ja %s", fallback.label)
	label := g.newLabel()
	g.emit("lea r11, [rip+%s]", label)
	g.emit("movsxd r10, dword ptr [r11+r10*4]")
	g.emit("add r10, r11")
	g.emit("jmp r10")
	targets := make([]string, len(table.Cases))
	for i, c := range table.Cases {
		targets[i] = fr.final(mb.succs[c]).label
	}
	g.jumps = append(g.jumps, jumpTable{label: label, targets: targets})
}

func (g *Generator) rebase(reg string, low int64) {
	switch {
	case low == 0:
	case low >= math.MinInt32 && low <= math.MaxInt32:
		g.emit("sub %s, %d", reg, low)
	default:
		g.emit("mov r11, %d", low)
		g.emit("sub %s, r11", reg)
	}
}

func (g *Generator) flushJumpTables() {
	for _, t := range g.jumps {
		g.align(2)
		g.label(t.label)
		for _, target := range t.targets {
			fmt.Fprintf(&g.text, "\t.long %s - %s\n", target, t.label)
		}
	}
	g.jumps = nil
}
