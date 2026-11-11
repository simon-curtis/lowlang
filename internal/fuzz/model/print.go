package model

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const noKind Kind = -1

type printer struct {
	out      strings.Builder
	depth    int
	result   Kind
	labelled map[int]bool
	loops    []int
}

func (p *Program) String() string {
	var pr printer
	for _, f := range p.Funcs {
		pr.function(f)
		pr.out.WriteString("\n")
	}
	pr.out.WriteString("fn main() {\n")
	pr.depth = 1
	pr.block(p.Main)
	pr.out.WriteString("}\n")
	return pr.out.String()
}

func (pr *printer) function(f *Func) {
	pr.labelled = map[int]bool{}
	pr.findLabels(f.Body)
	pr.result = f.Result
	params := make([]string, len(f.Params))
	for i, param := range f.Params {
		params[i] = fmt.Sprintf("%s: %s", param.Name, param.Kind)
	}
	fmt.Fprintf(&pr.out, "fn %s(%s) -> %s {\n", f.Name, strings.Join(params, ", "), f.Result)
	pr.depth = 1
	pr.block(f.Body)
	pr.indent()
	pr.expr(f.Tail, f.Result)
	pr.out.WriteString("\n}\n")
	pr.depth = 0
}

func (pr *printer) findLabels(stmts []Stmt) {
	for _, s := range stmts {
		switch s := s.(type) {
		case *IfStmt:
			pr.findLabels(s.Then)
			pr.findLabels(s.Else)
		case *Range:
			pr.loops = append(pr.loops, s.ID)
			pr.findLabels(s.Body)
			pr.loops = pr.loops[:len(pr.loops)-1]
		case *While:
			pr.loops = append(pr.loops, s.ID)
			pr.findLabels(s.Body)
			pr.loops = pr.loops[:len(pr.loops)-1]
		case *Jump:
			if len(pr.loops) > 0 && s.Target != pr.loops[len(pr.loops)-1] {
				pr.labelled[s.Target] = true
			}
		}
	}
}

func (pr *printer) indent() {
	for range pr.depth {
		pr.out.WriteString("    ")
	}
}

func (pr *printer) block(stmts []Stmt) {
	for _, s := range stmts {
		pr.stmt(s)
	}
}

func (pr *printer) nested(stmts []Stmt) {
	pr.depth++
	pr.block(stmts)
	pr.depth--
}

func (pr *printer) label(id int) {
	if pr.labelled[id] {
		fmt.Fprintf(&pr.out, "L%d: ", id)
	}
}

func (pr *printer) stmt(s Stmt) {
	pr.indent()
	switch s := s.(type) {
	case *Let:
		keyword := "let"
		if s.Mutable {
			keyword = "var"
		}
		fmt.Fprintf(&pr.out, "%s %s: %s = ", keyword, s.Name, s.Kind)
		pr.expr(s.Init, s.Kind)
		pr.out.WriteString("\n")
	case *Assign:
		want := s.Kind
		if s.Op.IsShift() {
			want = noKind
		}
		sym := s.Op.Symbol()
		if s.Op != Set {
			sym += "="
		}
		fmt.Fprintf(&pr.out, "%s %s ", s.Name, sym)
		pr.expr(s.Value, want)
		pr.out.WriteString("\n")
	case *IfStmt:
		pr.out.WriteString("if ")
		pr.expr(s.Cond, noKind)
		pr.out.WriteString(" {\n")
		pr.nested(s.Then)
		pr.indent()
		if len(s.Else) == 0 {
			pr.out.WriteString("}\n")
			return
		}
		pr.out.WriteString("} else {\n")
		pr.nested(s.Else)
		pr.indent()
		pr.out.WriteString("}\n")
	case *Range:
		pr.label(s.ID)
		fmt.Fprintf(&pr.out, "for %s in (", s.Var)
		pr.expr(s.Lo, noKind)
		pr.out.WriteString(")..(")
		pr.expr(s.Hi, noKind)
		pr.out.WriteString(") {\n")
		pr.loopBody(s.ID, s.Body)
	case *While:
		fmt.Fprintf(&pr.out, "var %s: i64 = 0\n", s.Counter)
		pr.indent()
		pr.label(s.ID)
		fmt.Fprintf(&pr.out, "for %s < %d", s.Counter, s.Bound)
		if s.Cond != nil {
			pr.out.WriteString(" && ")
			pr.expr(s.Cond, noKind)
		}
		pr.out.WriteString(" {\n")
		pr.depth++
		pr.indent()
		fmt.Fprintf(&pr.out, "%s += 1\n", s.Counter)
		pr.depth--
		pr.loopBody(s.ID, s.Body)
	case *Jump:
		word := "break"
		if s.Continue {
			word = "continue"
		}
		pr.out.WriteString(word)
		if len(pr.loops) == 0 || pr.loops[len(pr.loops)-1] != s.Target {
			fmt.Fprintf(&pr.out, " L%d", s.Target)
		}
		pr.out.WriteString("\n")
	case *Return:
		pr.out.WriteString("return ")
		pr.expr(s.Value, pr.result)
		pr.out.WriteString("\n")
	case *Print:
		pr.out.WriteString("print(")
		pr.expr(s.Value, noKind)
		pr.out.WriteString(")\n")
	}
}

func (pr *printer) loopBody(id int, body []Stmt) {
	pr.loops = append(pr.loops, id)
	pr.nested(body)
	pr.loops = pr.loops[:len(pr.loops)-1]
	pr.indent()
	pr.out.WriteString("}\n")
}

func isLit(e Expr) bool {
	_, ok := e.(*Lit)
	return ok
}

func siblingWants(kind Kind, l, r Expr) (Kind, Kind) {
	switch {
	case isLit(l) && !isLit(r):
		return kind, noKind
	case isLit(r) && !isLit(l):
		return noKind, kind
	}
	return noKind, noKind
}

func (pr *printer) expr(e Expr, want Kind) {
	switch e := e.(type) {
	case *Lit:
		pr.lit(e, want)
	case *Var:
		pr.out.WriteString(e.Name)
	case *Unary:
		fmt.Fprintf(&pr.out, "(%s", e.Op.Symbol())
		pr.expr(e.X, noKind)
		pr.out.WriteString(")")
	case *Binary:
		pr.binary(e)
	case *Convert:
		fmt.Fprintf(&pr.out, "%s(", e.To)
		pr.expr(e.X, noKind)
		pr.out.WriteString(")")
	case *IfExpr:
		pr.out.WriteString("(if ")
		pr.expr(e.Cond, noKind)
		pr.out.WriteString(" { ")
		pr.expr(e.Then, noKind)
		pr.out.WriteString(" } else { ")
		pr.expr(e.Else, noKind)
		pr.out.WriteString(" })")
	case *Call:
		pr.out.WriteString(e.Callee)
		pr.out.WriteString("(")
		for i, a := range e.Args {
			if i > 0 {
				pr.out.WriteString(", ")
			}
			pr.expr(a, a.Type())
		}
		pr.out.WriteString(")")
	case *Math:
		fmt.Fprintf(&pr.out, "%s(", e.Op)
		pr.expr(e.X, noKind)
		pr.out.WriteString(")")
	}
}

func (pr *printer) binary(e *Binary) {
	leftWant, rightWant := noKind, noKind
	if !e.Op.IsLogical() && !e.Op.IsShift() {
		leftWant, rightWant = siblingWants(e.L.Type(), e.L, e.R)
	}
	pr.out.WriteString("(")
	pr.expr(e.L, leftWant)
	fmt.Fprintf(&pr.out, " %s ", e.Op.Symbol())
	pr.expr(e.R, rightWant)
	pr.out.WriteString(")")
}

func (pr *printer) lit(e *Lit, want Kind) {
	switch {
	case e.Kind == Bool:
		pr.out.WriteString(strconv.FormatBool(e.Bits != 0))
	case e.Kind.IsFloat():
		pr.floatLit(e, want)
	default:
		pr.intLit(e, want)
	}
}

func bare(kind, want Kind) bool {
	if want == noKind {
		return kind == I64 || kind == F64
	}
	return want == kind
}

func (pr *printer) floatLit(e *Lit, want Kind) {
	text := strconv.FormatFloat(math.Abs(e.Float), 'f', -1, 64)
	if !strings.Contains(text, ".") {
		text += ".0"
	}
	if math.Signbit(e.Float) {
		text = fmt.Sprintf("(-%s)", text)
	}
	if bare(e.Kind, want) {
		pr.out.WriteString(text)
		return
	}
	fmt.Fprintf(&pr.out, "%s(%s)", e.Kind, text)
}

func (pr *printer) intLit(e *Lit, want Kind) {
	switch {
	case bare(e.Kind, want) && !compositeMinimum(e):
		pr.out.WriteString(intText(e.Kind, e.Bits, true))
	case e.Kind == U64 && e.Bits > math.MaxInt64:
		fmt.Fprintf(&pr.out, "(~u64(%d))", ^e.Bits)
	default:
		fmt.Fprintf(&pr.out, "%s(%s)", e.Kind, intText(e.Kind, e.Bits, false))
	}
}

func compositeMinimum(e *Lit) bool {
	return e.Kind != I64 && e.Kind.Signed() && e.Kind.Extend(e.Bits) == e.Kind.MinSigned()
}

func intText(kind Kind, bits uint64, bareContext bool) string {
	if !kind.Signed() {
		return strconv.FormatUint(kind.Wrap(bits), 10)
	}
	v := kind.Extend(bits)
	minimum := int64(math.MinInt64)
	if bareContext {
		minimum = kind.MinSigned()
	}
	switch {
	case v >= 0:
		return strconv.FormatInt(v, 10)
	case v == minimum:
		return fmt.Sprintf("(-%d - 1)", -(v + 1))
	default:
		return fmt.Sprintf("(-%d)", -v)
	}
}
