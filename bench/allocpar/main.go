package main

import (
	"fmt"
	"os"
	"strconv"
)

type Pair struct {
	a, b int64
}

func allocate(n, seed int64, sink **Pair) int64 {
	var sum int64
	keep := &Pair{a: seed}
	for i := range n {
		p := &Pair{a: i, b: seed}
		sum += p.a + p.b
		keep = p
		*sink = p
	}
	return sum + keep.a
}

func worker(n, seed int64, results chan<- int64) {
	var sink *Pair
	results <- allocate(n, seed, &sink)
}

func workers() int64 {
	if len(os.Args) > 1 {
		if n, err := strconv.ParseInt(os.Args[1], 10, 64); err == nil {
			return n
		}
	}
	return 4
}

func main() {
	const total = 100000000
	w := workers()
	results := make(chan int64)
	for k := range w {
		go worker(total/w, k, results)
	}
	var sum int64
	for range w {
		sum += <-results
	}
	fmt.Println(sum)
}
