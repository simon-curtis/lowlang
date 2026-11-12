package uitest

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lowlang/internal/compiler"
	"lowlang/internal/fixture/fixturetest"
	"lowlang/internal/target"
)

const noDevice = "no device"

type frame struct {
	width  int
	height int
	pixels []byte
}

type output struct {
	frames map[string]frame
	lines  []string
}

type pixel [4]int

func programs(t *testing.T) []string {
	t.Helper()
	mains, err := filepath.Glob(filepath.Join("testdata", "*", "main.low"))
	if err != nil {
		t.Fatal(err)
	}
	if len(mains) == 0 {
		t.Fatal("no programs in testdata")
	}
	names := make([]string, len(mains))
	for i, main := range mains {
		names[i] = filepath.Base(filepath.Dir(main))
	}
	return names
}

func build(t *testing.T, name string, platform target.OS, binary string) {
	t.Helper()
	unit, err := compiler.Analyse(filepath.Join("testdata", name), compiler.Release(platform))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := compiler.Build(context.Background(), unit, binary); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, name string) output {
	t.Helper()
	fixturetest.RequireLinuxHost(t)
	fixturetest.RequireLibrary(t, "vulkan")
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
	if strings.HasPrefix(text, noDevice) {
		t.Skip("no Vulkan 1.3 device")
	}
	return parse(t, text)
}

func parse(t *testing.T, text string) output {
	t.Helper()
	out := output{frames: map[string]frame{}}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		var name string
		var width, height int
		if _, err := fmt.Sscanf(lines[i], "image %s %d %d", &name, &width, &height); err != nil {
			out.lines = append(out.lines, lines[i])
			continue
		}
		if i+height >= len(lines) {
			t.Fatalf("image %s is cut short", name)
		}
		var pixels []byte
		for _, row := range lines[i+1 : i+1+height] {
			bytes, err := hex.DecodeString(strings.TrimPrefix(row, "0x"))
			if err != nil || len(bytes) != width*4 {
				t.Fatalf("image %s: bad row %q", name, row)
			}
			pixels = append(pixels, bytes...)
		}
		out.frames[name] = frame{width: width, height: height, pixels: pixels}
		i += height
	}
	return out
}

func (o output) frame(t *testing.T, name string) frame {
	t.Helper()
	f, ok := o.frames[name]
	if !ok {
		t.Fatalf("the program printed no image %q", name)
	}
	return f
}

func (f frame) at(x, y int) pixel {
	if x < 0 || y < 0 || x >= f.width || y >= f.height {
		panic(fmt.Sprintf("pixel %d,%d outside %dx%d", x, y, f.width, f.height))
	}
	i := (y*f.width + x) * 4
	return pixel{int(f.pixels[i]), int(f.pixels[i+1]), int(f.pixels[i+2]), int(f.pixels[i+3])}
}

func (f frame) expect(t *testing.T, x, y int, want pixel, tolerance int) {
	t.Helper()
	got := f.at(x, y)
	for c := range got {
		if abs(got[c]-want[c]) > tolerance {
			t.Errorf("pixel %d,%d = %v, want %v within %d", x, y, got, want, tolerance)
			return
		}
	}
}

func (f frame) expectAll(t *testing.T, want pixel, tolerance int, points ...[2]int) {
	t.Helper()
	for _, p := range points {
		f.expect(t, p[0], p[1], want, tolerance)
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func TestQuads(t *testing.T) {
	out := run(t, "quad")

	plain := out.frame(t, "plain")
	clear := pixel{0, 0, 0, 0}
	red := pixel{255, 0, 0, 255}
	green := pixel{0, 255, 0, 255}
	plain.expectAll(t, red, 0, [2]int{18, 18}, [2]int{8, 8}, [2]int{27, 27}, [2]int{8, 27}, [2]int{27, 8})
	plain.expectAll(t, clear, 0, [2]int{4, 4}, [2]int{7, 7}, [2]int{28, 28}, [2]int{7, 18}, [2]int{28, 18}, [2]int{18, 28})
	plain.expect(t, 32, 8, green, 0)
	plain.expect(t, 40, 8, green, 0)
	plain.expect(t, 44, 20, green, 0)
	plain.expect(t, 55, 31, green, 0)
	plain.expect(t, 55, 8, clear, 0)
	plain.expect(t, 32, 31, clear, 0)
	plain.expect(t, 52, 12, green, 0)
	plain.expect(t, 53, 10, clear, 0)
	plain.expect(t, 33, 30, clear, 0)

	bordered := out.frame(t, "bordered")
	background := pixel{0x10, 0x20, 0x30, 255}
	blue := pixel{0, 0, 255, 255}
	yellow := pixel{255, 255, 0, 255}
	bordered.expectAll(t, background, 0, [2]int{4, 4}, [2]int{7, 28}, [2]int{56, 28}, [2]int{30, 7}, [2]int{30, 48})
	bordered.expectAll(t, yellow, 0, [2]int{30, 28}, [2]int{16, 28}, [2]int{51, 28}, [2]int{30, 10}, [2]int{30, 41})
	bordered.expectAll(t, blue, 0,
		[2]int{8, 28}, [2]int{15, 28},
		[2]int{30, 8}, [2]int{30, 9},
		[2]int{52, 28}, [2]int{55, 28},
		[2]int{30, 42}, [2]int{30, 47})

	rounded := out.frame(t, "rounded")
	black := pixel{0, 0, 0, 255}
	white := pixel{255, 255, 255, 255}
	rounded.expectAll(t, white, 0, [2]int{8, 8}, [2]int{55, 8}, [2]int{8, 55}, [2]int{55, 55}, [2]int{2, 2})
	rounded.expectAll(t, green, 0, [2]int{32, 32}, [2]int{20, 20})
	rounded.expectAll(t, black, 0, [2]int{32, 8}, [2]int{32, 11}, [2]int{8, 32}, [2]int{55, 32}, [2]int{32, 55})
}

func TestShadow(t *testing.T) {
	out := run(t, "shadow")
	f := out.frame(t, "shadow")

	f.expect(t, 48, 32, pixel{0, 0, 0, 255}, 12)
	if v := f.at(31, 32)[0]; v < 100 || v > 160 {
		t.Errorf("the shadow at the rectangle's edge is %d, want about 128", v)
	}
	row := 32
	previous := -1
	for x := 31; x >= 18; x-- {
		v := f.at(x, row)[0]
		if v < previous {
			t.Errorf("the shadow brightens to %d at x=%d after %d, so it is not monotonic", v, x, previous)
		}
		previous = v
	}
	if v := f.at(20, row)[0]; v < 250 {
		t.Errorf("three sigma out the shadow is %d, want at least 250", v)
	}
	if v := f.at(28, row)[0]; v < 190 || v > 230 {
		t.Errorf("one sigma out the shadow is %d, want about 205", v)
	}
	f.expect(t, 10, row, pixel{255, 255, 255, 255}, 0)
	f.expect(t, 85, row, pixel{255, 255, 255, 255}, 0)
	f.expect(t, 48, 2, pixel{255, 255, 255, 255}, 0)

	for k := 0; k < 12; k++ {
		left, right := f.at(31-k, 32), f.at(64+k, 32)
		top, bottom := f.at(48, 15-k), f.at(48, 48+k)
		if abs(left[0]-right[0]) > 2 {
			t.Errorf("shadow is not symmetric in x at %d: %v against %v", k, left, right)
		}
		if abs(top[0]-bottom[0]) > 2 {
			t.Errorf("shadow is not symmetric in y at %d: %v against %v", k, top, bottom)
		}
		if abs(left[0]-top[0]) > 2 {
			t.Errorf("shadow of a square differs between x and y at %d: %v against %v", k, left, top)
		}
	}

	r := out.frame(t, "rounded")
	if inside, corner := r.at(48, 32)[0], r.at(32, 16)[0]; corner < inside+100 {
		t.Errorf("the rounded shadow's corner %d is not much lighter than its middle %d", corner, inside)
	}
}

func TestSprites(t *testing.T) {
	out := run(t, "sprite")
	f := out.frame(t, "sprites")

	f.expect(t, 4, 4, pixel{255, 0, 0, 255}, 1)
	f.expect(t, 5, 4, pixel{255, 0, 0, 255}, 1)
	f.expect(t, 6, 4, pixel{128, 0, 0, 255}, 2)
	f.expect(t, 7, 4, pixel{128, 0, 0, 255}, 2)
	f.expect(t, 8, 4, pixel{0, 0, 0, 255}, 0)
	f.expect(t, 9, 4, pixel{0, 0, 0, 255}, 0)
	f.expect(t, 10, 4, pixel{64, 0, 0, 255}, 2)
	f.expect(t, 11, 4, pixel{64, 0, 0, 255}, 2)
	f.expect(t, 4, 5, pixel{255, 0, 0, 255}, 1)
	f.expect(t, 12, 4, pixel{0, 0, 0, 255}, 0)
	f.expect(t, 4, 6, pixel{0, 0, 0, 255}, 0)

	f.expect(t, 4, 10, pixel{0, 128, 0, 255}, 2)
	f.expect(t, 6, 10, pixel{0, 64, 0, 255}, 2)
	f.expect(t, 8, 10, pixel{0, 0, 0, 255}, 0)
	f.expect(t, 10, 10, pixel{0, 32, 0, 255}, 2)

	for y := 16; y < 20; y++ {
		f.expect(t, 4, y, pixel{0, 0, 255, 255}, 1)
		f.expect(t, 7, y, pixel{0, 0, 255, 255}, 1)
		f.expect(t, 8, y, pixel{0, 0, 128, 255}, 2)
		f.expect(t, 11, y, pixel{0, 0, 128, 255}, 2)
		f.expect(t, 12, y, pixel{0, 0, 0, 255}, 0)
		f.expect(t, 15, y, pixel{0, 0, 0, 255}, 0)
		f.expect(t, 16, y, pixel{0, 0, 64, 255}, 2)
		f.expect(t, 19, y, pixel{0, 0, 64, 255}, 2)
	}
	f.expect(t, 20, 16, pixel{0, 0, 0, 255}, 0)
	f.expect(t, 4, 20, pixel{0, 0, 0, 255}, 0)
}

func TestClip(t *testing.T) {
	out := run(t, "clip")
	f := out.frame(t, "clip")
	black := pixel{0, 0, 0, 255}
	red := pixel{255, 0, 0, 255}
	blue := pixel{0, 0, 255, 255}

	f.expectAll(t, red, 0, [2]int{16, 8}, [2]int{31, 8}, [2]int{31, 23}, [2]int{16, 23})
	f.expectAll(t, black, 0, [2]int{32, 8}, [2]int{32, 23}, [2]int{47, 15}, [2]int{15, 15}, [2]int{31, 24}, [2]int{31, 7})

	f.expectAll(t, blue, 0, [2]int{8, 32}, [2]int{55, 32}, [2]int{30, 39}, [2]int{8, 39}, [2]int{55, 39})
	f.expectAll(t, black, 0, [2]int{30, 31}, [2]int{30, 40}, [2]int{30, 47}, [2]int{7, 35}, [2]int{56, 35})

	f.expectAll(t, pixel{0, 255, 0, 255}, 0, [2]int{8, 52}, [2]int{23, 59})
	f.expectAll(t, black, 0, [2]int{24, 52}, [2]int{8, 60}, [2]int{7, 52})
}

func TestDrawOrder(t *testing.T) {
	out := run(t, "order")
	f := out.frame(t, "order")
	green := pixel{0, 255, 0, 255}

	f.expect(t, 6, 6, pixel{255, 0, 0, 255}, 0)
	f.expect(t, 30, 10, green, 1)
	f.expect(t, 25, 5, pixel{0, 0, 255, 255}, 0)
	f.expect(t, 15, 15, pixel{255, 0, 0, 255}, 0)

	f.expect(t, 5, 31, pixel{255, 255, 255, 255}, 0)
	f.expect(t, 16, 38, pixel{0, 0, 0, 255}, 30)
	f.expect(t, 15, 41, green, 0)
	f.expect(t, 17, 43, green, 0)
	f.expect(t, 26, 52, pixel{255, 255, 255, 255}, 40)

	if got := out.lines; len(got) != 1 || got[0] != "batches 6" {
		t.Errorf("batches = %q, want one line batches 6", got)
	}
}

func TestFramesInFlight(t *testing.T) {
	out := run(t, "frames")
	for step := 0; step < 6; step++ {
		f := out.frame(t, fmt.Sprintf("step%d", step))
		shade := pixel{step * 30, 255 - step*30, 0, 255}
		f.expect(t, step*8+4, 8, shade, 1)
		f.expect(t, step*8, 4, shade, 1)
		f.expect(t, step*8+7, 11, shade, 1)
		for other := 0; other < 8; other++ {
			if other == step {
				continue
			}
			f.expect(t, other*8+4, 8, pixel{0, 0, 0, 255}, 0)
		}
	}

	burst := out.frame(t, "burst")
	burst.expect(t, 3*8+4, 8, pixel{90, 165, 0, 255}, 1)
	for step := 0; step < 3; step++ {
		burst.expect(t, step*8+4, 8, pixel{0, 0, 0, 255}, 0)
	}

	square := out.frame(t, "square")
	square.expect(t, 12, 12, pixel{255, 0, 255, 255}, 0)
	square.expect(t, 0, 0, pixel{128, 128, 128, 255}, 0)
	square.expect(t, 23, 23, pixel{128, 128, 128, 255}, 0)

	wide := out.frame(t, "wide")
	wide.expect(t, 7*8+4, 8, pixel{210, 45, 0, 255}, 1)
	wide.expect(t, 3*8+4, 8, pixel{0, 0, 0, 255}, 0)

	out.frame(t, "empty").expectAll(t, pixel{0x33, 0x66, 0x99, 255}, 0, [2]int{0, 0}, [2]int{12, 8}, [2]int{63, 15})

	if got := out.lines; len(got) != 1 || got[0] != "invalid 0 10" {
		t.Errorf("lines = %q, want one line invalid 0 10", got)
	}
}

func TestInstanceBufferGrowth(t *testing.T) {
	out := run(t, "growth")
	for i, count := range []int{10, 5000, 100, 5000} {
		f := out.frame(t, fmt.Sprintf("quads%d", i))
		for n := 0; n < 5000; n++ {
			x, y := n%100, n/100
			want := pixel{0, 0, 0, 255}
			if n < count {
				want = pixel{x * 2 % 256, y * 5 % 256, (x + y*3) % 256, 255}
			}
			if got := f.at(x, y); got != want {
				t.Fatalf("quads%d: pixel %d,%d = %v, want %v", i, x, y, got, want)
			}
		}
	}

	dots := out.frame(t, "sprites")
	for n := 0; n < 5000; n++ {
		x, y := n%100, n/100
		want := pixel{0, 0, 0, 255}
		if n < 1500 {
			want = pixel{x * 2 % 256, y * 5 % 256, (x + y*3) % 256, 255}
		}
		if got := dots.at(x, y); got != want {
			t.Fatalf("sprites: pixel %d,%d = %v, want %v", x, y, got, want)
		}
	}

	halos := out.frame(t, "shadows")
	for n := 0; n < 240; n++ {
		x, y := (n%20)*8+4, (n/20)*8+4
		if v := halos.at(x, y)[0]; v > 100 {
			t.Errorf("shadow %d has no body: pixel %d,%d is %d", n, x, y, v)
		}
		if v := halos.at(x-4, y-4)[0]; v < 200 {
			t.Errorf("shadow %d spills into the gap: pixel %d,%d is %d", n, x-4, y-4, v)
		}
	}
}

func TestAtlasUpdates(t *testing.T) {
	out := run(t, "atlas")
	covered := func(coverage ...int) []pixel {
		pixels := make([]pixel, len(coverage))
		for i, c := range coverage {
			pixels[i] = pixel{c, c, c, 255}
		}
		return pixels
	}
	check := func(name string, row int, x int, want []pixel) {
		f := out.frame(t, name)
		for i, p := range want {
			f.expect(t, x+i, row, p, 1)
		}
	}

	check("first", 2, 2, covered(255, 200, 100, 50))
	check("first", 6, 2, covered(0, 0, 0, 0))
	check("rewritten", 2, 2, covered(10, 20, 30, 40))
	check("rewritten", 6, 2, covered(0, 0, 0, 0))
	check("deep", 2, 2, covered(10, 20, 30, 40))
	check("deep", 6, 2, covered(60, 120, 180, 240))
	check("unchanged", 2, 2, covered(10, 20, 30, 40))
	check("unchanged", 6, 2, covered(60, 120, 180, 240))
	check("cleared", 2, 2, covered(0, 0, 0, 0))
	check("cleared", 6, 2, covered(0, 0, 0, 0))
	check("resized", 2, 2, covered(90, 91, 92, 93))
	check("resized", 6, 2, covered(0, 0, 0, 0))
	check("resized_again", 2, 2, covered(90, 91, 92, 93))

	want := []string{"dirty false", "dirty true 0 1", "dirty true 900 901"}
	if len(out.lines) != len(want) {
		t.Fatalf("lines = %q, want %q", out.lines, want)
	}
	for i := range want {
		if out.lines[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, out.lines[i], want[i])
		}
	}
}

func TestWindowsBuilds(t *testing.T) {
	for _, name := range programs(t) {
		t.Run(name, func(t *testing.T) {
			binary := filepath.Join(t.TempDir(), name+".exe")
			build(t, name, target.Windows, binary)
			data, err := os.ReadFile(binary)
			if err != nil {
				t.Fatal(err)
			}
			library := windowsLibrary(name)
			if !strings.Contains(strings.ToLower(string(data)), library) {
				t.Errorf("the Windows executable does not import %s", library)
			}
		})
	}
}

func windowsLibrary(name string) string {
	if strings.HasPrefix(name, "win32_") || name == "wayland_window" || name == "wayland_reopen" {
		return "user32.dll"
	}
	return "vulkan-1.dll"
}
