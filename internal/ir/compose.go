package ir

import (
	"slices"

	"lowlang/internal/ast"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

type segment struct {
	expr   ast.Expr
	render *ast.Func
	spec   *ast.Spec
}

type piece struct {
	text   string
	words  []*Value
	typ    *types.Type
	render *ast.Func
	spec   *ast.Spec
}

func (p piece) formatted() bool {
	return p.words != nil && (p.typ != types.Str || p.spec != nil)
}

func (p piece) unbounded() bool {
	return p.render != nil || p.spec != nil && p.spec.Format != nil
}

func (p piece) padding() int64 {
	if p.spec == nil {
		return 0
	}
	return int64(max(p.spec.Width, -p.spec.Width))
}

func renderedSize(t *types.Type) int64 {
	switch {
	case t == types.Bool:
		return 5
	case t.Float():
		return 25
	case t.Integer():
		return 20
	}
	return 32
}

func flatten(e ast.Expr, render *ast.Func, spec *ast.Spec, out []segment) []segment {
	if spec != nil {
		return append(out, segment{expr: e, render: render, spec: spec})
	}
	switch e := e.(type) {
	case *ast.Interp:
		for i, part := range e.Parts {
			out = flatten(part, e.Renders[i], e.Specs[i], out)
		}
		return out
	case *ast.Binary:
		if e.Op == lexer.Plus && e.X.TypeOf() == types.Str {
			left := flatten(e.X, nil, nil, out)
			return flatten(e.Y, nil, nil, left)
		}
	}
	return append(out, segment{expr: e, render: render})
}

func appendText(pieces []piece, text string) []piece {
	if text == "" {
		return pieces
	}
	if n := len(pieces); n > 0 && pieces[n-1].words == nil {
		pieces[n-1].text += text
		return pieces
	}
	return append(pieces, piece{text: text})
}

func (b *builder) pieces(parts []segment) []piece {
	var out []piece
	for _, part := range parts {
		if lit, ok := part.expr.(*ast.StrLit); ok && part.spec == nil {
			out = appendText(out, lit.Value)
			continue
		}
		t := part.expr.TypeOf()
		words := b.value(part.expr, t)
		out = append(out, piece{words: words, typ: t, render: part.render, spec: part.spec})
	}
	return out
}

func (b *builder) composedSize(pieces []piece) *Value {
	var fixed int64
	var lengths []*Value
	for _, p := range pieces {
		switch {
		case p.words == nil:
			fixed += int64(len(p.text))
		case p.typ == types.Str:
			lengths = append(lengths, p.words[1])
			fixed += p.padding()
		default:
			fixed += renderedSize(p.typ) + p.padding()
		}
	}
	total := b.constant(fixed)
	for _, n := range lengths {
		total = b.count(OpAdd, total, n)
	}
	return total
}

func (b *builder) interpolate(e *ast.Interp) []*Value {
	return b.compose(flatten(e, nil, nil, nil))
}

func (b *builder) concatenate(e *ast.Binary) []*Value {
	return b.compose(flatten(e, nil, nil, nil))
}

func (b *builder) compose(parts []segment) []*Value {
	pieces := b.pieces(parts)
	total := b.composedSize(pieces)
	zero := b.constant(0)
	data := b.runtime(rtGrow, zero, zero, total, zero)
	if slices.ContainsFunc(pieces, piece.formatted) {
		return b.composeFormatted(pieces, data, total)
	}
	position := zero
	for _, p := range pieces {
		src, n := b.source(p)
		b.runtime(rtMemmove, b.emit(OpAdd, data, position), src, n)
		position = b.count(OpAdd, position, n)
	}
	return []*Value{data, position}
}

func (b *builder) source(p piece) (*Value, *Value) {
	if p.words != nil {
		return p.words[0], p.words[1]
	}
	text := b.emit(OpString)
	text.Str = p.text
	text.Scalar = true
	return text, b.constant(int64(len(p.text)))
}

func (b *builder) composeFormatted(pieces []piece, data, total *Value) []*Value {
	slot := b.slot(3)
	b.store(place{addr: slot}, []*Value{data, b.constant(0), total})
	checked := slices.ContainsFunc(pieces, piece.unbounded)
	for _, p := range pieces {
		if p.formatted() {
			b.appendFormatted(slot, p)
			continue
		}
		src, n := b.source(p)
		if checked {
			b.callNamed("__append_str", slot, src, n)
			continue
		}
		buf := b.load(place{addr: slot}, types.Str)
		b.runtime(rtMemmove, b.emit(OpAdd, buf[0], buf[1]), src, n)
		b.store(place{addr: slot}.at(1), []*Value{b.count(OpAdd, buf[1], n)})
	}
	return b.load(place{addr: slot}, types.Str)
}

func (b *builder) appendFormatted(slot *Value, p piece) {
	if p.spec == nil || p.spec.Width == 0 {
		b.appendValue(slot, p)
		return
	}
	buffer := b.load(place{addr: slot}, types.Str)
	b.appendValue(slot, p)
	b.callNamed("__align", slot, buffer[1], b.constant(int64(p.spec.Width)))
}

func (b *builder) appendValue(slot *Value, p piece) {
	switch {
	case p.spec != nil && p.spec.Format != nil:
		b.appendNumber(slot, p)
	case p.typ == types.Str:
		b.callNamed("__append_str", slot, p.words[0], p.words[1])
	case p.render != nil:
		b.renderInto(p.render, slot, p.words)
	default:
		b.callNamed(appender(p.typ), slot, p.words[0])
	}
}

func (b *builder) flag(on bool) *Value {
	if on {
		return b.constant(1)
	}
	return b.constant(0)
}

func (b *builder) appendNumber(slot *Value, p piece) {
	f := p.spec.Format
	word := p.words[0]
	switch f.Kind {
	case ast.DecimalFormat:
		b.callNamed("__append_decimal", slot, word, b.flag(!p.typ.Signed), b.constant(int64(f.Precision)))
	case ast.HexFormat, ast.UpperHexFormat, ast.BinaryFormat:
		shift := int64(4)
		if f.Kind == ast.BinaryFormat {
			shift = 1
		}
		upper := f.Kind == ast.UpperHexFormat
		b.callNamed("__append_radix", slot, word, b.constant(int64(p.typ.Bits)), b.constant(shift), b.flag(upper), b.constant(int64(f.Precision)))
	default:
		b.appendLayout(slot, p)
	}
}

type decimalLayout struct {
	scientific bool
	intMin     int
	fracMin    int
	fracMax    int
	group      bool
}

func layoutOf(f *ast.Format, t *types.Type) decimalLayout {
	if f.Kind == ast.PictureFormat {
		return decimalLayout{intMin: f.MinInt, fracMin: f.MinFrac, fracMax: f.MaxFrac, group: f.Group}
	}
	precision := f.Precision
	if !f.Explicit {
		precision = defaultPrecision(f.Kind, t)
	}
	return decimalLayout{scientific: f.Kind == ast.ScientificFormat, intMin: 1, fracMin: precision, fracMax: precision, group: f.Kind == ast.GroupedFormat}
}

func defaultPrecision(kind ast.FormatKind, t *types.Type) int {
	switch {
	case kind == ast.ScientificFormat:
		return 6
	case kind == ast.GroupedFormat && t.Integer():
		return 0
	}
	return 2
}

func (b *builder) appendLayout(slot *Value, p piece) {
	l := layoutOf(p.spec.Format, p.typ)
	args := []*Value{slot, p.words[0]}
	name := "__append_float_layout"
	switch {
	case p.typ == types.F32:
		args[1] = b.convert(p.words[0], types.F32, types.F64)
	case p.typ.Integer():
		name = "__append_int_layout"
		args = append(args, b.flag(!p.typ.Signed))
	}
	args = append(args, b.flag(l.scientific), b.constant(int64(l.intMin)), b.constant(int64(l.fracMin)), b.constant(int64(l.fracMax)), b.flag(l.group))
	b.callNamed(name, args...)
}

func (b *builder) renderInto(render *ast.Func, buffer *Value, words []*Value) {
	zero := b.constant(0)
	path := b.slot(3)
	b.store(place{addr: path}, []*Value{zero, zero, zero})
	args := append([]*Value{buffer, path}, words...)
	b.invoke(b.funcs[render], args)
}

func (b *builder) renderString(render *ast.Func, words []*Value) []*Value {
	zero := b.constant(0)
	buffer := b.slot(3)
	b.store(place{addr: buffer}, []*Value{zero, zero, zero})
	b.renderInto(render, buffer, words)
	return b.load(place{addr: buffer}, types.Str)
}

func appender(t *types.Type) string {
	switch t {
	case types.Bool:
		return "__append_bool"
	case types.F64:
		return "__append_f64"
	case types.F32:
		return "__append_f32"
	case types.U64:
		return "__append_u64"
	}
	return "__append_i64"
}
