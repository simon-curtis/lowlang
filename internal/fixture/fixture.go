package fixture

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	StdoutExt       = ".out"
	StderrExt       = ".err"
	MaxRSSExt       = ".maxrss"
	MaxTimeExt      = ".maxtime"
	LibrariesExt    = ".needed"
	TagsExt         = ".tags"
	WarningsExt     = ".warn"
	CompileErrorExt = ".err"
	SourceExt       = ".low"
)

const (
	tagSIMD      = "simd"
	tagLinuxOnly = "linux-only"
)

type Sidecars struct {
	Stdout    string
	Stderr    string
	Panics    bool
	MaxRSSKB  int64
	MaxTime   time.Duration
	Libraries []string
	SIMD      bool
	LinuxOnly bool
}

func Base(path string) string {
	return strings.TrimSuffix(path, SourceExt)
}

func (s Sidecars) Windows() bool {
	return !s.LinuxOnly && len(s.Libraries) == 0
}

func Text(path, ext string) (string, bool, error) {
	data, err := os.ReadFile(Base(path) + ext)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return string(data), true, nil
}

func Read(path string) (Sidecars, error) {
	var s Sidecars
	var err error
	if s.Stdout, _, err = Text(path, StdoutExt); err != nil {
		return s, err
	}
	if s.Stderr, s.Panics, err = Text(path, StderrExt); err != nil {
		return s, err
	}
	if err := s.readMaxRSS(path); err != nil {
		return s, err
	}
	if err := s.readMaxTime(path); err != nil {
		return s, err
	}
	if err := s.readLibraries(path); err != nil {
		return s, err
	}
	if err := s.readTags(path); err != nil {
		return s, err
	}
	return s, nil
}

func (s *Sidecars) readMaxRSS(path string) error {
	text, ok, err := Text(path, MaxRSSExt)
	if err != nil || !ok {
		return err
	}
	s.MaxRSSKB, err = strconv.ParseInt(strings.TrimSpace(text), 10, 64)
	if err != nil {
		return fmt.Errorf("%s%s: %w", Base(path), MaxRSSExt, err)
	}
	return nil
}

func (s *Sidecars) readMaxTime(path string) error {
	text, ok, err := Text(path, MaxTimeExt)
	if err != nil || !ok {
		return err
	}
	s.MaxTime, err = time.ParseDuration(strings.TrimSpace(text))
	if err != nil {
		return fmt.Errorf("%s%s: %w", Base(path), MaxTimeExt, err)
	}
	return nil
}

func (s *Sidecars) readLibraries(path string) error {
	text, ok, err := Text(path, LibrariesExt)
	if err != nil || !ok {
		return err
	}
	s.Libraries = strings.Fields(text)
	return nil
}

func (s *Sidecars) readTags(path string) error {
	text, ok, err := Text(path, TagsExt)
	if err != nil || !ok {
		return err
	}
	for _, tag := range strings.Fields(text) {
		switch tag {
		case tagSIMD:
			s.SIMD = true
		case tagLinuxOnly:
			s.LinuxOnly = true
		default:
			return fmt.Errorf("%s%s: unknown tag %q", Base(path), TagsExt, tag)
		}
	}
	return nil
}

func (s Sidecars) Write(path string) error {
	base := Base(path)
	files := []struct {
		ext     string
		content string
		wanted  bool
	}{
		{StdoutExt, s.Stdout, s.Stdout != ""},
		{StderrExt, s.Stderr, s.Panics},
		{MaxRSSExt, fmt.Sprintf("%d\n", s.MaxRSSKB), s.MaxRSSKB > 0},
		{MaxTimeExt, fmt.Sprintf("%s\n", s.MaxTime), s.MaxTime > 0},
		{LibrariesExt, fmt.Sprintf("%s\n", strings.Join(s.Libraries, "\n")), len(s.Libraries) > 0},
		{TagsExt, s.tagLines(), s.SIMD || s.LinuxOnly},
	}
	for _, file := range files {
		if !file.wanted {
			continue
		}
		if err := os.WriteFile(base+file.ext, []byte(file.content), 0o644); err != nil {
			return fmt.Errorf("write fixture: %w", err)
		}
	}
	return nil
}

func (s Sidecars) tagLines() string {
	var tags []string
	if s.SIMD {
		tags = append(tags, tagSIMD)
	}
	if s.LinuxOnly {
		tags = append(tags, tagLinuxOnly)
	}
	return fmt.Sprintf("%s\n", strings.Join(tags, "\n"))
}
