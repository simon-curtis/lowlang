package main

import "fmt"

func echo(input <-chan int64, output chan<- int64) {
	for v := range input {
		output <- v + 1
	}
	close(output)
}

func produce(jobs chan<- int64, n int64) {
	for i := range n {
		jobs <- i
	}
	close(jobs)
}

func work(jobs <-chan int64, results chan<- int64) {
	var sum int64
	for j := range jobs {
		sum += j * 2
	}
	results <- sum
}

func main() {
	ping := make(chan int64)
	pong := make(chan int64)
	go echo(ping, pong)
	var x int64
	for range 500000 {
		ping <- x
		x = <-pong
	}
	close(ping)
	fmt.Println(x)

	const workers = 4
	jobs := make(chan int64, 128)
	results := make(chan int64)
	go produce(jobs, 2000000)
	for range workers {
		go work(jobs, results)
	}
	var total int64
	for range workers {
		total += <-results
	}
	fmt.Println(total)
}
