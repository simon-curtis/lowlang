package lsp

import (
	"fmt"
	"slices"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/types"
)

var builtinParams = map[string][]string{
	"print": {"value"}, "new": {"value"}, "push": {"xs", "value"}, "pop": {"xs"}, "truncate": {"xs", "n"},
	"delete": {"map", "key"}, "bytes": {"s"}, "cstr": {"s"}, "str": {"value"}, "view": {"p", "n"},
	"load": {"p"}, "store": {"p", "value"}, "pin": {"value"}, "unpin": {"p"}, "embed": {"path"},
	"sqrt": {"x"}, "floor": {"x"}, "exp": {"x"}, "close": {"ch"}, "sleep": {"ms"},
	"wait_readable": {"fd"}, "wait_writable": {"fd"},
}

func builtinSignature(call *ast.Call) string {
	id, ok := call.Fn.(*ast.Ident)
	if !ok || id.Local != nil || id.Func != nil || id.Variant != nil || call.TypeOf() == nil {
		return ""
	}
	names, known := builtinParams[id.Name]
	if !known && !slices.Contains(builtinTypes, id.Name) {
		return ""
	}
	params := make([]string, len(call.Args))
	for i, arg := range call.Args {
		params[i] = builtinParam(arg, names, i)
	}
	result := ""
	if ret := call.TypeOf(); ret != types.Void {
		result = fmt.Sprintf(" -> %s", ret)
	}
	return fmt.Sprintf("fn %s(%s)%s", id.Name, strings.Join(params, ", "), result)
}

func builtinParam(arg ast.Expr, names []string, i int) string {
	name := "value"
	if i < len(names) {
		name = names[i]
	}
	if _, ok := arg.(*ast.Ref); ok {
		return fmt.Sprintf("mut %s: %s", name, arg.TypeOf())
	}
	return fmt.Sprintf("%s: %s", name, arg.TypeOf())
}
