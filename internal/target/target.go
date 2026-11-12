package target

import (
	"fmt"
	"runtime"
	"strings"
)

type OS int

const (
	Linux OS = iota
	Windows
)

type descriptor struct {
	os     OS
	name   string
	suffix string
	exe    string
}

var targets = []descriptor{
	{Linux, "linux/amd64", "_linux.low", ""},
	{Windows, "windows/amd64", "_windows.low", ".exe"},
}

func (o OS) String() string {
	return o.entry().name
}

func (o OS) entry() descriptor {
	for _, t := range targets {
		if t.os == o {
			return t
		}
	}
	return descriptor{}
}

func (o OS) Executable(name string) string {
	return fmt.Sprintf("%s%s", name, o.entry().exe)
}

func (o OS) Builds(file string) bool {
	for _, t := range targets {
		if t.os != o && strings.HasSuffix(file, t.suffix) {
			return false
		}
	}
	return true
}

func Names() []string {
	out := make([]string, len(targets))
	for i, t := range targets {
		out[i] = t.name
	}
	return out
}

func Choices() string {
	return strings.Join(Names(), "|")
}

func Parse(name string) (OS, error) {
	for _, t := range targets {
		if t.name == name {
			return t.os, nil
		}
	}
	return Linux, fmt.Errorf("unknown target %q; expected %s", name, strings.Join(Names(), " or "))
}

func Host() OS {
	if runtime.GOOS == "windows" {
		return Windows
	}
	return Linux
}
