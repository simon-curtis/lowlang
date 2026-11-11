package main

import (
	"fmt"
	"os"
)

type Pair struct {
	a, b int64
}

var sink *Pair

func allocate(n int64) int64 {
	var sum int64
	keep := &Pair{}
	for i := range n {
		p := &Pair{a: i, b: 1}
		sum += p.a + p.b
		keep = p
		sink = p
	}
	return sum + keep.a
}

func control(n int64) int64 {
	var sum int64
	keep := Pair{}
	for i := range n {
		p := Pair{a: i, b: 1}
		sum += p.a + p.b
		keep = p
	}
	return sum + keep.a
}

func main() {
	const n = 100000000
	if len(os.Args) > 1 && os.Args[1] == "control" {
		fmt.Println(control(n))
	} else {
		fmt.Println(allocate(n))
	}
}
