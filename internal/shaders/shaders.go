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

const (
	SumFile   = "shaders.sum"
	TargetEnv = "vulkan1.2"
)

var stageExts = []string{".vert", ".frag"}

const (
	includeExt = ".glsl"
	binaryExt  = ".spv"
)

func Binary(stage string) string {
	return fmt.Sprintf("%s%s", stage, binaryExt)
}

func Stages(dir string) ([]string, error) {
	return list(dir, func(ext string) bool { return slices.Contains(stageExts, ext) })
}

func Sum(dir string) (string, error) {
	names, err := list(dir, func(ext string) bool {
		return slices.Contains(stageExts, ext) || ext == includeExt || ext == binaryExt
	})
	if err != nil {
		return "", err
	}
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

func list(dir string, wanted func(ext string) bool) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if wanted(filepath.Ext(e.Name())) {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	return names, nil
}
