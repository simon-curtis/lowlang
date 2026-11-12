package corpus

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"lowlang/internal/fixture"
	"lowlang/internal/fuzz/model"
)

type Fixture struct {
	fixture.Sidecars
	Name   string
	Source string
}

func FromProgram(p *model.Program, outcome model.Outcome) Fixture {
	source := p.String()
	sum := sha256.Sum256([]byte(source))
	return Fixture{
		Name:   fmt.Sprintf("fuzz_%x", sum[:4]),
		Source: source,
		Sidecars: fixture.Sidecars{
			Stdout: outcome.Stdout,
			Stderr: outcome.Stderr(),
			Panics: outcome.Panic != "",
		},
	}
}

func (f Fixture) Write(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create fixture directory: %w", err)
	}
	path := filepath.Join(dir, f.Name+fixture.SourceExt)
	if err := os.WriteFile(path, []byte(f.Source), 0o644); err != nil {
		return "", fmt.Errorf("write fixture: %w", err)
	}
	if err := f.Sidecars.Write(path); err != nil {
		return "", err
	}
	return path, nil
}
