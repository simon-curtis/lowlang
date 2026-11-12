package hot

import (
	"fmt"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/ir"
	"lowlang/internal/types"
)

func layout(files []*ast.File) string {
	var b strings.Builder
	for _, file := range files {
		for _, d := range file.Enums {
			describeEnum(&b, d.Type)
		}
		for _, d := range file.Structs {
			describeStruct(&b, d.Type)
		}
	}
	return b.String()
}

func describeEnum(b *strings.Builder, t *types.Type) {
	fmt.Fprintf(b, "enum %s %d", t.Name, t.Size())
	for _, v := range t.Variants {
		fmt.Fprintf(b, " %s=%d", v.Name, v.Tag)
		for i, f := range v.Fields {
			fmt.Fprintf(b, " %s@%d", f, offsetOf(v, i))
		}
	}
	b.WriteString(";")
}

func offsetOf(v *types.Variant, field int) int {
	if field < len(v.Offsets) {
		return v.Offsets[field]
	}
	return -1
}

func describeStruct(b *strings.Builder, t *types.Type) {
	fmt.Fprintf(b, "struct %s %d", t.Name, t.Size())
	for _, f := range t.Fields {
		fmt.Fprintf(b, " %s:%s@%d", f.Name, f.Type, f.Bytes)
	}
	b.WriteString(";")
}

func signatures(prog *ir.Program) map[string]string {
	out := make(map[string]string, len(prog.Funcs))
	for _, f := range prog.Funcs {
		var b strings.Builder
		for _, p := range f.Params {
			fmt.Fprintf(&b, "%t %s,", p.Mut, p.Type)
		}
		fmt.Fprintf(&b, "-> %s", f.Ret)
		for _, c := range f.Captures {
			fmt.Fprintf(&b, " %s %t", c.Type, c.Boxed)
		}
		out[f.Name] = b.String()
	}
	return out
}
