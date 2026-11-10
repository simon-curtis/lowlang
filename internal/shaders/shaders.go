package shaders

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const SumFile = "sources.sum"

func Stages(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var stages []string
	for _, e := range entries {
		ext := filepath.Ext(e.Name())
		if ext == ".vert" || ext == ".frag" {
			stages = append(stages, e.Name())
		}
	}
	slices.Sort(stages)
	return stages, nil
}

func Sum(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var names []string
	for _, e := range entries {
		switch filepath.Ext(e.Name()) {
		case ".vert", ".frag", ".glsl":
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	var lines strings.Builder
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}
		digest := sha256.Sum256(data)
		fmt.Fprintf(&lines, "%s  %s\n", hex.EncodeToString(digest[:]), name)
	}
	return lines.String(), nil
}
