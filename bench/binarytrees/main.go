package main

import "fmt"

type tree struct {
	left, right *tree
}

func makeTree(depth int64) *tree {
	if depth == 0 {
		return &tree{}
	}
	return &tree{left: makeTree(depth - 1), right: makeTree(depth - 1)}
}

func (t *tree) check() int64 {
	var total int64 = 1
	if t.left != nil {
		total += t.left.check()
	}
	if t.right != nil {
		total += t.right.check()
	}
	return total
}

func main() {
	const maxDepth = 18
	const stretch = maxDepth + 1
	fmt.Printf("stretch tree of depth %d\t check: %d\n", stretch, makeTree(stretch).check())
	longLived := makeTree(maxDepth)
	for depth := int64(4); depth <= maxDepth; depth += 2 {
		iterations := int64(1) << (maxDepth - depth + 4)
		var sum int64
		for range iterations {
			sum += makeTree(depth).check()
		}
		fmt.Printf("%d\t trees of depth %d\t check: %d\n", iterations, depth, sum)
	}
	fmt.Printf("long lived tree of depth %d\t check: %d\n", maxDepth, longLived.check())
}
