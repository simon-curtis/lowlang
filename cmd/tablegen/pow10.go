package main

import (
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"
)

const (
	pow10Min = -348
	pow10Max = 347
)

type pow10Entry struct {
	hi uint64
	lo uint64
}

func generatePow10(source, output string) error {
	return writeSections(output, pow10Sections())
}

func pow10Sections() []section {
	entries := pow10Entries()
	data := make([]byte, 0, 16*len(entries))
	for _, e := range entries {
		data = binary.LittleEndian.AppendUint64(data, e.hi)
		data = binary.LittleEndian.AppendUint64(data, e.lo)
	}
	return []section{
		{"pow10.bin", data},
		{"pow10_prelude.low", []byte(pow10Source(entries))},
	}
}

func pow10Entries() []pow10Entry {
	one := big.NewInt(1)
	ten := big.NewInt(10)
	b1p64 := new(big.Int).Lsh(one, 64)
	limit := new(big.Rat).SetInt(new(big.Int).Lsh(one, 128))
	two := big.NewRat(2, 1)
	entries := make([]pow10Entry, 0, pow10Max-pow10Min+1)
	for e := int64(pow10Min); e <= pow10Max; e++ {
		var r *big.Rat
		if e >= 0 {
			r = new(big.Rat).SetInt(new(big.Int).Exp(ten, big.NewInt(e), nil))
		} else {
			r = new(big.Rat).SetFrac(one, new(big.Int).Exp(ten, big.NewInt(-e), nil))
		}
		for r.Cmp(limit) < 0 {
			r.Mul(r, two)
		}
		for r.Cmp(limit) >= 0 {
			r.Quo(r, two)
		}
		d := new(big.Int).Div(r.Num(), r.Denom())
		hi, lo := new(big.Int).DivMod(d, b1p64, new(big.Int))
		high, low := hi.Uint64(), lo.Uint64()
		if !r.IsInt() {
			low++
			if low == 0 {
				high++
			}
		}
		if low != 0 {
			high++
			low = -low
		}
		entries = append(entries, pow10Entry{high, low})
	}
	return entries
}

func pow10Source(entries []pow10Entry) string {
	var out strings.Builder
	out.WriteString(pow10Function("__pow10_hi", entries, func(e pow10Entry) uint64 { return e.hi }))
	out.WriteString("\n")
	out.WriteString(pow10Function("__pow10_lo", entries, func(e pow10Entry) uint64 { return e.lo }))
	return out.String()
}

func pow10Function(name string, entries []pow10Entry, field func(pow10Entry) uint64) string {
	const perLine = 4
	var out strings.Builder
	fmt.Fprintf(&out, "fn %s(i: i64) -> u64 {\n    let t: []u64 = [\n", name)
	for start := 0; start < len(entries); start += perLine {
		end := min(start+perLine, len(entries))
		words := make([]string, 0, perLine)
		for _, e := range entries[start:end] {
			words = append(words, fmt.Sprintf("0x%016x", field(e)))
		}
		line := strings.Join(words, ", ")
		if end < len(entries) {
			line += ","
		}
		fmt.Fprintf(&out, "        %s\n", line)
	}
	out.WriteString("    ]\n    t[i]\n}\n")
	return out.String()
}
