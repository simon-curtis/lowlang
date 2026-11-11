package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func fixtureSource(t *testing.T, name string) string {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("..", "..", "testdata", "run", fmt.Sprintf("%s.low", name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(source)
}

const squareMarking = `fn main() {
    let limit = %d
    var composite: []u8 = []
    for i in 0..limit {
        push(mut composite, u8(0))
    }
    var count = 0
    for i in 2..limit {
        if composite[i] == 0 {
            count += 1
            var j = i * i
            for j < limit {
                composite[j] = 1
                j += i
            }
        }
    }
    print(count)
}
`

func TestSquareBoundDecidesTheMarkingCheck(t *testing.T) {
	cases := []struct {
		limit  int64
		checks int
	}{
		{1000, 0},
		{3037000500, 0},
		{3037000501, 1},
		{3037000502, 1},
	}
	for _, c := range cases {
		t.Run(fmt.Sprint(c.limit), func(t *testing.T) {
			prog := analyseRelease(t, fmt.Sprintf(squareMarking, c.limit))
			f := function(t, prog, "main")
			if got := countChecks(f); got != c.checks {
				t.Errorf("%d checks, want %d\n%s", got, c.checks, f)
			}
		})
	}
}

const steppedMarking = `fn main() {
    var composite: []u8 = []
    for i in 0..1000 {
        push(mut composite, u8(0))
    }
    var j = %d
    for j < 1000 {
        composite[j] = 1
        j += %d
    }
    print(composite[0])
}
`

func TestSteppedIndexKeepsItsCheckOnlyWhenTheAddCanWrap(t *testing.T) {
	cases := []struct {
		name   string
		start  int64
		step   int64
		checks int
	}{
		{"small step", 0, 7, 0},
		{"largest step that cannot wrap", 5, 9223372036854774808, 0},
		{"smallest step that can wrap", 5, 9223372036854774809, 1},
		{"step near the maximum", 5, 9223372036854775805, 1},
		{"negative start", -3, 1, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prog := analyseRelease(t, fmt.Sprintf(steppedMarking, c.start, c.step))
			f := function(t, prog, "main")
			if got := countChecks(f); got != c.checks {
				t.Errorf("%d checks, want %d\n%s", got, c.checks, f)
			}
		})
	}
}

func TestSpecialisedLimitRemovesTheCheck(t *testing.T) {
	prog := analyseRelease(t, fixtureSource(t, "range_specialise_constant"))
	f := function(t, prog, "sieve")
	if f.ParamWords != 0 {
		t.Errorf("sieve keeps %d parameter words", f.ParamWords)
	}
	if got := countChecks(f); got != 0 {
		t.Errorf("%d checks remain\n%s", got, f)
	}
}

func TestConstantSpecialisationNeedsEveryCallToAgree(t *testing.T) {
	cases := []struct {
		fixture  string
		function string
		params   int
	}{
		{"range_specialise_constant", "sieve", 0},
		{"range_specialise_mixed", "sieve", 1},
		{"range_specialise_address", "sieve", 1},
		{"range_specialise_recursion", "marks", 1},
		{"range_specialise_recursion", "climb", 3},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s/%s", c.fixture, c.function), func(t *testing.T) {
			prog := analyseRelease(t, fixtureSource(t, c.fixture))
			f := function(t, prog, c.function)
			if f.ParamWords != c.params {
				t.Errorf("%d parameter words, want %d\n%s", f.ParamWords, c.params, f)
			}
		})
	}
}

func TestDevBuildKeepsConstantParameters(t *testing.T) {
	prog := analyse(t, fixtureSource(t, "range_specialise_constant"))
	f := function(t, prog, "sieve")
	if f.ParamWords != 1 {
		t.Errorf("dev build specialised sieve: %d parameter words", f.ParamWords)
	}
}

const idleConstant = `fn spread(xs: []i64, scale: i64) -> i64 {
    var total = 0
    for i in 0..xs.len {
        total += xs[i] * scale
    }
    for i in 0..xs.len {
        total += xs[i]
    }
    total
}

fn main() {
    var xs: []i64 = []
    for i in 0..10 {
        push(mut xs, i)
    }
    print(spread(xs, 3))
}
`

func TestConstantThatFoldsNothingStaysAParameter(t *testing.T) {
	prog := analyseRelease(t, idleConstant)
	f := function(t, prog, "spread")
	if f.ParamWords != 4 {
		t.Errorf("%d parameter words, want 4 (the slice and the scale)\n%s", f.ParamWords, f)
	}
}
