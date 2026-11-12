package wlgen

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"lowlang/internal/compiler"
	"lowlang/internal/fixture/fixturetest"
	"lowlang/internal/target"
)

func load(t *testing.T, names ...string) []*Protocol {
	t.Helper()
	var protocols []*Protocol
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join("..", "..", "ui", "protocols", name))
		if err != nil {
			t.Fatal(err)
		}
		p, err := Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		protocols = append(protocols, p)
	}
	return protocols
}

func TestSignatures(t *testing.T) {
	cases := []struct {
		msg   Message
		sig   string
		types string
	}{
		{Message{Args: []Arg{{Type: "uint"}, {Type: "new_id"}}}, "usun", "null null null null"},
		{Message{Since: 3, Args: []Arg{{Type: "object", Interface: "wl_surface", AllowNull: true}, {Type: "fixed"}}}, "3?of", "ifs.wl_surface null"},
		{Message{Args: []Arg{{Type: "new_id", Interface: "wl_callback"}, {Type: "fd"}, {Type: "array"}, {Type: "string"}}}, "nhas", "ifs.wl_callback null null null"},
	}
	for _, c := range cases {
		sig, types := signature(c.msg)
		if sig != c.sig || strings.Join(types, " ") != c.types {
			t.Errorf("signature = %q %v, want %q %q", sig, types, c.sig, c.types)
		}
	}
}

func TestRejectsUnknownReferences(t *testing.T) {
	_, err := Generate(load(t, "xdg-shell.xml"))
	if err == nil || !strings.Contains(err.Error(), "refers to interface wl_surface") {
		t.Fatalf("err = %v", err)
	}
}

func TestRejectsDuplicates(t *testing.T) {
	_, err := Generate(load(t, "wayland.xml", "wayland.xml"))
	if err == nil || !strings.Contains(err.Error(), "declared twice") {
		t.Fatalf("err = %v", err)
	}
}

func TestRegistry(t *testing.T) {
	fixturetest.RequireLibrary(t, "wayland-client")
	source, err := Generate(load(t, "wayland.xml", "xdg-shell.xml"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	program, err := os.ReadFile(filepath.Join("testdata", "registry.low"))
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"wayland.low": []byte(source), "main.low": program} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	unit, err := compiler.Analyse(dir, compiler.Release(target.Linux))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "registry")
	ctx := context.Background()
	if _, err := compiler.Build(ctx, unit, binary); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Skip("no Wayland compositor to connect to")
	}
	out, err := exec.CommandContext(ctx, binary).CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if want := "true\ntrue\ntrue\ntrue\n1\n"; string(out) != want {
		t.Fatalf("output:\n%s\nwant:\n%s", out, want)
	}
}
