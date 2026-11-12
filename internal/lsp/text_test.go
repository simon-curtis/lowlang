package lsp

import (
	"runtime"
	"testing"
)

type uriCase struct {
	path, uri string
}

func hostURICases() []uriCase {
	if runtime.GOOS == "windows" {
		return []uriCase{
			{`G:\personal\a b\main.low`, "file:///G:/personal/a%20b/main.low"},
			{`\\server\share\main.low`, "file://server/share/main.low"},
		}
	}
	return []uriCase{
		{"/home/a b/main.low", "file:///home/a%20b/main.low"},
	}
}

func TestURIRoundTrip(t *testing.T) {
	for _, c := range hostURICases() {
		if got := uriOf(c.path); got != c.uri {
			t.Errorf("uriOf(%q) = %q, want %q", c.path, got, c.uri)
		}
		if got := pathOf(c.uri); got != c.path {
			t.Errorf("pathOf(%q) = %q, want %q", c.uri, got, c.path)
		}
	}
}

func TestPathOfEditorDriveURI(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("drive letters exist only on a Windows host")
	}
	const uri = "file:///g%3A/personal/main.low"
	if got := pathOf(uri); got != `G:\personal\main.low` {
		t.Errorf("pathOf(%q) = %q", uri, got)
	}
}
