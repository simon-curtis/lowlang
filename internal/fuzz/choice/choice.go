package choice

import (
	"encoding/binary"
	"math/rand"
)

type Source struct {
	rng   *rand.Rand
	bytes []byte
	pos   int
}

func FromSeed(seed int64) *Source {
	return &Source{rng: rand.New(rand.NewSource(seed))}
}

func FromBytes(data []byte) *Source {
	return &Source{bytes: data}
}

func (s *Source) Uint64() uint64 {
	if s.rng != nil {
		return s.rng.Uint64()
	}
	var word [8]byte
	n := copy(word[:], s.bytes[min(s.pos, len(s.bytes)):])
	s.pos += n
	return binary.LittleEndian.Uint64(word[:])
}

func (s *Source) Intn(n int) int {
	if n <= 1 {
		return 0
	}
	if s.rng != nil {
		return s.rng.Intn(n)
	}
	if n <= 256 {
		return s.byteChoice() % n
	}
	return int(s.Uint64() % uint64(n))
}

func (s *Source) byteChoice() int {
	if s.pos >= len(s.bytes) {
		return 0
	}
	b := s.bytes[s.pos]
	s.pos++
	return int(b)
}

func (s *Source) Bool() bool {
	return s.Intn(2) == 1
}

func (s *Source) Chance(percent int) bool {
	return s.Intn(100) < percent
}

func (s *Source) Range(lo, hi int) int {
	return lo + s.Intn(hi-lo+1)
}

func Pick[T any](s *Source, items []T) T {
	return items[s.Intn(len(items))]
}
