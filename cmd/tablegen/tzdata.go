package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

const (
	tzdataSource = "tzdata.zi"
	versionTag   = "# version "
)

func generateTzdata(source, output string) error {
	input := filepath.Join(source, tzdataSource)
	version, err := tzdataVersion(input)
	if err != nil {
		return err
	}
	compiled, err := os.MkdirTemp("", "tzdata")
	if err != nil {
		return err
	}
	defer os.RemoveAll(compiled)
	if out, err := exec.Command("zic", "-b", "slim", "-d", compiled, input).CombinedOutput(); err != nil {
		return fmt.Errorf("zic: %v\n%s", err, out)
	}
	files, err := readZones(compiled)
	if err != nil {
		return err
	}
	index, data := packZones(files)
	return writeSections(output, []section{
		{"index.txt", index},
		{"zones.bin", data},
		{"version.txt", []byte(version + "\n")},
	})
}

func tzdataVersion(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if version, ok := strings.CutPrefix(scanner.Text(), versionTag); ok {
			return strings.TrimSpace(version), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%s has no version line", path)
}

func readZones(root string) (map[string][]byte, error) {
	files := map[string][]byte{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.HasPrefix(data, []byte("TZif")) {
			return nil
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(name)] = data
		return nil
	})
	return files, err
}

func packZones(files map[string][]byte) (index, data []byte) {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	type span struct{ offset, length int }
	placed := map[string]span{}
	index = append(index, '\n')
	for _, name := range names {
		body := files[name]
		at, seen := placed[string(body)]
		if !seen {
			at = span{len(data), len(body)}
			placed[string(body)] = at
			data = append(data, body...)
		}
		index = fmt.Appendf(index, "%s %d %d\n", name, at.offset, at.length)
	}
	return index, data
}
