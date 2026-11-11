package amd64

import (
	"fmt"
	"strings"
)

func (g *Generator) cellAddress(index int64) string {
	return fmt.Sprintf("[rip+rt_cells+%d]", 8*index)
}

func writeCells(out *strings.Builder, count int) {
	fmt.Fprintf(out, "\n\t.data\n\t.p2align 3\n\t.globl rt_cells\nrt_cells:\n\t.zero %d\n\t.globl rt_cells_end\nrt_cells_end:\n", 8*count)
}
