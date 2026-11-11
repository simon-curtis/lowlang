package ir

import (
	"slices"

	"lowlang/internal/ast"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

type piece struct {
	text  string
	words []*Value
	typ   *types.Type
}

func (p piece) formatted() bool {
	return p.words != nil && p.typ != types.Str
}

func renderedSize(t *types.Type) int64 {
	switch {
	case t == types.Bool:
		return 5
	case t.Float():
		return 25
	}
	return 20
}

func flatten(e ast.Expr, out []ast.Expr) []ast.Expr {
	switch e := e.(type) {
	case *ast.Interp:
		for _, part := range e.Parts {
			out = flatten(part, out)
		}
		return out
	case *ast.Binary:
		if e.Op == lexer.Plus && e.X.TypeOf() == types.Str {
			return flatten(e.Y, flatten(e.X, out))
		}
	}
	return append(out, e)
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

func (b *builder) pieces(parts []ast.Expr) []piece {
	var out []piece
	for _, part := range parts {
		if lit, ok := part.(*ast.StrLit); ok {
			out = appendText(out, lit.Value)
			continue
		}
		t := part.TypeOf()
		out = append(out, piece{words: b.value(part, t), typ: t})
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
		default:
			fixed += renderedSize(p.typ)
		}
	}
	total := b.constant(fixed)
	for _, n := range lengths {
		total = b.count(OpAdd, total, n)
	}
	return total
}

func (b *builder) interpolate(e *ast.Interp) []*Value {
	return b.compose(flatten(e, nil))
}

func (b *builder) concatenate(e *ast.Binary) []*Value {
	return b.compose(flatten(e, nil))
}

func (b *builder) compose(parts []ast.Expr) []*Value {
	pieces := b.pieces(parts)
	total := b.composedSize(pieces)
	zero := b.constant(0)
	data := b.runtime("grow", zero, zero, total, zero)
	if slices.ContainsFunc(pieces, piece.formatted) {
		return b.composeFormatted(pieces, data, total)
	}
	position := zero
	for _, p := range pieces {
		src, n := b.source(p)
		b.runtimeVoid("memmove", b.emit(OpAdd, data, position), src, n)
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
	for _, p := range pieces {
		if p.formatted() {
			b.callNamed(appender(p.typ), slot, p.words[0])
			continue
		}
		src, n := b.source(p)
		buf := b.load(place{addr: slot}, types.Str)
		b.runtimeVoid("memmove", b.emit(OpAdd, buf[0], buf[1]), src, n)
		b.store(place{addr: slot}.at(1), []*Value{b.count(OpAdd, buf[1], n)})
	}
	return b.load(place{addr: slot}, types.Str)
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
