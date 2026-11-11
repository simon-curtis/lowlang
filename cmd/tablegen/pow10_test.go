package main

import (
	"bytes"
	"os"
	"testing"
)

func TestPow10TablesMatchGenerator(t *testing.T) {
	sections := pow10Sections()
	binary, err := os.ReadFile("../../internal/compiler/std/text/pow10.bin")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(binary, sections[0].data) {
		t.Errorf("std/text/pow10.bin differs from the generator; run tablegen pow10")
	}
	prelude, err := os.ReadFile("../../internal/compiler/prelude.low")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(prelude, sections[1].data) {
		t.Errorf("prelude.low __pow10_hi and __pow10_lo differ from the generator; run tablegen pow10")
	}
}
