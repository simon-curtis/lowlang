package model

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

var (
	ErrBudget    = errors.New("step budget exhausted")
	ErrUndefined = errors.New("program has undefined behaviour")
)

type Value struct {
	Bits  uint64
	Float float64
}

type Outcome struct {
	Stdout string
	Panic  string
}

func (o Outcome) ExitCode() int {
	if o.Panic != "" {
		return 2
	}
	return 0
}

func (o Outcome) Stderr() string {
	if o.Panic == "" {
		return ""
	}
	return fmt.Sprintf("panic: %s\n", o.Panic)
}

type trap struct {
	message string
}

type abort struct {
	err error
}

type signal int

const (
	flowNone signal = iota
	flowBreak
	flowContinue
	flowReturn
)

type flow struct {
	signal signal
	target int
	value  Value
}

type frame struct {
	vars map[string]Value
}

type interpreter struct {
	funcs  map[string]*Func
	out    strings.Builder
	steps  int
	budget int
}

func Run(p *Program, budget int) (outcome Outcome, err error) {
	in := &interpreter{funcs: map[string]*Func{}, budget: budget}
	for _, f := range p.Funcs {
		in.funcs[f.Name] = f
	}
	defer func() {
		r := recover()
		switch r := r.(type) {
		case nil:
			outcome.Stdout = in.out.String()
		case trap:
			outcome = Outcome{Stdout: in.out.String(), Panic: r.message}
		case abort:
			err = r.err
		default:
			panic(r)
		}
	}()
	in.block(p.Main, &frame{vars: map[string]Value{}})
	return outcome, nil
}

func (in *interpreter) tick() {
	in.steps++
	if in.steps > in.budget {
		panic(abort{ErrBudget})
	}
}

func (in *interpreter) block(stmts []Stmt, f *frame) flow {
	for _, s := range stmts {
		if fl := in.stmt(s, f); fl.signal != flowNone {
			return fl
		}
	}
	return flow{}
}

func (in *interpreter) stmt(s Stmt, f *frame) flow {
	in.tick()
	switch s := s.(type) {
	case *Let:
		f.vars[s.Name] = in.eval(s.Init, f)
	case *Assign:
		in.assign(s, f)
	case *IfStmt:
		if in.eval(s.Cond, f).Bits != 0 {
			return in.block(s.Then, f)
		}
		return in.block(s.Else, f)
	case *Range:
		return in.rangeLoop(s, f)
	case *While:
		return in.whileLoop(s, f)
	case *Jump:
		if s.Continue {
			return flow{signal: flowContinue, target: s.Target}
		}
		return flow{signal: flowBreak, target: s.Target}
	case *Return:
		return flow{signal: flowReturn, value: in.eval(s.Value, f)}
	case *Print:
		in.print(s.Value.Type(), in.eval(s.Value, f))
	}
	return flow{}
}

func (in *interpreter) assign(s *Assign, f *frame) {
	value := in.eval(s.Value, f)
	if s.Op == Set {
		f.vars[s.Name] = value
		return
	}
	f.vars[s.Name] = binary(s.Op, s.Kind, s.Value.Type(), f.vars[s.Name], value)
}

func (in *interpreter) print(kind Kind, v Value) {
	switch {
	case kind == Bool:
		in.out.WriteString(strconv.FormatBool(v.Bits != 0))
	case kind.Signed():
		in.out.WriteString(strconv.FormatInt(kind.Extend(v.Bits), 10))
	default:
		in.out.WriteString(strconv.FormatUint(v.Bits, 10))
	}
	in.out.WriteString("\n")
}

func (in *interpreter) loopFlow(fl flow, id int) (exit bool, result flow) {
	switch fl.signal {
	case flowNone:
		return false, flow{}
	case flowReturn:
		return true, fl
	}
	if fl.target != id {
		return true, fl
	}
	return fl.signal == flowBreak, flow{}
}

func (in *interpreter) rangeLoop(s *Range, f *frame) flow {
	lo := in.eval(s.Lo, f)
	hi := in.eval(s.Hi, f)
	for cur := lo.Bits; less(s.Kind, cur, hi.Bits); cur = s.Kind.Wrap(cur + 1) {
		in.tick()
		f.vars[s.Var] = Value{Bits: cur}
		exit, result := in.loopFlow(in.block(s.Body, f), s.ID)
		if exit {
			return result
		}
	}
	return flow{}
}

func (in *interpreter) whileLoop(s *While, f *frame) flow {
	for counter := 0; counter < s.Bound && (s.Cond == nil || in.eval(s.Cond, f).Bits != 0); {
		in.tick()
		counter++
		exit, result := in.loopFlow(in.block(s.Body, f), s.ID)
		if exit {
			return result
		}
	}
	return flow{}
}

func less(k Kind, a, b uint64) bool {
	if k.Signed() {
		return k.Extend(a) < k.Extend(b)
	}
	return a < b
}

func (in *interpreter) eval(e Expr, f *frame) Value {
	in.tick()
	switch e := e.(type) {
	case *Lit:
		return Value{Bits: e.Kind.Wrap(e.Bits), Float: roundTo(e.Kind, e.Float)}
	case *Var:
		return f.vars[e.Name]
	case *Unary:
		return in.unary(e, f)
	case *Binary:
		return in.binaryExpr(e, f)
	case *Convert:
		return convert(e.X.Type(), e.To, in.eval(e.X, f))
	case *IfExpr:
		if in.eval(e.Cond, f).Bits != 0 {
			return in.eval(e.Then, f)
		}
		return in.eval(e.Else, f)
	case *Call:
		return in.call(e, f)
	case *Math:
		return in.math(e, f)
	}
	panic(fmt.Sprintf("unknown expression %T", e))
}

func (in *interpreter) unary(e *Unary, f *frame) Value {
	x := in.eval(e.X, f)
	k := e.X.Type()
	switch {
	case e.Op == Not:
		return Value{Bits: x.Bits ^ 1}
	case e.Op == Neg && k.IsFloat():
		return Value{Float: -x.Float}
	case e.Op == Neg:
		return Value{Bits: k.Wrap(-x.Bits)}
	}
	return Value{Bits: k.Wrap(^x.Bits)}
}

func (in *interpreter) binaryExpr(e *Binary, f *frame) Value {
	l := in.eval(e.L, f)
	switch e.Op {
	case LogAnd:
		if l.Bits == 0 {
			return l
		}
		return in.eval(e.R, f)
	case LogOr:
		if l.Bits != 0 {
			return l
		}
		return in.eval(e.R, f)
	}
	r := in.eval(e.R, f)
	if e.Op.IsCompare() {
		return compare(e.Op, e.L.Type(), l, r)
	}
	return binary(e.Op, e.L.Type(), e.R.Type(), l, r)
}

func boolValue(b bool) Value {
	if b {
		return Value{Bits: 1}
	}
	return Value{}
}

func compare(op Op, k Kind, l, r Value) Value {
	if k.IsFloat() {
		return boolValue(compareOrdered(op, l.Float, r.Float))
	}
	if k.Signed() {
		return boolValue(compareOrdered(op, k.Extend(l.Bits), k.Extend(r.Bits)))
	}
	return boolValue(compareOrdered(op, l.Bits, r.Bits))
}

func compareOrdered[T int64 | uint64 | float64](op Op, a, b T) bool {
	switch op {
	case Eq:
		return a == b
	case Ne:
		return a != b
	case Lt:
		return a < b
	case Le:
		return a <= b
	case Gt:
		return a > b
	}
	return a >= b
}

func binary(op Op, k, rk Kind, l, r Value) Value {
	switch {
	case k.IsFloat():
		return floatArith(op, k, l.Float, r.Float)
	case op.IsShift():
		return Value{Bits: shift(op, k, rk, l.Bits, r.Bits)}
	}
	return Value{Bits: intArith(op, k, l.Bits, r.Bits)}
}

func roundTo(k Kind, v float64) float64 {
	if k == F32 {
		return float64(float32(v))
	}
	return v
}

func floatArith(op Op, k Kind, a, b float64) Value {
	var v float64
	switch op {
	case Add:
		v = a + b
	case Sub:
		v = a - b
	case Mul:
		v = a * b
	default:
		v = a / b
	}
	return Value{Float: roundTo(k, v)}
}

func intArith(op Op, k Kind, a, b uint64) uint64 {
	switch op {
	case Add:
		return k.Wrap(a + b)
	case Sub:
		return k.Wrap(a - b)
	case Mul:
		return k.Wrap(a * b)
	case And:
		return a & b
	case Or:
		return a | b
	case Xor:
		return a ^ b
	}
	if b == 0 {
		panic(trap{"integer divide by zero"})
	}
	return divide(op, k, a, b)
}

func divide(op Op, k Kind, a, b uint64) uint64 {
	if !k.Signed() {
		if op == Div {
			return a / b
		}
		return a % b
	}
	x, y := k.Extend(a), k.Extend(b)
	if y == -1 {
		if op == Div {
			return k.Wrap(uint64(-x))
		}
		return 0
	}
	if op == Div {
		return k.Wrap(uint64(x / y))
	}
	return k.Wrap(uint64(x % y))
}

func shift(op Op, k, ck Kind, a, count uint64) uint64 {
	n := ck.Wrap(count)
	huge := n >= 64 || (ck.Signed() && ck.Extend(count) < 0)
	if op == Shl {
		if huge || int(n) >= k.Bits() {
			return 0
		}
		return k.Wrap(a << n)
	}
	if k.Signed() {
		if huge {
			n = 63
		}
		return k.Wrap(uint64(k.Extend(a) >> n))
	}
	if huge || int(n) >= k.Bits() {
		return 0
	}
	return a >> n
}

func convert(from, to Kind, v Value) Value {
	switch {
	case from.IsInt() && to.IsInt():
		return Value{Bits: to.Wrap(uint64(from.Extend(v.Bits)))}
	case from.IsInt():
		return Value{Float: intToFloat(from, to, v.Bits)}
	case to.IsInt():
		return Value{Bits: floatToInt(to, v.Float)}
	}
	return Value{Float: roundTo(to, v.Float)}
}

func intToFloat(from, to Kind, bits uint64) float64 {
	if from.Signed() {
		if to == F32 {
			return float64(float32(from.Extend(bits)))
		}
		return float64(from.Extend(bits))
	}
	if to == F32 {
		return float64(float32(bits))
	}
	return float64(bits)
}

func floatToInt(to Kind, v float64) uint64 {
	t := math.Trunc(v)
	bits := float64(to.Bits())
	lo, hi := 0.0, math.Exp2(bits)
	if to.Signed() {
		lo, hi = -math.Exp2(bits-1), math.Exp2(bits-1)
	}
	if math.IsNaN(t) || t < lo || t >= hi {
		panic(abort{ErrUndefined})
	}
	if to.Signed() {
		return to.Wrap(uint64(int64(t)))
	}
	return uint64(t)
}

func (in *interpreter) call(e *Call, f *frame) Value {
	callee := in.funcs[e.Callee]
	callFrame := &frame{vars: make(map[string]Value, len(callee.Params))}
	for i, p := range callee.Params {
		callFrame.vars[p.Name] = in.eval(e.Args[i], f)
	}
	fl := in.block(callee.Body, callFrame)
	if fl.signal == flowReturn {
		return fl.value
	}
	return in.eval(callee.Tail, callFrame)
}

func (in *interpreter) math(e *Math, f *frame) Value {
	x := in.eval(e.X, f)
	if e.Op == Floor {
		return Value{Float: math.Floor(x.Float)}
	}
	return Value{Float: roundTo(e.X.Type(), math.Sqrt(x.Float))}
}
