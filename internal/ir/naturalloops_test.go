package ir

import (
	"slices"
	"testing"
)

func loopsOf(edges map[int][]int) []*NaturalLoop[int] {
	preds := map[int][]int{}
	for from, tos := range edges {
		for _, to := range tos {
			preds[to] = append(preds[to], from)
		}
	}
	doms := NewDominators(0, func(n int) []int { return edges[n] }, func(n int) []int { return preds[n] })
	return doms.Loops()
}

func bodyOf(l *NaturalLoop[int]) []int {
	var body []int
	for n := range l.Body {
		body = append(body, n)
	}
	slices.Sort(body)
	return body
}

func TestNestedLoopsAreOrderedInnermostFirst(t *testing.T) {
	loops := loopsOf(map[int][]int{0: {1}, 1: {2, 5}, 2: {3}, 3: {2, 4}, 4: {1}})
	if len(loops) != 2 {
		t.Fatalf("found %d loops, want 2", len(loops))
	}
	if got := bodyOf(loops[0]); !slices.Equal(got, []int{2, 3}) || loops[0].Header != 2 {
		t.Errorf("inner loop %v header %d", got, loops[0].Header)
	}
	if got := bodyOf(loops[1]); !slices.Equal(got, []int{1, 2, 3, 4}) || loops[1].Header != 1 {
		t.Errorf("outer loop %v header %d", got, loops[1].Header)
	}
}

func TestCycleWithTwoEntriesIsNotALoop(t *testing.T) {
	loops := loopsOf(map[int][]int{0: {1, 2}, 1: {2, 3}, 2: {1, 3}})
	if len(loops) != 0 {
		t.Errorf("found %d loops in an irreducible cycle, want 0", len(loops))
	}
}
