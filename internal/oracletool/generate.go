package oracletool

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"lowlang/internal/amd64"
	"lowlang/internal/compiler"
	"lowlang/internal/target"
)

type Generated struct {
	Asm       string
	Libraries []string
}

func Generate(dir, path string, platform target.OS) (generated Generated, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("compiler panic: %v", r)
		}
	}()
	unit, err := compiler.Analyse(path, compiler.Release(platform))
	if err != nil {
		return Generated{}, err
	}
	out, err := amd64.Generate(unit.Program, unit.Platform, nil)
	if err != nil {
		return Generated{}, err
	}
	source, err := materialise(dir, out.Asm, unit.Embeds)
	if err != nil {
		return Generated{}, err
	}
	return Generated{Asm: source, Libraries: unit.Program.Libraries()}, nil
}

func materialise(dir, source string, embeds map[string][]byte) (string, error) {
	keys := make([]string, 0, len(embeds))
	for key := range embeds {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var replacements []string
	for _, key := range keys {
		sum := sha256.Sum256([]byte(key))
		file := filepath.Join(dir, fmt.Sprintf("embed-%x", sum[:8]))
		if err := os.WriteFile(file, embeds[key], 0o644); err != nil {
			return "", fmt.Errorf("write embed %s: %w", key, err)
		}
		replacements = append(replacements, amd64.QuoteString(key), amd64.QuoteString(file))
	}
	return strings.NewReplacer(replacements...).Replace(source), nil
}
