package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: tablegen unicode <ucd directory> <output directory>")
		fmt.Fprintln(os.Stderr, "       tablegen properties <ucd directory> <output directory>")
		fmt.Fprintln(os.Stderr, "       tablegen charset <index directory> <output directory>")
		fmt.Fprintln(os.Stderr, "       tablegen entities <entities.json> <output directory>")
		os.Exit(2)
	}
	if err := run(os.Args[1], os.Args[2], os.Args[3]); err != nil {
		fmt.Fprintf(os.Stderr, "tablegen: %v\n", err)
		os.Exit(1)
	}
}

func run(command, source, output string) error {
	switch command {
	case "unicode":
		return generateUnicode(source, output)
	case "properties":
		return generateProperties(source, output)
	case "charset":
		return generateCharset(source, output)
	case "entities":
		return generateEntities(source, output)
	}
	return fmt.Errorf("unknown command %q", command)
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
