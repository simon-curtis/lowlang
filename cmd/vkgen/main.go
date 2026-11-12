package main

import (
	"flag"
	"fmt"
	"os"

	"lowlang/internal/vkgen"
)

func main() {
	output := flag.String("o", "", "output .low file")
	registry := flag.String("registry", "", "vk.xml or vk.xml.gz")
	selection := flag.String("select", "", "selection list of commands and extensions")
	flag.Parse()
	if *output == "" || *registry == "" || *selection == "" {
		fmt.Fprintln(os.Stderr, "usage: vkgen -registry vk.xml.gz -select vulkan.list -o vulkan.low")
		os.Exit(2)
	}
	if err := run(*output, *registry, *selection); err != nil {
		fmt.Fprintf(os.Stderr, "vkgen: %v\n", err)
		os.Exit(1)
	}
}

func run(output, registryPath, selectionPath string) error {
	reg, err := vkgen.ReadRegistry(registryPath)
	if err != nil {
		return err
	}
	text, err := os.ReadFile(selectionPath)
	if err != nil {
		return err
	}
	sel, err := vkgen.ParseSelection(string(text))
	if err != nil {
		return err
	}
	source, err := vkgen.Generate(reg, sel)
	if err != nil {
		return err
	}
	return os.WriteFile(output, []byte(source), 0o644)
}
