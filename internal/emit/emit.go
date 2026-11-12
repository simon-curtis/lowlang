package emit

import (
	"fmt"
	"strings"
)

type Lines struct {
	out strings.Builder
}

func (l *Lines) Line(format string, args ...any) {
	fmt.Fprintf(&l.out, format, args...)
	l.out.WriteString("\n")
}

func (l *Lines) String() string {
	return l.out.String()
}
