package uitest

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"lowlang/internal/target"
)

const noDisplayText = "no display"

var winePrefix sync.Once

func wineEnvironment() []string {
	return append(os.Environ(), "WINEDEBUG=-all")
}

func initialiseWine(t *testing.T, wine string) {
	t.Helper()
	winePrefix.Do(func() {
		boot := exec.Command(wine, "wineboot", "--init")
		boot.Env = wineEnvironment()
		if out, err := boot.CombinedOutput(); err != nil {
			t.Fatalf("initialise the wine prefix: %v\n%s", err, out)
		}
	})
}

func runWin32(t *testing.T, name string) []string {
	t.Helper()
	var wine string
	if target.Host() != target.Windows {
		path, err := exec.LookPath("wine")
		if err != nil {
			t.Skip("wine is not installed")
		}
		wine = path
		initialiseWine(t, wine)
	}
	binary := filepath.Join(t.TempDir(), name+".exe")
	build(t, name, target.Windows, binary)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary)
	if wine != "" {
		cmd = exec.CommandContext(ctx, wine, binary)
		cmd.Env = wineEnvironment()
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%v\n%s\n%s", err, stdout.String(), stderr.String())
	}
	text := stdout.String()
	if strings.HasPrefix(text, noDisplayText) {
		t.Skip("wine cannot create a window here")
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

func compareLines(t *testing.T, got, want []string) {
	t.Helper()
	for i := 0; i < len(got) || i < len(want); i++ {
		var g, w string
		if i < len(got) {
			g = got[i]
		}
		if i < len(want) {
			w = want[i]
		}
		if g != w {
			t.Errorf("line %d = %q, want %q", i+1, g, w)
		}
	}
}

func repeated(times int, lines ...string) []string {
	var out []string
	for i := 0; i < times; i++ {
		out = append(out, lines...)
	}
	return out
}

func TestWin32(t *testing.T) {
	cases := []struct {
		name string
		want []string
	}{
		{"win32_open", repeated(2,
			"native true true",
			"resized 320 200",
			"scale true",
			"client true",
			"no events",
		)},
		{"win32_pair", repeated(2,
			"same window false",
			"scroll 0 -48",
			"close",
			"no events",
			"close",
		)},
		{"win32_mouse", []string{
			"move 10 20",
			"move 11 -3",
			"leave",
			"move 10 20",
			"down left 4 5",
			"up left 4 5",
			"down right 6 7",
			"up right 6 7",
			"down middle 8 9",
			"up middle 8 9",
			"down other4 1 2",
			"up other4 1 2",
			"down other5 3 4",
			"up other5 3 4",
			"scroll 0 -48",
			"scroll 0 96",
			"scroll 24 0",
			"scroll -48 0",
			"down left 10 20",
			"down right 10 20",
			"up left 10 20",
			"up right 10 20",
		}},
		{"win32_keys", []string{
			"key press Enter text=",
			"key release Enter text=",
			"key press Escape text=",
			"key release Escape text=",
			"key press Tab text=",
			"key release Tab text=",
			"key press Backspace text=",
			"key release Backspace text=",
			"key press Delete text=",
			"key release Delete text=",
			"key press Left text=",
			"key release Left text=",
			"key press Right text=",
			"key release Right text=",
			"key press Up text=",
			"key release Up text=",
			"key press Down text=",
			"key release Down text=",
			"key press Home text=",
			"key release Home text=",
			"key press End text=",
			"key release End text=",
			"key press PageUp text=",
			"key release PageUp text=",
			"key press PageDown text=",
			"key release PageDown text=",
			"key press F1 text=",
			"key release F1 text=",
			"key press F12 text=",
			"key release F12 text=",
			"key press 'a' text=a",
			"key press 'a' text=a repeat",
			"key release 'a' text=",
			"key press ' ' text= ",
			"key release ' ' text=",
			"key press 'A' text=A shift",
			"key release 'A' text= shift",
			"key press 'a' text= control",
			"key release 'a' text= control",
			"key press Enter text= control",
			"key press Left text= shift control alt",
			"key press 'x' text=x alt",
			"key release 'x' text= alt",
			"key press 'a' text=\U0001F600",
			"key release 'a' text=",
			"key press 'a' text=é",
			"key release 'a' text=",
		}},
		{"win32_window", []string{
			"focus true",
			"focus false",
			"close",
			"move 5 6",
			"resized 400 300",
			"resized 401 301",
			"resized scale 2 sized true",
			"resized scale 1 sized true",
			"resized scale 1 sized true",
			"resized scale 2 sized true",
		}},
		{"win32_wake", []string{
			"no events",
			"woken",
			"woken",
			"woken",
			"woken",
			"move 3 4",
			"no events",
			"woken 3",
			"no events",
		}},
	}
	for _, c := range cases {
		t.Run(strings.TrimPrefix(c.name, "win32_"), func(t *testing.T) {
			compareLines(t, runWin32(t, c.name), c.want)
		})
	}
}
