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

var all = []OS{Linux, Windows}

var names = map[OS]string{
	Linux:   "linux/amd64",
	Windows: "windows/amd64",
}

func (o OS) String() string {
	return names[o]
}

func (o OS) Executable(name string) string {
	if o == Windows {
		return fmt.Sprintf("%s.exe", name)
	}
	return name
}

func Names() []string {
	out := make([]string, len(all))
	for i, o := range all {
		out[i] = o.String()
	}
	return out
}

func Choices() string {
	return strings.Join(Names(), "|")
}

func Parse(name string) (OS, error) {
	for _, o := range all {
		if o.String() == name {
			return o, nil
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
