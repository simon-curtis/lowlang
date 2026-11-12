package regex

import (
	"fmt"
	"strings"

	"lowlang/internal/lexer"
)

const (
	Module       = "std/regex"
	Generator    = "generated"
	Result       = "Regex"
	maxGenerated = 2000
	inlineRanges = 4
)

func Generate(name, pattern string, prog *Program) string {
	g := &generator{name: name, insts: prog.Insts}
	if len(prog.Insts) > maxGenerated {
		g.interpreted(pattern)
		return g.out.String()
	}
	g.analyse()
	g.constructor(prog.Groups)
	g.start()
	g.step()
	for pc := range g.insts {
		if g.called[pc] {
			g.add(pc)
		}
	}
	for _, pc := range g.leaves {
		if g.insts[pc].Op == Class && len(g.insts[pc].Ranges) > inlineRanges {
			g.class(pc)
		}
	}
	g.skip()
	return g.out.String()
}

type generator struct {
	name   string
	insts  []Inst
	called []bool
	leaves []int
	stops  []Range
	out    strings.Builder
	temps  int
}

func (g *generator) line(format string, args ...any) {
	fmt.Fprintf(&g.out, format, args...)
	g.out.WriteByte('\n')
}

func (g *generator) temp(prefix string) string {
	g.temps++
	return fmt.Sprintf("%s%d", prefix, g.temps)
}

func (g *generator) interpreted(pattern string) {
	g.line("fn %s() -> %s {", g.name, Result)
	g.line("compile(%s) or __panic(%s)", lexer.Quote(pattern), lexer.Quote("regex: a generated pattern failed to compile"))
	g.line("}")
}

func (inst Inst) consumes() bool {
	return inst.Op == Char || inst.Op == Class
}

func (inst Inst) leaf() bool {
	return inst.consumes() || inst.Op == Done
}

func (inst Inst) epsilons(pc int) []int {
	switch inst.Op {
	case Split:
		return []int{inst.X, inst.Y}
	case Jump:
		return []int{inst.X}
	case Save, Check:
		return []int{pc + 1}
	}
	return nil
}

func (g *generator) analyse() {
	n := len(g.insts)
	reached := make([]bool, n)
	incoming := make([]int, n)
	entry := make([]bool, n)
	entry[0] = true
	work := []int{0}
	reached[0] = true
	for len(work) > 0 {
		pc := work[len(work)-1]
		work = work[:len(work)-1]
		inst := g.insts[pc]
		next := inst.epsilons(pc)
		if inst.consumes() {
			entry[pc+1] = true
			next = []int{pc + 1}
		}
		for _, to := range next {
			if !inst.consumes() {
				incoming[to]++
			}
			if !reached[to] {
				reached[to] = true
				work = append(work, to)
			}
		}
	}
	g.called = make([]bool, n)
	for pc := range n {
		g.called[pc] = reached[pc] && (entry[pc] || incoming[pc] > 1)
		if reached[pc] && g.insts[pc].leaf() {
			g.leaves = append(g.leaves, pc)
		}
	}
	g.stops = g.first()
}

func (g *generator) adder(pc int) string {
	return fmt.Sprintf("%s_a%d", g.name, pc)
}

func (g *generator) constructor(groups int) {
	skip := "stay"
	if g.stops != nil {
		skip = g.name + "_skip"
	}
	g.line("fn %s() -> %s {", g.name, Result)
	g.line("let engine = Engine { program: [], size: %d, start: %s_start, step: %s_step, skip: %s }", len(g.insts), g.name, g.name, skip)
	g.line("Regex { engine, groups: %d }", groups)
	g.line("}")
}

func (g *generator) start() {
	g.line("fn %s_start(mut core: Core, mut q: Queue, pos: i64) {", g.name)
	g.line("for i in 0..core.width {\ncore.scratch[i] = -1\n}")
	g.line("%s(mut core, mut q, pos)", g.adder(0))
	g.line("}")
}

func (g *generator) step() {
	g.line("fn %s_step(mut m: Machine, s: Search, pos: i64, c: u32, next: i64) {", g.name)
	g.line("for j in 0..m.run.len {")
	g.line("let pc = m.run.pcs[j]")
	g.dispatch(g.leaves)
	g.line("}")
	g.line("}")
}

func (g *generator) dispatch(pcs []int) {
	if len(pcs) <= inlineRanges {
		for i, pc := range pcs {
			if i > 0 {
				g.out.WriteString("} else ")
			}
			g.line("if pc == %d {", pc)
			g.act(pc)
		}
		if len(pcs) > 0 {
			g.line("}")
		}
		return
	}
	mid := len(pcs) / 2
	g.line("if pc < %d {", pcs[mid])
	g.dispatch(pcs[:mid])
	g.line("} else {")
	g.dispatch(pcs[mid:])
	g.line("}")
}

func (g *generator) act(pc int) {
	inst := g.insts[pc]
	switch inst.Op {
	case Done:
		g.line("if accepts(m.core, s, pos) {\nkeep(mut m, j)\nreturn\n}")
		return
	case Char:
		g.line("if c == %d {", inst.C)
	case Class:
		g.line("if %s {", g.member(pc))
	}
	g.line("resume(mut m, j)")
	g.line("%s(mut m.core, mut m.next, next)", g.adder(pc+1))
	g.line("}")
}

func (g *generator) member(pc int) string {
	ranges := g.insts[pc].Ranges
	if len(ranges) > inlineRanges {
		return fmt.Sprintf("%s_c%d(c)", g.name, pc)
	}
	return within("c", ranges)
}

func within(v string, ranges []Range) string {
	if len(ranges) == 0 {
		return "false"
	}
	terms := make([]string, len(ranges))
	for i, r := range ranges {
		switch {
		case r.Lo == r.Hi:
			terms[i] = fmt.Sprintf("%s == %d", v, r.Lo)
		case r.Lo == 0:
			terms[i] = fmt.Sprintf("%s <= %d", v, r.Hi)
		default:
			terms[i] = fmt.Sprintf("%s >= %d && %s <= %d", v, r.Lo, v, r.Hi)
		}
	}
	return strings.Join(terms, " || ")
}

func (g *generator) class(pc int) {
	g.line("fn %s_c%d(c: u32) -> bool {", g.name, pc)
	g.search(g.insts[pc].Ranges, true)
	g.line("}")
}

func (g *generator) search(ranges []Range, tail bool) {
	if len(ranges) <= inlineRanges {
		if tail {
			g.line("%s", within("c", ranges))
		} else {
			g.line("return %s", within("c", ranges))
		}
		return
	}
	mid := len(ranges) / 2
	g.line("if c < %d {", ranges[mid].Lo)
	g.search(ranges[:mid], false)
	g.line("}")
	g.search(ranges[mid:], tail)
}

func (g *generator) add(pc int) {
	g.line("fn %s(mut core: Core, mut q: Queue, pos: i64) {", g.adder(pc))
	g.line("let at = q.sparse[%d]", pc)
	g.line("if at < q.len && q.pcs[at] == %d {\nreturn\n}", pc)
	if !g.insts[pc].leaf() {
		g.mark(pc)
	}
	g.expand(pc)
	g.line("}")
}

func (g *generator) mark(pc int) string {
	k := g.temp("k")
	g.line("let %s = q.len", k)
	g.line("q.sparse[%d] = %s", pc, k)
	g.line("q.pcs[%s] = %d", k, pc)
	g.line("q.len = %s + 1", k)
	return k
}

func (g *generator) follow(pc int) {
	if g.called[pc] {
		g.line("%s(mut core, mut q, pos)", g.adder(pc))
		return
	}
	g.expand(pc)
}

func (g *generator) expand(pc int) {
	inst := g.insts[pc]
	switch inst.Op {
	case Char, Class, Done:
		k := g.mark(pc)
		g.line("for i in 0..core.width {\nq.slots[%s * core.width + i] = core.scratch[i]\n}", k)
	case Jump:
		g.follow(inst.X)
	case Split:
		g.follow(inst.X)
		g.follow(inst.Y)
	case Save:
		old := g.temp("o")
		g.line("let %s = core.scratch[%d]", old, inst.X)
		g.line("core.scratch[%d] = pos", inst.X)
		g.follow(pc + 1)
		g.line("core.scratch[%d] = %s", inst.X, old)
	case Check:
		g.line("if %s {", holds(inst.Look))
		g.follow(pc + 1)
		g.line("}")
	}
}

func holds(look Look) string {
	switch look {
	case TextStart:
		return "pos == 0"
	case TextEnd:
		return "pos == core.input.len"
	case EndOrFinalNewline:
		return "pos == core.input.len || pos == core.input.len - 1 && core.input[pos] == 10"
	case LineStart:
		return "pos == 0 || core.input[pos - 1] == 10"
	case LineEnd:
		return "pos == core.input.len || core.input[pos] == 10"
	case WordBoundary:
		return "word_before(core.input, pos) != word_at(core.input, pos)"
	}
	return "word_before(core.input, pos) == word_at(core.input, pos)"
}

func (g *generator) first() []Range {
	seen := make([]bool, len(g.insts))
	var ranges []Range
	work := []int{0}
	seen[0] = true
	for len(work) > 0 {
		pc := work[len(work)-1]
		work = work[:len(work)-1]
		inst := g.insts[pc]
		switch inst.Op {
		case Done:
			return nil
		case Char:
			ranges = append(ranges, Range{inst.C, inst.C})
		case Class:
			ranges = append(ranges, inst.Ranges...)
		}
		for _, to := range inst.epsilons(pc) {
			if !seen[to] {
				seen[to] = true
				work = append(work, to)
			}
		}
	}
	ascii := ascii(normalize(ranges))
	if len(ascii) == 1 && ascii[0] == (Range{0, 127}) {
		return nil
	}
	return normalize(append(ascii, Range{128, 255}))
}

func ascii(ranges []Range) []Range {
	var out []Range
	for _, r := range ranges {
		if r.Lo <= 127 {
			out = append(out, Range{r.Lo, min(r.Hi, 127)})
		}
	}
	return out
}

func (g *generator) skip() {
	if g.stops == nil {
		return
	}
	g.line("fn %s_skip(input: str, from: i64) -> i64 {", g.name)
	g.line("var i = from")
	g.line("for i < input.len {")
	g.line("let b = input[i]")
	g.line("if %s {\nreturn i\n}", within("b", g.stops))
	g.line("i += 1")
	g.line("}")
	g.line("i")
	g.line("}")
}
