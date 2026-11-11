package main

import "fmt"

func applyAll(xs []int64, f func(int64) int64) int64 {
	var total int64
	for _, x := range xs {
		total += f(x)
	}
	var lowBits int64
	for _, x := range xs {
		lowBits += f(x) & 1
	}
	return total + lowBits
}

func countIf(xs []int64, keep func(int64) bool) int64 {
	var n int64
	for _, x := range xs {
		if keep(x) {
			n++
		}
	}
	var rest int64
	for _, x := range xs {
		if !keep(x) {
			rest += 2
		}
	}
	return n + rest
}

func main() {
	xs := make([]int64, 0, 2000)
	for i := range int64(2000) {
		xs = append(xs, i)
	}
	var total, hits int64
	for round := range int64(5000) {
		total += applyAll(xs, func(x int64) int64 { return x*3 + 1 })
		total += applyAll(xs, func(x int64) int64 { return (x ^ round) & 1023 })
		total += applyAll(xs, func(x int64) int64 { return x >> 1 })
		hits += countIf(xs, func(x int64) bool { return x%7 == 0 })
		hits += countIf(xs, func(x int64) bool { return x > round%2000 })
	}
	fmt.Println(total)
	fmt.Println(hits)
}
