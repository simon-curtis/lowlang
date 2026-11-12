package check

import (
	"fmt"

	"lowlang/internal/ast"
	"lowlang/internal/regex"
	"lowlang/internal/types"
)

func GeneratedRegex() (name, signature string) {
	return regex.Generator, fmt.Sprintf("fn %s(pattern: str) -> regex.Regex", regex.Generator)
}

func TakesFirst(fn *ast.Func, t *types.Type) bool {
	if len(fn.ParamTypes) == 0 {
		return false
	}
	in := &inference{params: fn.TypeArgs, bound: map[*types.Type]*types.Type{}}
	return in.unify(fn.ParamTypes[0], t)
}
