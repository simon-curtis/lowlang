package ir

import (
	"lowlang/internal/ast"
	"lowlang/internal/types"
)

type Class int

const (
	Integer Class = iota
	Vector
	Stack
)

type CPlan struct {
	Symbol     string
	Args       []Class
	Results    []Class
	Memory     bool
	Vectors    int
	StackWords int
	Ret        *types.Type
}

func isC(t *types.Type) bool {
	return t.Kind == types.StructKind && t.C
}

func eightbytes(t *types.Type) []Class {
	if !isC(t) {
		if t.Float() {
			return []Class{Vector}
		}
		return []Class{Integer}
	}
	classes := make([]Class, t.Size())
	for k := range classes {
		classes[k] = Vector
	}
	var visit func(s *types.Type, base int)
	visit = func(s *types.Type, base int) {
		for _, f := range s.Fields {
			at := base + f.Bytes
			if isC(f.Type) {
				visit(f.Type, at)
				continue
			}
			if !f.Type.Float() {
				classes[at/8] = Integer
			}
		}
	}
	visit(t, 0)
	return classes
}

type registers struct {
	ints    int
	vectors int
}

func (r *registers) fit(classes []Class) bool {
	ints, vectors := r.ints, r.vectors
	for _, c := range classes {
		if c == Integer {
			ints++
		} else {
			vectors++
		}
	}
	if ints > 6 || vectors > 8 {
		return false
	}
	r.ints, r.vectors = ints, vectors
	return true
}

func (b *builder) cPlan(fn *ast.Func, argTypes []*types.Type, mut []bool) *CPlan {
	plan := &CPlan{Symbol: fn.Name, Ret: fn.RetType}
	regs := &registers{}
	if fn.RetType != types.Void {
		if isC(fn.RetType) && fn.RetType.Bytes > 16 {
			plan.Memory = true
			regs.ints = 1
		} else {
			plan.Results = eightbytes(fn.RetType)
		}
	}
	for i, t := range argTypes {
		classes := []Class{Integer}
		if !mut[i] {
			classes = eightbytes(t)
		}
		if (isC(t) && !mut[i] && t.Bytes > 16) || !regs.fit(classes) {
			for range classes {
				plan.Args = append(plan.Args, Stack)
			}
			plan.StackWords += len(classes)
			continue
		}
		plan.Args = append(plan.Args, classes...)
	}
	plan.Vectors = regs.vectors
	return plan
}

func (b *builder) cCall(e *ast.Call) []*Value {
	fn := e.Target
	var args []*Value
	var argTypes []*types.Type
	var mut []bool
	var refs []*ast.Ref
	for i, arg := range e.Args {
		if r, ok := arg.(*ast.Ref); ok {
			args = append(args, b.address(b.place(r.Place)))
			argTypes = append(argTypes, r.Type)
			mut = append(mut, true)
			refs = append(refs, r)
			continue
		}
		t := arg.TypeOf()
		if i < len(fn.ParamTypes) {
			t = fn.ParamTypes[i]
		}
		words := b.value(arg, t)
		if t == types.F32 && i >= len(fn.ParamTypes) {
			words = []*Value{b.convert(words[0], types.F32, types.F64)}
			t = types.F64
		}
		args = append(args, words...)
		argTypes = append(argTypes, t)
		mut = append(mut, false)
	}
	call := b.emit(OpCCall, args...)
	call.Plan = b.cPlan(fn, argTypes, mut)
	results := make([]*Value, fn.RetType.Size())
	for k := range results {
		results[k] = b.result(call, k)
	}
	if width := fn.RetType.Width(); width != nil && len(results) == 1 {
		if width.Float() {
			width = types.U32
		}
		results[0] = b.convert(results[0], types.I64, width)
	}
	classify(results, fn.RetType)
	for _, r := range refs {
		b.renormalize(r)
	}
	return results
}

func (b *builder) renormalize(r *ast.Ref) {
	width := r.Type.Width()
	if width == nil || r.Place.Width != nil {
		return
	}
	p := b.place(r.Place)
	p.scalar = width
	value := b.load(p, r.Type)
	p.scalar = nil
	b.store(p, value)
}

func (b *builder) cStruct(e *ast.StructLit) []*Value {
	slot := b.slot(e.Type.Size())
	zero := b.constant(0)
	for k := range e.Type.Size() {
		b.store(place{addr: slot, offset: 8 * k}, []*Value{zero})
	}
	for _, init := range e.Fields {
		at := place{addr: slot, offset: init.Field.Bytes, scalar: init.Field.Type.Width()}
		b.store(at, b.value(init.Value, init.Field.Type))
	}
	return b.load(place{addr: slot}, e.Type)
}

func (b *builder) cField(e *ast.Selector) []*Value {
	words := b.value(e.X, e.X.TypeOf())
	slot := b.slot(len(words))
	b.store(place{addr: slot}, words)
	return b.load(place{addr: slot, offset: e.Field.Bytes, scalar: e.Field.Type.Width()}, e.Type)
}

func (b *builder) slot(words int) *Value {
	slot := b.emit(OpSlot)
	slot.Aux = int64(len(b.fn.Slots))
	b.fn.Slots = append(b.fn.Slots, words)
	return slot
}

func (b *builder) pointerBuiltin(e *ast.Call) ([]*Value, bool) {
	arg := e.Args[0]
	switch e.Builtin {
	case "view":
		n := b.word(e.Args[1])
		return []*Value{b.word(arg), n, n}, true
	case "load":
		elem := e.Type
		return b.load(place{addr: b.word(arg), scalar: elem.Width()}, elem), true
	case "store":
		elem := e.Args[1].TypeOf()
		ptr := b.word(arg)
		b.store(place{addr: ptr, scalar: elem.Width()}, b.value(e.Args[1], elem))
		return nil, true
	case "cstr":
		return b.callNamed("__cstr", b.value(arg, types.Str)...), true
	case "from_cstr":
		return b.callNamed("__from_cstr", b.word(arg)), true
	case "pin":
		w := b.value(arg, arg.TypeOf())[0]
		b.runtimeVoid("pin", w)
		return []*Value{w}, true
	case "unpin":
		b.runtimeVoid("unpin", b.word(arg))
		return nil, true
	case "__slice_ptr":
		return b.value(arg, arg.TypeOf())[:1], true
	}
	return nil, false
}
