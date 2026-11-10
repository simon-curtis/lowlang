package target

import (
	"fmt"
	"runtime"
)

type OS int

const (
	Linux OS = iota
	Windows
)

var names = map[OS]string{
	Linux:   "linux/amd64",
	Windows: "windows/amd64",
}

func (o OS) String() string {
	return names[o]
}

func Parse(name string) (OS, error) {
	for o, n := range names {
		if n == name {
			return o, nil
		}
	}
	return Linux, fmt.Errorf("unknown target %q; expected linux/amd64 or windows/amd64", name)
}

func Host() OS {
	if runtime.GOOS == "windows" {
		return Windows
	}
	return Linux
}
