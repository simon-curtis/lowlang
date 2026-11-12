package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type command struct {
	name      string
	arguments string
	run       func(paths []string) error
}

var commands = []command{
	{"unicode", "<ucd directory> <output directory>", sourceAndOutput(generateUnicode)},
	{"properties", "<ucd directory> <output directory>", sourceAndOutput(generateProperties)},
	{"charset", "<index directory> <output directory>", sourceAndOutput(generateCharset)},
	{"entities", "<entities.json> <output directory>", sourceAndOutput(generateEntities)},
	{"pow10", "<output directory>", outputOnly(generatePow10)},
	{"tzdata", "<zoneinfo directory with tzdata.zi> <output directory>", sourceAndOutput(generateTzdata)},
}

func sourceAndOutput(generate func(source, output string) error) func([]string) error {
	return func(paths []string) error {
		return generate(paths[0], paths[1])
	}
}

func outputOnly(generate func(output string) error) func([]string) error {
	return func(paths []string) error {
		return generate(paths[0])
	}
}

func (c command) parameters() int {
	return len(strings.Fields(c.arguments))
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		var usage usageError
		if errors.As(err, &usage) {
			fmt.Fprintln(os.Stderr, usage.Error())
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "tablegen: %v\n", err)
		os.Exit(1)
	}
}

type usageError struct{}

func (usageError) Error() string {
	var text strings.Builder
	for i, c := range commands {
		prefix := "       "
		if i == 0 {
			prefix = "usage:"
		}
		fmt.Fprintf(&text, "%s tablegen %s %s\n", prefix, c.name, c.arguments)
	}
	return strings.TrimSuffix(text.String(), "\n")
}

func run(arguments []string) error {
	if len(arguments) == 0 {
		return usageError{}
	}
	for _, c := range commands {
		if c.name != arguments[0] {
			continue
		}
		if len(arguments)-1 != c.parameters() {
			return usageError{}
		}
		return c.run(arguments[1:])
	}
	return fmt.Errorf("unknown command %q", arguments[0])
}

type section struct {
	name string
	data []byte
}

func writeSections(dir string, sections []section) error {
	for _, s := range sections {
		path := fmt.Sprintf("%s/%s", dir, s.name)
		if err := os.WriteFile(path, s.data, 0o644); err != nil {
			return err
		}
		fmt.Printf("%s: %d bytes\n", s.name, len(s.data))
	}
	return nil
}

func appendU16(out []byte, v int) []byte {
	return append(out, byte(v), byte(v>>8))
}

func appendU32(out []byte, v rune) []byte {
	return append(out, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}
