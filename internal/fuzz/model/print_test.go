package model

import (
	"math"
	"strings"
	"testing"
)

func printed(e Expr) string {
	var pr printer
	pr.expr(e, noKind)
	return pr.out.String()
}

func TestPrintLiterals(t *testing.T) {
	tests := []struct {
		name string
		expr Expr
		want string
	}{
		{"i64 default", lit(I64, 5), "5"},
		{"i64 negative", lit(I64, math.MaxUint64), "(-1)"},
		{"i64 minimum", lit(I64, 1<<63), "(-9223372036854775807 - 1)"},
		{"u8 wrapped", lit(U8, 5), "u8(5)"},
		{"i32 minimum wrapped", lit(I32, 1<<31), "i32((-2147483648))"},
		{"u64 high wrapped", lit(U64, math.MaxUint64), "(~u64(0))"},
		{"f64 default", &Lit{Kind: F64, Float: 1.5}, "1.5"},
		{"f64 whole", &Lit{Kind: F64, Float: 2}, "2.0"},
		{"f64 negative zero", &Lit{Kind: F64, Float: math.Copysign(0, -1)}, "(-0.0)"},
		{"f32 wrapped", &Lit{Kind: F32, Float: 0.5}, "f32(0.5)"},
		{"bool", lit(Bool, 1), "true"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := printed(tt.expr); got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestPrintSiblingTyping(t *testing.T) {
	x := &Var{Name: "x", Kind: U8}
	tests := []struct {
		name string
		expr Expr
		want string
	}{
		{"literal beside variable", &Binary{Op: Add, L: x, R: lit(U8, 3)}, "(x + 3)"},
		{"literal left of variable", &Binary{Op: Sub, L: lit(U8, 3), R: x}, "(3 - x)"},
		{"two literals wrap", &Binary{Op: Add, L: lit(U8, 1), R: lit(U8, 2)}, "(u8(1) + u8(2))"},
		{"nested literals wrap beside variable", &Binary{Op: Add, L: &Binary{Op: Add, L: lit(U8, 1), R: lit(U8, 2)}, R: x}, "((u8(1) + u8(2)) + x)"},
		{"shift count never inferred", &Binary{Op: Shl, L: x, R: lit(U16, 3)}, "(x << u16(3))"},
		{"unary wraps literal", &Unary{Op: Neg, X: lit(U8, 1)}, "(-u8(1))"},
		{"if branches wrap", &IfExpr{Cond: lit(Bool, 1), Then: lit(U8, 1), Else: x}, "(if true { u8(1) } else { x })"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := printed(tt.expr); got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestPrintLabels(t *testing.T) {
	inner := &While{ID: 2, Counter: "w_2", Bound: 3, Body: []Stmt{
		&IfStmt{Cond: lit(Bool, 1), Then: []Stmt{&Jump{Target: 1, Continue: true}}},
		&IfStmt{Cond: lit(Bool, 1), Then: []Stmt{&Jump{Target: 2}}},
	}}
	outer := &Range{ID: 1, Var: "i_1", Kind: I64, Lo: lit(I64, 0), Hi: lit(I64, 2), Body: []Stmt{inner}}
	p := &Program{
		Funcs: []*Func{{Name: "f", Result: I64, Body: []Stmt{outer}, Tail: lit(I64, 0)}},
	}
	text := p.String()
	for _, want := range []string{"L1: for i_1 in (0)..(2) {", "continue L1", "for w_2 < 3 {"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	if strings.Contains(text, "L2") {
		t.Errorf("innermost loop labelled in:\n%s", text)
	}
}
