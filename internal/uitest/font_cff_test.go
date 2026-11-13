package uitest

import (
	"context"
	"io/fs"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"lowlang/internal/compiler"
	"lowlang/internal/fixture/fixturetest"
	"lowlang/internal/target"
)

const (
	fontRoot    = "/usr/share/fonts"
	maxOpenType = 8
	unsupported = `Unsupported{table: "CFF2"}`
)

type fontReport struct {
	path    string
	em      float64
	glyphs  int
	empty   int
	letters map[string][4]float64
}

func installedFonts(t *testing.T) []string {
	t.Helper()
	var opentype, collections []string
	err := filepath.WalkDir(fontRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".otf":
			opentype = append(opentype, path)
		case ".ttc", ".otc":
			collections = append(collections, path)
		}
		return nil
	})
	if err != nil {
		t.Skipf("cannot read %s: %v", fontRoot, err)
	}
	if len(opentype) == 0 {
		t.Skipf("no .otf font under %s", fontRoot)
	}
	slices.Sort(opentype)
	slices.Sort(collections)
	if len(opentype) > maxOpenType {
		opentype = opentype[:maxOpenType]
	}
	if len(collections) > 0 {
		opentype = append(opentype, collections[0])
	}
	return opentype
}

func buildFontProgram(t *testing.T) string {
	t.Helper()
	unit, err := compiler.Analyse("fonttest", compiler.Release(target.Linux))
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "fonttest")
	if _, err := compiler.Build(context.Background(), unit, binary); err != nil {
		t.Fatal(err)
	}
	return binary
}

func readReports(t *testing.T, text string) []fontReport {
	t.Helper()
	var reports []fontReport
	lines := strings.Split(strings.TrimSpace(text), "\n")
	for i := 0; i < len(lines); i++ {
		fields := strings.Fields(lines[i])
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "failed":
			if !strings.HasSuffix(lines[i], unsupported) {
				t.Errorf("%s", lines[i])
			}
			continue
		case "unreadable":
			t.Errorf("%s", lines[i])
			continue
		case "font":
		default:
			t.Fatalf("unexpected line %q", lines[i])
		}
		if len(fields) != 10 || i+3 >= len(lines) {
			t.Fatalf("bad report %q", lines[i])
		}
		report := fontReport{path: fields[1], letters: map[string][4]float64{}}
		report.em = number(t, fields[3])
		report.glyphs = int(number(t, fields[5]))
		report.empty = int(number(t, fields[7]))
		for k, letter := range []string{"A", "o", "g"} {
			glyph := strings.Fields(lines[i+1+k])
			if len(glyph) != 8 || glyph[0] != "glyph" {
				t.Fatalf("bad glyph line %q", lines[i+1+k])
			}
			var box [4]float64
			for c := range box {
				box[c] = number(t, glyph[2+c])
			}
			report.letters[letter] = box
		}
		reports = append(reports, report)
		i += 3
	}
	return reports
}

func number(t *testing.T, text string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(text, 64)
	if err != nil {
		t.Fatalf("bad number %q", text)
	}
	return v
}

func TestInstalledOpenTypeFonts(t *testing.T) {
	fixturetest.RequireLinuxHost(t)
	paths := installedFonts(t)
	binary := buildFontProgram(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	stdout, err := exec.CommandContext(ctx, binary, paths...).Output()
	if err != nil {
		t.Fatalf("%v\n%s", err, stdout)
	}
	reports := readReports(t, string(stdout))
	if len(reports) == 0 {
		t.Skip("every installed font uses CFF2 outlines")
	}
	for _, r := range reports {
		name := filepath.Base(r.path)
		if r.glyphs == 0 || r.empty > max(8, r.glyphs/50) {
			t.Errorf("%s: %d of %d glyphs have no outline", name, r.empty, r.glyphs)
		}
		em := r.em
		for letter, wantTop := range map[string][2]float64{"A": {0.5, 1.0}, "o": {0.3, 1.0}, "g": {0.3, 1.0}} {
			box := r.letters[letter]
			x0, y0, x1, y1 := box[0], box[1], box[2], box[3]
			if x0 < -0.25*em || x1 > 1.25*em || x1-x0 < 0.2*em {
				t.Errorf("%s: %q spans x %v to %v of %v", name, letter, x0, x1, em)
			}
			if y1 < wantTop[0]*em || y1 > wantTop[1]*em {
				t.Errorf("%s: %q top %v of %v", name, letter, y1, em)
			}
			if letter == "g" && y0 > 0 {
				t.Errorf("%s: g bottom %v is above the baseline", name, y0)
			}
			if letter != "g" && (y0 < -0.1*em || y0 > 0.1*em) {
				t.Errorf("%s: %q bottom %v of %v", name, letter, y0, em)
			}
		}
	}
}
