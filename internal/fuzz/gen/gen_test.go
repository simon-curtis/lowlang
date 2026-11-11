package gen

import (
	"errors"
	"testing"

	"lowlang/internal/fuzz/choice"
	"lowlang/internal/fuzz/model"
)

func TestGenerateIsDeterministic(t *testing.T) {
	first := Generate(choice.FromSeed(7), DefaultConfig).String()
	second := Generate(choice.FromSeed(7), DefaultConfig).String()
	if first != second {
		t.Error("the same seed produced different programs")
	}
	other := Generate(choice.FromSeed(8), DefaultConfig).String()
	if first == other {
		t.Error("different seeds produced the same program")
	}
}

func TestGenerateFromBytes(t *testing.T) {
	inputs := [][]byte{nil, {0}, {255, 255, 255}, make([]byte, 4096)}
	for _, in := range inputs {
		program := Generate(choice.FromBytes(in), DefaultConfig)
		if program.String() == "" {
			t.Errorf("empty source for %d input bytes", len(in))
		}
	}
}

func TestGeneratedProgramsTerminateWithinBudget(t *testing.T) {
	skipped := 0
	for seed := int64(0); seed < 300; seed++ {
		program := Generate(choice.FromSeed(seed), DefaultConfig)
		_, err := model.Run(program, 2_000_000)
		switch {
		case errors.Is(err, model.ErrBudget):
			skipped++
		case err != nil:
			t.Fatalf("seed %d: %v", seed, err)
		}
	}
	if skipped > 15 {
		t.Errorf("%d of 300 programs exceeded the step budget", skipped)
	}
}

func TestPanicsDisabledAvoidsRawDivisors(t *testing.T) {
	cfg := DefaultConfig
	cfg.Panics = false
	for seed := int64(0); seed < 200; seed++ {
		program := Generate(choice.FromSeed(seed), cfg)
		outcome, err := model.Run(program, 2_000_000)
		if err != nil {
			continue
		}
		if outcome.Panic != "" {
			t.Fatalf("seed %d panicked: %s", seed, outcome.Panic)
		}
	}
}
