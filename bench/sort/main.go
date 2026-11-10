package main

import "fmt"

func quicksort[T any](xs []T, lo, hi int, less func(T, T) bool) {
	if hi-lo < 2 {
		return
	}
	pivot := xs[lo+(hi-lo)/2]
	i, j := lo, hi-1
	for i <= j {
		for less(xs[i], pivot) {
			i++
		}
		for less(pivot, xs[j]) {
			j--
		}
		if i <= j {
			xs[i], xs[j] = xs[j], xs[i]
			i++
			j--
		}
	}
	quicksort(xs, lo, j+1, less)
	quicksort(xs, i, hi, less)
}

func main() {
	const n = 3000000
	xs := make([]int64, 0, n)
	var seed int64 = 42
	for range n {
		seed = (seed*1103515245 + 12345) & 0x7fffffff
		xs = append(xs, seed)
	}
	quicksort(xs, 0, len(xs), func(a, b int64) bool { return a < b })
	var unsorted, checksum int64
	for i, x := range xs {
		if i > 0 && xs[i-1] > x {
			unsorted++
		}
		if i%1000 == 0 {
			checksum += x
		}
	}
	fmt.Println(unsorted)
	fmt.Println(checksum)
}
