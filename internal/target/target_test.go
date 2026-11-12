package target

import (
	"strings"
	"testing"
)

func TestEveryNameParsesBack(t *testing.T) {
	for _, name := range Names() {
		parsed, err := Parse(name)
		if err != nil {
			t.Fatal(err)
		}
		if parsed.String() != name {
			t.Errorf("%s parsed to %s", name, parsed)
		}
	}
}

func TestParseErrorListsTheNames(t *testing.T) {
	_, err := Parse("plan9/amd64")
	if err == nil {
		t.Fatal("parsed an unknown target")
	}
	for _, name := range Names() {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
	}
}

func TestExecutableSuffix(t *testing.T) {
	if got := Linux.Executable("app"); got != "app" {
		t.Errorf("linux executable %q", got)
	}
	if got := Windows.Executable("app"); got != "app.exe" {
		t.Errorf("windows executable %q", got)
	}
}

func TestBuildsFilesByTargetSuffix(t *testing.T) {
	cases := []struct {
		os   OS
		file string
		want bool
	}{
		{Linux, "main.low", true},
		{Linux, "surface_linux.low", true},
		{Linux, "surface_windows.low", false},
		{Windows, "surface_windows.low", true},
		{Windows, "dir/surface_linux.low", false},
		{Windows, "linux.low", true},
	}
	for _, c := range cases {
		if got := c.os.Builds(c.file); got != c.want {
			t.Errorf("%s builds %s = %v, want %v", c.os, c.file, got, c.want)
		}
	}
}

func TestAllListsEveryTarget(t *testing.T) {
	got := All()
	if len(got) != len(Names()) {
		t.Fatalf("All() = %v, want %d targets", got, len(Names()))
	}
	for i, o := range got {
		if o.String() != Names()[i] {
			t.Errorf("All()[%d] = %s, want %s", i, o, Names()[i])
		}
	}
	got[0] = Windows
	if All()[0] != Linux {
		t.Error("All() returns the shared slice")
	}
}
