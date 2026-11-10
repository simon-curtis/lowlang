package main

import "fmt"

func escapes(cr, ci float64, limit int) bool {
	var zr, zi float64
	for range limit {
		tr := zr*zr - zi*zi + cr
		zi = 2*zr*zi + ci
		zr = tr
		if zr*zr+zi*zi > 4 {
			return true
		}
	}
	return false
}

func main() {
	const size = 2000
	const scale = 2.0 / size
	var inside int64
	for y := range size {
		ci := float64(y)*scale - 1
		for x := range size {
			cr := float64(x)*scale - 1.5
			if !escapes(cr, ci, 50) {
				inside++
			}
		}
	}
	fmt.Println(inside)
}
