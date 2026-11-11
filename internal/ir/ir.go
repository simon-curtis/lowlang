package ir

import (
	"fmt"
	"strings"

	"lowlang/internal/target"
	"lowlang/internal/types"
)

type Op int

const (
	OpConst Op = iota
	OpString
	OpEmbed
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
	OpCheck
	OpSlot
	OpCall
	OpCCall
	OpRuntime
	OpResult
	OpFuncAddr
	OpMulHigh
	OpVector
	OpVectorScalar
	OpTrailingZeros
	OpHashSeed
	OpCell
)

const (
	CodeAddr = iota
	ThunkAddr
	SymbolAddr
)

var opNames = [...]string{
	OpConst: "const", OpString: "string", OpEmbed: "embed", OpParam: "param", OpUndef: "undef", OpPhi: "phi",
	OpAdd: "add", OpSub: "sub", OpMul: "mul", OpDiv: "div", OpRem: "rem", OpNeg: "neg", OpNot: "not",
	OpAnd: "and", OpOr: "or", OpXor: "xor", OpShl: "shl", OpShr: "shr", OpCompl: "compl",
	OpConvert: "convert", OpSqrt: "sqrt", OpFloor: "floor",
	OpEq: "eq", OpNe: "ne", OpLt: "lt", OpLe: "le", OpGt: "gt", OpGe: "ge",
	OpLoad: "load", OpStore: "store", OpCheck: "check", OpSlot: "slot", OpCall: "call", OpCCall: "ccall", OpRuntime: "runtime", OpResult: "result",
	OpFuncAddr: "funcaddr", OpMulHigh: "mulhigh", OpVector: "vector", OpVectorScalar: "vectorscalar", OpTrailingZeros: "ctz", OpHashSeed: "hashseed", OpCell: "cell",
}

func (o Op) String() string {
	return opNames[o]
}

func (o Op) Compare() bool {
	return o >= OpEq && o <= OpGe
}

func (o Op) HasResult() bool {
	return o != OpStore && o != OpCall && o != OpCCall && o != OpRuntime && o != OpCheck && o != OpVector
}

type Value struct {
	ID        int
	Op        Op
	Args      []*Value
	Block     *Block
	Aux       int64
	Str       string
	Target    *Func
	Plan      *CPlan
	Type      *types.Type
	From      *types.Type
	Scalar    bool
	FloatBits int
	Indirect  bool
	Uses      int
	forward   *Value
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
	Switch
)

type JumpTable struct {
	Low   int64
	Cases []int
}

func (t *JumpTable) Index(control int64) int {
	offset := uint64(control) - uint64(t.Low)
	if offset >= uint64(len(t.Cases)) {
		return 0
	}
	return t.Cases[offset]
}

type Block struct {
	ID      int
	Kind    Kind
	Values  []*Value
	Control *Value
	Table   *JumpTable
	Results []*Value
	Succs   []*Block
	Preds   []*Block
	Poll    bool
	merged  bool
}

func (b *Block) String() string {
	return fmt.Sprintf("b%d", b.ID)
}

type Func struct {
	Name        string
	ParamWords  int
	ResultWords int
	Ret         *types.Type
	Callback    bool
	CParams     []*types.Type
	Blocks      []*Block
	Slots       []int
	values      int
	blockIDs    int
}

type Options struct {
	Inline bool
	Hoist  bool
}

type Program struct {
	OS        target.OS
	Libraries []string
	CSymbols  []string
	Imports   map[string][]string
	Funcs     []*Func
	Main      *Func
	MainErr   *types.Type
	Cells     int
}

func (f *Func) newBlock() *Block {
	b := &Block{ID: f.blockIDs}
	f.blockIDs++
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
	case Switch:
		fmt.Fprintf(s, "  switch %s low %d table %v default %s targets %v\n", b.Control, b.Table.Low, b.Table.Cases, b.Succs[0], b.Succs[1:])
	case Ret:
		fmt.Fprintf(s, "  ret %v\n", b.Results)
	case Unreachable:
		s.WriteString("  unreachable\n")
	}
}

func describe(v *Value) string {
	var detail string
	switch v.Op {
	case OpConst, OpParam, OpSlot, OpResult, OpLoad, OpStore, OpHashSeed, OpCell:
		detail = fmt.Sprintf(" #%d", v.Aux)
	case OpString, OpEmbed:
		detail = fmt.Sprintf(" %q", v.Str)
	case OpRuntime:
		detail = fmt.Sprintf(" %s", v.Str)
	case OpCall:
		detail = fmt.Sprintf(" %s", v.Target.Name)
	case OpFuncAddr:
		detail = fmt.Sprintf(" %s", v.Str)
		if v.Target != nil {
			detail = fmt.Sprintf(" %s", v.Target.Name)
		}
	case OpCCall:
		detail = fmt.Sprintf(" %s", v.Plan.Symbol)
	case OpConvert:
		detail = fmt.Sprintf(" %s->%s", v.From, v.Type)
	case OpVector, OpVectorScalar:
		detail = fmt.Sprintf(" %s", VectorOp(v.Aux))
	}
	if v.Type != nil && v.Op != OpConvert {
		detail = fmt.Sprintf("%s:%s", detail, v.Type)
	}
	return fmt.Sprintf("%s = %s%s %v", v, v.Op, detail, v.Args)
}
