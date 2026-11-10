package main

import (
	"flag"
	"fmt"
	"os"

	"lowlang/internal/wlgen"
)

func main() {
	output := flag.String("o", "", "output .low file")
	flag.Parse()
	if *output == "" || flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: wlgen -o out.low protocol.xml...")
		os.Exit(2)
	}
	if err := run(*output, flag.Args()); err != nil {
		fmt.Fprintf(os.Stderr, "wlgen: %v\n", err)
		os.Exit(1)
	}
}

func run(output string, paths []string) error {
	var protocols []*wlgen.Protocol
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		p, err := wlgen.Parse(data)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		protocols = append(protocols, p)
	}
	source, err := wlgen.Generate(protocols)
	if err != nil {
		return err
	}
	return os.WriteFile(output, []byte(source), 0o644)
}
