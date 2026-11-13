package main

import (
	"fmt"
	"strings"
)

type lowType struct {
	name   string
	format string
}

var (
	lowInt       = lowType{"i64", "%d"}
	lowWord      = lowType{"u32", "%d"}
	lowCodePoint = lowType{"u32", "0x%X"}
)

type layoutConstant struct {
	name  string
	value int64
	kind  lowType
}

func lowConstant(name string, value int, kind lowType) layoutConstant {
	return layoutConstant{name: name, value: int64(value), kind: kind}
}

func classConstants(prefix string, names []string, kind lowType) []layoutConstant {
	constants := make([]layoutConstant, len(names))
	for i, name := range names {
		constants[i] = lowConstant(prefix+strings.ToLower(name), i, kind)
	}
	return constants
}

func layoutSource(constants []layoutConstant, tables []string) string {
	var out strings.Builder
	for _, c := range constants {
		value := fmt.Sprintf(c.kind.format, c.value)
		if c.kind == lowInt {
			fmt.Fprintf(&out, "const %s = %s\n", c.name, value)
			continue
		}
		fmt.Fprintf(&out, "const %s: %s = %s\n", c.name, c.kind.name, value)
	}
	out.WriteString("\n")
	for _, table := range tables {
		name := strings.TrimSuffix(table, ".bin")
		fmt.Fprintf(&out, "fn %s_table() -> str { embed(%q) }\n", name, table)
	}
	return out.String()
}

func namedSections(names []string, data ...[]byte) []section {
	sections := make([]section, len(names))
	for i, name := range names {
		sections[i] = section{name, data[i]}
	}
	return sections
}
