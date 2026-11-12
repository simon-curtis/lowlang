package ir

import (
	"lowlang/internal/ast"
	"lowlang/internal/types"
)

type CSig struct {
	Symbol string
	Args   []*types.Type
	ByRef  []bool
	Ret    *types.Type
}

func (b *builder) cCall(e *ast.Call) []*Value {
	return b.cInvoke(e.Target.Symbol, e.Target.ParamTypes, e.Target.RetType, nil, e.Args)
}

func (b *builder) cInvoke(symbol string, params []*types.Type, ret *types.Type, callee []*Value, exprs []ast.Expr) []*Value {
	var args []*Value
	var argTypes []*types.Type
	var mut []bool
	var refs []*ast.Ref
	for i, arg := range exprs {
		if r, ok := arg.(*ast.Ref); ok {
			args = append(args, b.address(b.placeOf(r.X)))
			argTypes = append(argTypes, r.Type)
			mut = append(mut, true)
			refs = append(refs, r)
			continue
		}
		t := arg.TypeOf()
		if i < len(params) {
			t = params[i]
		}
		words := b.value(arg, t)
		if t == types.F32 && i >= len(params) {
			words = []*Value{b.convert(words[0], types.F32, types.F64)}
			t = types.F64
		}
		args = append(args, words...)
		argTypes = append(argTypes, t)
		mut = append(mut, false)
	}
	operands := append(callee, args...)
	call := b.emit(OpCCall, operands...)
	call.Sig = &CSig{Symbol: symbol, Args: argTypes, ByRef: mut, Ret: ret}
	results := make([]*Value, ret.Size())
	for k := range results {
		results[k] = b.result(call, k)
	}
	if width := ret.Width(); width != nil && !width.Float() && len(results) == 1 {
		results[0] = b.convert(results[0], types.I64, width)
	}
	classify(results, ret)
	for _, r := range refs {
		b.renormalize(r)
	}
	return results
}

func (b *builder) renormalize(r *ast.Ref) {
	width := r.Type.Width()
	if width == nil {
		return
	}
	if mustLocate(r.X).width != nil {
		return
	}
	p := b.placeOf(r.X)
	p.scalar = width
	value := b.load(p, r.Type)
	p.scalar = nil
	b.store(p, value)
}

func (b *builder) cStruct(e *ast.StructLit) []*Value {
	slot := b.slot(e.Type.Size())
	zero := b.constant(0)
	for k := range e.Type.Size() {
		b.store(place{addr: slot, offset: 8 * k, mem: MemRaw}, []*Value{zero})
	}
	for _, init := range e.Fields {
		at := place{addr: slot, offset: init.Field.Bytes, scalar: init.Field.Type.Width(), tail: init.Field.Type.Tail(), mem: MemRaw}
		b.store(at, b.value(init.Value, init.Field.Type))
	}
	return b.load(place{addr: slot, mem: MemRaw}, e.Type)
}

func (b *builder) cField(e *ast.Selector) []*Value {
	words := b.value(e.X, e.X.TypeOf())
	slot := b.slot(len(words))
	b.store(place{addr: slot, mem: MemRaw}, words)
	return b.load(place{addr: slot, offset: e.Field.Bytes, scalar: e.Field.Type.Width(), tail: e.Field.Type.Tail(), mem: MemRaw}, e.Type)
}

func (b *builder) slot(words int) *Value {
	slot := b.emit(OpSlot)
	slot.Aux = int64(len(b.fn.Slots))
	b.fn.Slots = append(b.fn.Slots, words)
	return slot
}
