package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLayoutFilesMatchGenerator(t *testing.T) {
	cases := []struct {
		dir     string
		file    string
		source  string
		tables  []string
		command string
	}{
		{"unicode", normalizationLayoutFile, normalizationLayoutSource(), unicodeTables, "unicode"},
		{"unicode", propertyLayoutFile, propertyLayoutSource(), propertyTables, "properties"},
		{"html", entityLayoutFile, entityLayoutSource(), entityTables, "entities"},
	}
	for _, c := range cases {
		dir := filepath.Join("..", "..", "internal", "compiler", "std", c.dir)
		committed, err := os.ReadFile(filepath.Join(dir, c.file))
		if err != nil {
			t.Fatal(err)
		}
		if string(committed) != c.source {
			t.Errorf("std/%s/%s differs from the generator; run tablegen %s", c.dir, c.file, c.command)
		}
		for _, table := range c.tables {
			if _, err := os.Stat(filepath.Join(dir, table)); err != nil {
				t.Errorf("std/%s/%s: %v", c.dir, table, err)
			}
		}
	}
}
