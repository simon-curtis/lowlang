package main

import "fmt"

func sieve(limit int64) int64 {
	composite := make([]uint8, limit)
	var count int64
	for i := int64(2); i < limit; i++ {
		if composite[i] == 0 {
			count++
			for j := i * i; j < limit; j += i {
				composite[j] = 1
			}
		}
	}
	return count
}

func main() {
	var total int64
	for range 5 {
		total += sieve(10000000)
	}
	fmt.Println(total)
}
