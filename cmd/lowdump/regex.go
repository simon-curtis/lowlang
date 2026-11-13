package main

import (
	"fmt"
	"io"
	"strings"

	"lowlang/internal/regex"
)

func dumpRegexCommand(out io.Writer, src []byte) {
	for _, pattern := range strings.Split(strings.TrimSuffix(string(src), "\n"), "\n") {
		dumpRegex(out, pattern)
	}
}

func dumpRegex(out io.Writer, pattern string) {
	prog, err := regex.Compile(pattern)
	if err != nil {
		fmt.Fprintf(out, "error: %s\n---\n", err)
		return
	}
	fmt.Fprintf(out, "%s---\n", regex.Generate("__regex_test", pattern, prog))
}
