package ir

import (
	"fmt"
	"strings"

	"lowlang/internal/types"
)

type Op int

const (
	OpConst Op = iota
	OpString
	OpParam
	OpUndef
	OpPhi
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpRem
	OpNeg
	OpNot
	OpAnd
	OpOr
	OpXor
	OpShl
	OpShr
	OpCompl
	OpConvert
	OpSqrt
	OpFloor
	OpEq
	OpNe
	OpLt
	OpLe
	OpGt
	OpGe
	OpLoad
	OpStore
	OpSlot
	OpCall
	OpRuntime
	OpResult
)

var opNames = [...]string{
	OpConst: "const", OpString: "string", OpParam: "param", OpUndef: "undef", OpPhi: "phi",
	OpAdd: "add", OpSub: "sub", OpMul: "mul", OpDiv: "div", OpRem: "rem", OpNeg: "neg", OpNot: "not",
	OpAnd: "and", OpOr: "or", OpXor: "xor", OpShl: "shl", OpShr: "shr", OpCompl: "compl",
	OpConvert: "convert", OpSqrt: "sqrt", OpFloor: "floor",
	OpEq: "eq", OpNe: "ne", OpLt: "lt", OpLe: "le", OpGt: "gt", OpGe: "ge",
	OpLoad: "load", OpStore: "store", OpSlot: "slot", OpCall: "call", OpRuntime: "runtime", OpResult: "result",
}

func (o Op) String() string {
	return opNames[o]
}

func (o Op) Compare() bool {
	return o >= OpEq && o <= OpGe
}

func (o Op) HasResult() bool {
	return o != OpStore && o != OpCall && o != OpRuntime
}

type Value struct {
	ID      int
	Op      Op
	Args    []*Value
	Block   *Block
	Aux     int64
	Str     string
	Target  *Func
	Type    *types.Type
	From    *types.Type
	Uses    int
	forward *Value
}

func (v *Value) String() string {
	return fmt.Sprintf("v%d", v.ID)
}

type Kind int

const (
	Plain Kind = iota
	If
	Ret
	Unreachable
)

type Block struct {
	ID      int
	Kind    Kind
	Values  []*Value
	Control *Value
	Results []*Value
	Succs   []*Block
	Preds   []*Block
}

func (b *Block) String() string {
	return fmt.Sprintf("b%d", b.ID)
}

type Func struct {
	Name        string
	ParamWords  int
	ResultWords int
	Blocks      []*Block
	Slots       []int
	values      int
}

type Program struct {
	Funcs   []*Func
	Main    *Func
	MainErr *types.Type
}

func (f *Func) newBlock() *Block {
	b := &Block{ID: len(f.Blocks)}
	f.Blocks = append(f.Blocks, b)
	return b
}

func (f *Func) newValue(b *Block, op Op, args ...*Value) *Value {
	f.values++
	return &Value{ID: f.values, Op: op, Args: args, Block: b}
}

func (f *Func) String() string {
	var s strings.Builder
	fmt.Fprintf(&s, "fn %s params=%d results=%d slots=%v\n", f.Name, f.ParamWords, f.ResultWords, f.Slots)
	for _, b := range f.Blocks {
		f.writeBlock(&s, b)
	}
	return s.String()
}

func (f *Func) writeBlock(s *strings.Builder, b *Block) {
	fmt.Fprintf(s, "%s: preds=%v\n", b, b.Preds)
	for _, v := range b.Values {
		fmt.Fprintf(s, "  %s\n", describe(v))
	}
	switch b.Kind {
	case Plain:
		fmt.Fprintf(s, "  jump %s\n", b.Succs[0])
	case If:
		fmt.Fprintf(s, "  if %s then %s else %s\n", b.Control, b.Succs[0], b.Succs[1])
	case Ret:
		fmt.Fprintf(s, "  ret %v\n", b.Results)
	case Unreachable:
		s.WriteString("  unreachable\n")
	}
}

func describe(v *Value) string {
	var detail string
	switch v.Op {
	case OpConst, OpParam, OpSlot, OpResult, OpLoad, OpStore:
		detail = fmt.Sprintf(" #%d", v.Aux)
	case OpString:
		detail = fmt.Sprintf(" %q", v.Str)
	case OpRuntime:
		detail = fmt.Sprintf(" %s", v.Str)
	case OpCall:
		detail = fmt.Sprintf(" %s", v.Target.Name)
	case OpConvert:
		detail = fmt.Sprintf(" %s->%s", v.From, v.Type)
	}
	if v.Type != nil && v.Op != OpConvert {
		detail = fmt.Sprintf("%s:%s", detail, v.Type)
	}
	return fmt.Sprintf("%s = %s%s %v", v, v.Op, detail, v.Args)
}
