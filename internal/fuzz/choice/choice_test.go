package choice

import "testing"

func TestSeedIsDeterministic(t *testing.T) {
	a, b := FromSeed(42), FromSeed(42)
	for range 100 {
		if x, y := a.Intn(1000), b.Intn(1000); x != y {
			t.Fatalf("%d != %d", x, y)
		}
	}
}

func TestBytesAreConsumedInOrder(t *testing.T) {
	s := FromBytes([]byte{3, 200, 1})
	got := []int{s.Intn(10), s.Intn(256), s.Intn(2)}
	want := []int{3, 200, 1}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("choice %d is %d, want %d", i, got[i], want[i])
		}
	}
}

func TestExhaustedBytesYieldZero(t *testing.T) {
	s := FromBytes(nil)
	if s.Intn(10) != 0 || s.Uint64() != 0 || s.Bool() {
		t.Error("an empty source must produce zero choices")
	}
}

func TestRangeIsInclusive(t *testing.T) {
	s := FromSeed(1)
	seen := map[int]bool{}
	for range 200 {
		v := s.Range(2, 4)
		if v < 2 || v > 4 {
			t.Fatalf("%d out of range", v)
		}
		seen[v] = true
	}
	if len(seen) != 3 {
		t.Errorf("saw %v", seen)
	}
}
