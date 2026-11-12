package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"lowlang/internal/vkgen"
)

func main() {
	output := flag.String("o", "", "output directory for vulkan.low and one vulkan_<system>.low per target")
	registry := flag.String("registry", "", "vk.xml or vk.xml.gz")
	selection := flag.String("select", "", "selection list of extensions, commands, types and libraries")
	flag.Parse()
	if *output == "" || *registry == "" || *selection == "" {
		fmt.Fprintln(os.Stderr, "usage: vkgen -registry vk.xml.gz -select vulkan.list -o directory")
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
	files, err := vkgen.Generate(reg, sel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		return err
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(output, file.Name), []byte(file.Source), 0o644); err != nil {
			return err
		}
	}
	return nil
}
