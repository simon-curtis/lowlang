package corpus

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"lowlang/internal/fuzz/model"
)

type Fixture struct {
	Name   string
	Source string
	Stdout string
	Stderr string
}

func FromProgram(p *model.Program, outcome model.Outcome) Fixture {
	source := p.String()
	sum := sha256.Sum256([]byte(source))
	return Fixture{
		Name:   fmt.Sprintf("fuzz_%x", sum[:4]),
		Source: source,
		Stdout: outcome.Stdout,
		Stderr: outcome.Stderr(),
	}
}

func (f Fixture) Write(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create fixture directory: %w", err)
	}
	files := []struct {
		ext     string
		content string
		always  bool
	}{
		{".low", f.Source, true},
		{".out", f.Stdout, false},
		{".err", f.Stderr, false},
	}
	for _, file := range files {
		if file.content == "" && !file.always {
			continue
		}
		path := filepath.Join(dir, f.Name+file.ext)
		if err := os.WriteFile(path, []byte(file.content), 0o644); err != nil {
			return "", fmt.Errorf("write fixture: %w", err)
		}
	}
	return filepath.Join(dir, f.Name+".low"), nil
}
