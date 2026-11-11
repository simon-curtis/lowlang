package main

import "fmt"

type Node struct {
	next  *Node
	value int64
}

type Pair struct {
	a, b int64
}

var sink *Pair

func buffer(n, seed int64) string {
	b := make([]byte, 0, 1024)
	for i := range n {
		b = append(b, byte(97+(i*31+seed)%26))
	}
	return string(b)
}

func chain(n int64) *Node {
	var head *Node
	for i := range n {
		head = &Node{next: head, value: i}
	}
	return head
}

func total(head *Node) int64 {
	var sum int64
	for cur := head; cur != nil; cur = cur.next {
		sum += cur.value
	}
	return sum
}

func views(source string, count int64) []string {
	var out []string
	span := int64(len(source)) - 64
	for k := range count {
		off := (k * 7919) % span
		out = append(out, source[off:off+1+k%40])
	}
	return out
}

func weight(list []string) int64 {
	var sum int64
	for _, s := range list {
		sum += int64(len(s)) + int64(s[0])
	}
	return sum
}

func churn(rounds int64) int64 {
	var sum int64
	for i := range rounds {
		p := &Pair{a: i, b: 1}
		sink = p
		sum += p.a + p.b
		if i%64 == 0 {
			var scratch []int64
			for k := range int64(48) {
				scratch = append(scratch, k+i)
			}
			sum += scratch[47]
		}
	}
	return sum
}

func main() {
	first := buffer(32*1024*1024, 1)
	second := buffer(32*1024*1024, 2)
	a := views(first, 150000)
	b := views(second, 150000)
	list := chain(500000)

	names := map[string]int64{}
	for i := range int64(150000) {
		names[fmt.Sprintf("key-%d", i)] = i
	}
	numbers := map[int64]int64{}
	for i := range int64(200000) {
		numbers[i*3] = i
	}

	fmt.Println(churn(120000000))

	var hits int64
	for i := range 150000 {
		hits += names[fmt.Sprintf("key-%d", i)]
	}
	fmt.Println(len(first) + len(second))
	fmt.Println(weight(a) + weight(b))
	fmt.Println(total(list))
	fmt.Println(hits)
	fmt.Println(len(numbers))
}
