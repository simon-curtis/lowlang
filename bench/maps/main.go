package main

import "fmt"

func main() {
	const n = 1000000
	m := map[int64]int64{}
	for i := range int64(n) {
		m[i*7] = i
	}
	var hits, sum int64
	for k := range int64(n * 2) {
		if v, ok := m[k]; ok {
			hits++
			sum += v
		}
	}
	for i := range int64(n) {
		if i%2 == 0 {
			delete(m, i*7)
		}
	}
	for i := range int64(n / 2) {
		m[i*7+1] = i
	}
	var total int64
	for k, v := range m {
		total += k + v
	}
	fmt.Println(hits)
	fmt.Println(sum)
	fmt.Println(len(m))
	fmt.Println(total)
}
