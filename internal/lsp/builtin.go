package lsp

import (
	"fmt"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/check"
	"lowlang/internal/types"
)

func builtinSignature(call *ast.Call) string {
	id, ok := call.Fn.(*ast.Ident)
	if !ok || id.Local != nil || id.Func != nil || id.Variant != nil || call.TypeOf() == nil {
		return ""
	}
	b := check.LookupBuiltin(id.Name)
	if b == nil || (b.Kind != check.FunctionBuiltin && b.Kind != check.TypeBuiltin) {
		return ""
	}
	params := make([]string, len(call.Args))
	for i, arg := range call.Args {
		params[i] = builtinParam(arg, b.Params, i)
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
