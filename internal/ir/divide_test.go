package ir

import (
	"math"
	"math/bits"
	"math/rand"
	"testing"
)

func unsignedQuotient(m unsignedMagic, x uint64) uint64 {
	high, _ := bits.Mul64(m.multiplier, x)
	if m.add {
		high = ((x - high) >> 1) + high
	}
	return high >> m.shift
}

func signedQuotient(m signedMagic, x int64) int64 {
	high, _ := bits.Mul64(uint64(m.multiplier), uint64(x))
	product := int64(high)
	if m.multiplier < 0 {
		product -= x
	}
	if x < 0 {
		product -= m.multiplier
	}
	if m.add {
		product += x
	}
	product >>= m.shift
	return product + int64(uint64(x)>>63)
}

func dividends() []uint64 {
	values := []uint64{0, 1, 2, 3, 5, 7, 9, 10, 11, 100, 1000, math.MaxInt32, math.MaxInt32 + 1, math.MaxUint32,
		math.MaxInt64 - 1, math.MaxInt64, math.MaxInt64 + 1, math.MaxInt64 + 2, math.MaxUint64 - 1, math.MaxUint64}
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 200; i++ {
		values = append(values, rng.Uint64(), rng.Uint64()>>uint(rng.Intn(64)))
	}
	for i := 0; i < 64; i++ {
		values = append(values, 1<<i, 1<<i-1, 1<<i+1)
	}
	return values
}

func divisors() []uint64 {
	var values []uint64
	for d := uint64(3); d < 3000; d++ {
		values = append(values, d)
	}
	rng := rand.New(rand.NewSource(2))
	for i := 0; i < 2000; i++ {
		values = append(values, rng.Uint64()>>uint(rng.Intn(62)))
	}
	for i := 2; i < 64; i++ {
		values = append(values, 1<<i-1, 1<<i+1, 1<<i+3)
	}
	values = append(values, math.MaxUint64, math.MaxUint64-1, math.MaxInt64, math.MaxInt64-1, math.MaxInt64+1, math.MaxInt64+2)
	return values
}

func TestMagicNumbersMatchDivision(t *testing.T) {
	for _, d := range divisors() {
		if d&(d-1) == 0 || d < 3 {
			continue
		}
		unsigned := newUnsignedMagic(d)
		var signed signedMagic
		if d <= math.MaxInt64 {
			signed = newSignedMagic(d)
		}
		for _, u := range dividends() {
			if got := unsignedQuotient(unsigned, u); got != u/d {
				t.Fatalf("%d / %d = %d, unsigned magic gives %d (%+v)", u, d, u/d, got, unsigned)
			}
			if d > math.MaxInt64 {
				continue
			}
			x := int64(u)
			if got := signedQuotient(signed, x); got != x/int64(d) {
				t.Fatalf("%d / %d = %d, signed magic gives %d (%+v)", x, d, x/int64(d), got, signed)
			}
		}
	}
}
