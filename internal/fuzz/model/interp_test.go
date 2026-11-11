package model

import (
	"errors"
	"math"
	"testing"
)

func lit(kind Kind, bits uint64) *Lit {
	return &Lit{Kind: kind, Bits: bits}
}

func mainPrinting(exprs ...Expr) *Program {
	p := &Program{}
	for _, e := range exprs {
		p.Main = append(p.Main, &Print{Value: e})
	}
	return p
}

func TestRunArithmetic(t *testing.T) {
	minI64 := uint64(math.MaxInt64) + 1
	minI32 := uint64(1) << 31
	tests := []struct {
		name string
		expr Expr
		want string
	}{
		{"u8 add wraps", &Binary{Op: Add, L: lit(U8, 250), R: lit(U8, 10)}, "4\n"},
		{"u8 sub wraps", &Binary{Op: Sub, L: lit(U8, 1), R: lit(U8, 2)}, "255\n"},
		{"i64 min div -1", &Binary{Op: Div, L: lit(I64, minI64), R: lit(I64, math.MaxUint64)}, "-9223372036854775808\n"},
		{"i64 min rem -1", &Binary{Op: Rem, L: lit(I64, minI64), R: lit(I64, math.MaxUint64)}, "0\n"},
		{"i32 min div -1", &Binary{Op: Div, L: lit(I32, minI32), R: lit(I32, 0xFFFFFFFF)}, "-2147483648\n"},
		{"i64 rem sign follows dividend", &Binary{Op: Rem, L: lit(I64, uint64(math.MaxUint64-6)), R: lit(I64, 4)}, "-3\n"},
		{"i32 shl by width", &Binary{Op: Shl, L: lit(I32, 1), R: lit(I64, 32)}, "0\n"},
		{"i32 shl below width", &Binary{Op: Shl, L: lit(I32, 1), R: lit(U8, 31)}, "-2147483648\n"},
		{"i32 shr sign fill", &Binary{Op: Shr, L: lit(I32, 0xFFFFFFF8), R: lit(U8, 40)}, "-1\n"},
		{"i32 shr negative count", &Binary{Op: Shr, L: lit(I32, 0xFFFFFFF8), R: lit(I64, math.MaxUint64)}, "-1\n"},
		{"u32 shr huge count", &Binary{Op: Shr, L: lit(U32, 8), R: lit(U64, math.MaxUint64)}, "0\n"},
		{"u8 shl truncates", &Binary{Op: Shl, L: lit(U8, 200), R: lit(U8, 1)}, "144\n"},
		{"neg wraps", &Unary{Op: Neg, X: lit(I32, minI32)}, "-2147483648\n"},
		{"compl u8", &Unary{Op: Compl, X: lit(U8, 5)}, "250\n"},
		{"u64 compare", &Binary{Op: Gt, L: lit(U64, math.MaxUint64), R: lit(U64, 1)}, "true\n"},
		{"i64 compare", &Binary{Op: Gt, L: lit(I64, math.MaxUint64), R: lit(I64, 1)}, "false\n"},
		{"narrowing convert", &Convert{To: U8, X: lit(I64, math.MaxUint64)}, "255\n"},
		{"sign extending convert", &Convert{To: U64, X: lit(I32, 0xFFFFFFFF)}, "18446744073709551615\n"},
		{"zero extending convert", &Convert{To: I64, X: lit(U32, 0xFFFFFFFF)}, "4294967295\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome, err := Run(mainPrinting(tt.expr), 1000)
			if err != nil {
				t.Fatal(err)
			}
			if outcome.Stdout != tt.want || outcome.Panic != "" {
				t.Errorf("stdout %q panic %q, want %q", outcome.Stdout, outcome.Panic, tt.want)
			}
		})
	}
}

func TestRunFloat(t *testing.T) {
	nan := &Binary{Op: Div, L: &Lit{Kind: F64}, R: &Lit{Kind: F64}}
	outcome, err := Run(mainPrinting(
		&Binary{Op: Eq, L: nan, R: nan},
		&Binary{Op: Ne, L: nan, R: nan},
		&Binary{Op: Lt, L: &Lit{Kind: F32, Float: 0.1}, R: &Lit{Kind: F32, Float: 0.2}},
	), 1000)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Stdout != "false\ntrue\ntrue\n" {
		t.Errorf("stdout %q", outcome.Stdout)
	}
}

func TestRunDivideByZeroPanics(t *testing.T) {
	outcome, err := Run(mainPrinting(
		lit(I64, 5),
		&Binary{Op: Div, L: lit(I64, 1), R: lit(I64, 0)},
		lit(I64, 6),
	), 1000)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Stdout != "5\n" || outcome.ExitCode() != 2 || outcome.Stderr() != "panic: integer divide by zero\n" {
		t.Errorf("outcome %+v", outcome)
	}
}

func TestRunLabelledJump(t *testing.T) {
	counter := &Var{Name: "n", Kind: I64}
	inner := &Range{ID: 2, Var: "j", Kind: I64, Lo: lit(I64, 0), Hi: lit(I64, 3), Body: []Stmt{
		&IfStmt{
			Cond: &Binary{Op: Eq, L: &Var{Name: "j", Kind: I64}, R: lit(I64, 1)},
			Then: []Stmt{&Jump{Target: 1, Continue: true}},
		},
		&Assign{Name: "n", Kind: I64, Op: Add, Value: lit(I64, 1)},
	}}
	outer := &Range{ID: 1, Var: "i", Kind: I64, Lo: lit(I64, 0), Hi: lit(I64, 3), Body: []Stmt{inner}}
	p := &Program{Main: []Stmt{
		&Let{Name: "n", Kind: I64, Mutable: true, Init: lit(I64, 0)},
		outer,
		&Print{Value: counter},
	}}
	outcome, err := Run(p, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Stdout != "3\n" {
		t.Errorf("stdout %q", outcome.Stdout)
	}
}

func TestRunBudget(t *testing.T) {
	p := &Program{Main: []Stmt{&Range{ID: 1, Var: "i", Kind: U64, Lo: lit(U64, 0), Hi: lit(U64, 1<<40), Body: []Stmt{
		&Print{Value: lit(I64, 1)},
	}}}}
	if _, err := Run(p, 500); !errors.Is(err, ErrBudget) {
		t.Errorf("error %v, want %v", err, ErrBudget)
	}
}

func TestRunUndefinedFloatConversion(t *testing.T) {
	p := mainPrinting(&Convert{To: U8, X: &Lit{Kind: F64, Float: 300}})
	if _, err := Run(p, 100); !errors.Is(err, ErrUndefined) {
		t.Errorf("error %v, want %v", err, ErrUndefined)
	}
}
