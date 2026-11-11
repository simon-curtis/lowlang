package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func document(records int) []byte {
	var doc strings.Builder
	doc.WriteString("[")
	for i := range records {
		if i > 0 {
			doc.WriteString(",")
		}
		active := "false"
		if i%3 == 0 {
			active = "true"
		}
		fmt.Fprintf(&doc, "{\"id\":%d,\"name\":\"user-%d\",\"email\":\"user%d@example.org\",", i, i, i)
		fmt.Fprintf(&doc, "\"score\":%d.5,\"active\":%s,\"tags\":[\"alpha\",\"beta\",\"tag-%d\"],", i, active, i%97)
		fmt.Fprintf(&doc, "\"address\":{\"street\":\"%d Main Street\",\"zip\":%d,\"geo\":[%d.25,-%d.75]},", i, 10000+i%9000, i%90, i%180)
		doc.WriteString("\"note\":null}")
	}
	doc.WriteString("]")
	return []byte(doc.String())
}

func visit(v any) int64 {
	switch v := v.(type) {
	case nil:
		return 1
	case bool:
		return 1
	case float64:
		return 1 + int64(v)
	case string:
		return 1 + int64(len(v))
	case []any:
		total := int64(1)
		for _, item := range v {
			total += visit(item)
		}
		return total
	case map[string]any:
		total := int64(1)
		for name, value := range v {
			total += int64(len(name)) + visit(value)
		}
		return total
	}
	return 0
}

func decode(data []byte, rounds int64) int64 {
	var total int64
	for range rounds {
		var v any
		if err := json.Unmarshal(data, &v); err != nil {
			return -1
		}
		total += visit(v)
	}
	return total
}

func worker(data []byte, rounds int64, results chan<- int64) {
	results <- decode(data, rounds)
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
	const rounds = 512
	w := workers()
	data := document(2000)
	results := make(chan int64)
	for range w {
		go worker(data, rounds/w, results)
	}
	var sum int64
	for range w {
		sum += <-results
	}
	fmt.Println(len(data))
	fmt.Println(sum)
}
