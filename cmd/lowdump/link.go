package main

import (
	"fmt"
	"os"
	"strings"

	"lowlang/internal/asm"
	"lowlang/internal/compiler"
	"lowlang/internal/link"
	"lowlang/internal/target"
)

type linkOptions struct {
	platform  target.OS
	embeds    map[string][]byte
	libraries []string
	dirs      []string
	paths     []string
}

func parseLinkFlags(args []string) (linkOptions, error) {
	options := linkOptions{platform: target.Linux, embeds: map[string][]byte{}, dirs: link.SystemDirs}
	customDirs := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-windows":
			options.platform = target.Windows
		case "-embed", "-lib", "-libdir":
			flag := args[i]
			i++
			if i == len(args) {
				return options, fmt.Errorf("%s", usage)
			}
			switch flag {
			case "-embed":
				name, path, ok := strings.Cut(args[i], "=")
				if !ok {
					return options, fmt.Errorf("%s", usage)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return options, err
				}
				options.embeds[name] = data
			case "-lib":
				options.libraries = append(options.libraries, args[i])
			case "-libdir":
				if !customDirs {
					options.dirs = nil
					customDirs = true
				}
				options.dirs = append(options.dirs, args[i])
			}
		default:
			options.paths = append(options.paths, args[i])
		}
	}
	return options, nil
}

func dumpLinkCommand(args []string) error {
	options, err := parseLinkFlags(args)
	if err != nil {
		return err
	}
	if len(options.paths) != 2 {
		return fmt.Errorf("%s", usage)
	}
	src, err := os.ReadFile(options.paths[0])
	if err != nil {
		return err
	}
	assembler := compiler.Assembler(options.platform)
	assembler.Embeds = options.embeds
	object, err := asm.Assemble(string(src), assembler)
	if err != nil {
		return err
	}
	bytes, err := linkObject(object, options)
	if err != nil {
		return err
	}
	return os.WriteFile(options.paths[1], bytes, 0o755)
}

func linkObject(object *asm.Object, options linkOptions) ([]byte, error) {
	if options.platform == target.Windows {
		return link.PE(object)
	}
	if len(options.libraries) == 0 {
		image, err := link.Static(object)
		return image.Bytes, err
	}
	resolved, err := link.Resolver{Dirs: options.dirs}.Resolve(options.libraries)
	if err != nil {
		return nil, err
	}
	image, err := link.Dynamic(object, resolved)
	return image.Bytes, err
}
