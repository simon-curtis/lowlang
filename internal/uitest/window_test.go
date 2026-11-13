package uitest

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"lowlang/internal/compiler"
	"lowlang/internal/fixture/fixturetest"
	"lowlang/internal/target"
)

const noDisplay = "no display"

func runWindow(t *testing.T, name string, libraries ...string) []string {
	t.Helper()
	fixturetest.RequireLinuxHost(t)
	for _, library := range append([]string{"wayland-client", "xkbcommon"}, libraries...) {
		fixturetest.RequireLibrary(t, library)
	}
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Skip("no Wayland compositor to connect to")
	}
	binary := filepath.Join(t.TempDir(), name)
	build(t, name, target.Linux, binary)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary)
	stdout, err := cmd.Output()
	if err != nil {
		var stderr []byte
		if exit, ok := err.(*exec.ExitError); ok {
			stderr = exit.Stderr
		}
		t.Fatalf("%v\n%s\n%s", err, stdout, stderr)
	}
	text := string(stdout)
	if strings.HasPrefix(text, noDisplay) {
		t.Skip("the program could not connect to a Wayland compositor")
	}
	if strings.HasPrefix(text, noDevice) {
		t.Skip("no Vulkan 1.3 device that presents to a Wayland surface")
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

func expectLines(t *testing.T, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("output:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func platformProgram(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	sources, err := filepath.Glob(filepath.Join("..", "compiler", "std", "ui", "platform", "*.low"))
	if err != nil {
		t.Fatal(err)
	}
	sources = slices.DeleteFunc(sources, func(path string) bool { return strings.HasSuffix(path, "_windows.low") })
	sources = append(sources, filepath.Join("platformtest", "main.low"))
	for _, source := range sources {
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.Base(source)), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestPlatformCallbacks(t *testing.T) {
	fixturetest.RequireLinuxHost(t)
	fixturetest.RequireLibrary(t, "wayland-client")
	fixturetest.RequireLibrary(t, "xkbcommon")
	dir := platformProgram(t)
	unit, err := compiler.Analyse(dir, compiler.Release(target.Linux))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(dir, "platformtest")
	if _, err := compiler.Build(context.Background(), unit, binary); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(),
		"XKB_DEFAULT_RULES=evdev", "XKB_DEFAULT_MODEL=pc105", "XKB_DEFAULT_LAYOUT=us", "XKB_DEFAULT_VARIANT=", "XKB_DEFAULT_OPTIONS=")
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("%v\n%s", err, stdout)
	}
	text := string(stdout)
	if strings.HasPrefix(text, "no keymap") {
		t.Skip("libxkbcommon has no default keymap")
	}
	want, err := os.ReadFile(filepath.Join("platformtest", "main.out"))
	if err != nil {
		t.Fatal(err)
	}
	if text != string(want) {
		t.Errorf("output:\n%s\nwant:\n%s", text, want)
	}
}

func TestWaylandWindow(t *testing.T) {
	got := runWindow(t, "wayland_window")
	expectLines(t, got, []string{
		"resized 320 200 true",
		"idle true true",
		"woken 1 true true",
		"pending 1 true",
		"coalesced 0",
		"idle true true",
		"closing",
	})
}

func TestWaylandClipboard(t *testing.T) {
	got := runWindow(t, "wayland_clipboard")
	expectLines(t, got, []string{`read "héllo 😀\nline"`, `kept "héllo 😀\nline"`})
}

func TestWaylandReopen(t *testing.T) {
	got := runWindow(t, "wayland_reopen")
	expectLines(t, got, []string{"cycle 0", "cycle 1", "cycle 2", "cycle 3", "cycle 4", "cycle 5"})
}

func TestWaylandPresent(t *testing.T) {
	got := runWindow(t, "wayland_present", "vulkan")
	expectLines(t, got, []string{
		"first size",
		"grown",
		"empty",
		"shrunk",
		"restored",
		"offscreen 255 0 0 255",
		"no surface",
	})
}

func TestApp(t *testing.T) {
	got := runWindow(t, "app", "vulkan")
	expectLines(t, got, []string{
		"count 1",
		`pasted "from the app"`,
		"frames true",
		"closed true",
	})
}

func TestWindows(t *testing.T) {
	got := runWindow(t, "windows", "vulkan")
	expectLines(t, got, []string{
		"left closed true count 1",
		"right closed true count 3",
	})
}

func TestCounterExample(t *testing.T) {
	fixturetest.RequireLinuxHost(t)
	for _, library := range []string{"wayland-client", "xkbcommon", "vulkan"} {
		fixturetest.RequireLibrary(t, library)
	}
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		t.Skip("no Wayland compositor to connect to")
	}
	binary := filepath.Join(t.TempDir(), "counter")
	build(t, "counter", target.Linux, binary)
	var stdout, stderr strings.Builder
	cmd := exec.Command(binary)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	select {
	case err := <-exited:
		text := stdout.String()
		if strings.HasPrefix(text, noDisplay) || strings.HasPrefix(text, noDevice) {
			t.Skip(strings.TrimSpace(text))
		}
		t.Fatalf("the example exited while its window was open: %v\n%s\n%s", err, text, stderr.String())
	case <-time.After(2 * time.Second):
		if err := cmd.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		<-exited
	}
	if stderr.Len() > 0 {
		t.Errorf("the example wrote to stderr: %s", stderr.String())
	}
}
