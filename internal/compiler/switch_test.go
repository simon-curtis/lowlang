package compiler

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

func switchProgram(keys []int, extra string) string {
	var b strings.Builder
	b.WriteString("fn pick(n: i64) -> i64 {\n    match n {\n")
	for i, k := range keys {
		fmt.Fprintf(&b, "        %d => %d,\n", k, i)
	}
	b.WriteString(extra)
	b.WriteString("        _ => -1\n    }\n}\n\nfn main() {\n    print(pick(1))\n}\n")
	return b.String()
}

func countOps(text, op string) int {
	return len(regexp.MustCompile(fmt.Sprintf(` = %s[: ]`, op)).FindAllString(text, -1))
}

func countTables(text string) int {
	return strings.Count(text, "  switch ")
}

func sequence(first, count, step int) []int {
	out := make([]int, count)
	for i := range out {
		out[i] = first + i*step
	}
	return out
}

func TestSwitchLowering(t *testing.T) {
	cases := []struct {
		name   string
		keys   []int
		extra  string
		tables int
		lt     int
		eq     int
	}{
		{"eleven dense keys stay linear", sequence(1, 11, 1), "", 0, 0, 11},
		{"twelve dense keys use a table", sequence(1, 12, 1), "", 1, 0, 0},
		{"shuffled keys use a table", []int{5, 1, 9, 3, 7, 2, 8, 4, 6, 0, 11, 10}, "", 1, 0, 0},
		{"gaps inside the span use a table", sequence(0, 12, 2), "", 1, 0, 0},
		{"negative keys use a table", sequence(-6, 12, 1), "", 1, 0, 0},
		{"forty-seven sparse keys stay linear", sequence(0, 47, 100), "", 0, 0, 47},
		{"forty-eight sparse keys use a tree", sequence(0, 48, 100), "", 0, 15, 48},
		{"duplicate key stays linear", append(sequence(1, 12, 1), 2), "", 0, 0, 13},
		{"test arm stays linear", sequence(1, 12, 1), "        % 10 == 0 => 99,\n", 0, 0, 13},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prog := devProgram(t, switchProgram(tc.keys, tc.extra))
			text := function(t, prog, "pick").String()
			if got := countTables(text); got != tc.tables {
				t.Errorf("tables %d, want %d\n%s", got, tc.tables, text)
			}
			if got := countOps(text, "lt"); got != tc.lt {
				t.Errorf("lt compares %d, want %d", got, tc.lt)
			}
			if got := countOps(text, "eq"); got != tc.eq {
				t.Errorf("eq compares %d, want %d", got, tc.eq)
			}
		})
	}
}

func TestSwitchTablesEnumTags(t *testing.T) {
	var src strings.Builder
	src.WriteString("enum Op {\n")
	for i := range 13 {
		fmt.Fprintf(&src, "    V%d\n", i)
	}
	src.WriteString("}\n\nfn pick(op: Op) -> i64 {\n    match op {\n")
	for i := range 13 {
		fmt.Fprintf(&src, "        V%d => %d\n", i, i)
	}
	src.WriteString("    }\n}\n\nfn main() {\n    print(pick(V3))\n}\n")
	prog := devProgram(t, src.String())
	text := function(t, prog, "pick").String()
	if got := countTables(text); got != 1 {
		t.Errorf("tables %d, want 1\n%s", got, text)
	}
}

func indexProgram(indexes []int) string {
	var src strings.Builder
	src.WriteString("enum Op {\n")
	for i, index := range indexes {
		fmt.Fprintf(&src, "    V%d = %d,\n", i, index)
	}
	src.WriteString("}\n\nfn pick(op: Op) -> i64 {\n    op.index\n}\n\nfn main() {\n    print(pick(V0))\n}\n")
	return src.String()
}

func TestEnumIndexLowering(t *testing.T) {
	cases := []struct {
		name    string
		indexes []int
		tables  int
		eq      int
		add     int
	}{
		{"indexes equal to tags read the tag", sequence(0, 5, 1), 0, 0, 0},
		{"indexes offset from tags add a constant", sequence(10, 5, 1), 0, 0, 1},
		{"shared indexes default to the largest group", []int{5, 9, 5, 5}, 0, 1, 0},
		{"many distinct indexes use a table", append(sequence(0, 16, 1), -1, -1), 1, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			prog := devProgram(t, indexProgram(tc.indexes))
			text := function(t, prog, "pick").String()
			if got := countTables(text); got != tc.tables {
				t.Errorf("tables %d, want %d\n%s", got, tc.tables, text)
			}
			if got := countOps(text, "eq"); got != tc.eq {
				t.Errorf("eq compares %d, want %d\n%s", got, tc.eq, text)
			}
			if got := countOps(text, "add"); got != tc.add {
				t.Errorf("adds %d, want %d\n%s", got, tc.add, text)
			}
		})
	}
}

func TestSwitchFoldsConstantSubject(t *testing.T) {
	prog := releaseProgram(t, switchProgram(sequence(1, 12, 1), ""))
	text := function(t, prog, "main").String()
	if got := countTables(text); got != 0 {
		t.Errorf("tables in main %d, want 0\n%s", got, text)
	}
}
