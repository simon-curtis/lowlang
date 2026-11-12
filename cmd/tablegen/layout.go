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

func layoutSource(constants []layoutConstant, tables []string) string {
	var out strings.Builder
	for _, c := range constants {
		value := fmt.Sprintf(c.kind.format, c.value)
		fmt.Fprintf(&out, "fn %s() -> %s { %s }\n", c.name, c.kind.name, value)
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
