package main

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"lowlang/internal/asm"
	"lowlang/internal/compiler"
	"lowlang/internal/lexer"
	"lowlang/internal/link"
	"lowlang/internal/target"
)

func dumpPatchCommand(out io.Writer, args []string) error {
	options, err := parseLinkFlags(args)
	if err != nil {
		return err
	}
	if len(options.paths) != 3 {
		return fmt.Errorf("%s", usage)
	}
	src, err := os.ReadFile(options.paths[0])
	if err != nil {
		return err
	}
	base, err := strconv.ParseUint(options.paths[1], 10, 64)
	if err != nil {
		return fmt.Errorf("%s", usage)
	}
	running, err := readRunning(options.paths[2])
	if err != nil {
		return err
	}
	assembler := compiler.Assembler(target.Linux)
	assembler.Embeds = options.embeds
	object, err := asm.Assemble(string(src), assembler)
	if err != nil {
		return err
	}
	image, err := link.Patch(object, base, running)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "address %d\n", image.Address)
	fmt.Fprintf(out, "code %s\n", hex.EncodeToString(image.Code))
	names := make([]string, 0, len(image.Symbols))
	for name := range image.Symbols {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		fmt.Fprintf(out, "symbol %s %d\n", lexer.Quote(name), image.Symbols[name])
	}
	return nil
}

func readRunning(path string) (map[string]uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	running := map[string]uint64{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		address, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s", usage)
		}
		running[fields[0]] = address
	}
	return running, nil
}
