package main

import "fmt"

func main() {
	counts := map[string]int64{}
	var length int64
	for i := range 1000000 {
		word := fmt.Sprintf("w%d-%d", i%5000, i%7)
		length += int64(len(word))
		counts[word]++
	}
	var points, matches int64
	for word, count := range counts {
		for _, c := range word {
			points += int64(c)
		}
		if word == "w42-0" {
			matches += count
		}
	}
	fmt.Println(len(counts))
	fmt.Println(length)
	fmt.Println(points)
	fmt.Println(matches)
}
