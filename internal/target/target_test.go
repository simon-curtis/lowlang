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

func TestChoicesListEveryName(t *testing.T) {
	choices := strings.Split(Choices(), "|")
	if len(choices) != len(names) {
		t.Fatalf("choices %v, want %d names", choices, len(names))
	}
	for _, name := range choices {
		if _, err := Parse(name); err != nil {
			t.Error(err)
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
