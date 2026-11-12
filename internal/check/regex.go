package check

import (
	"fmt"
	"hash/fnv"

	"lowlang/internal/ast"
	"lowlang/internal/diag"
	"lowlang/internal/parser"
	"lowlang/internal/regex"
)

func (c *Checker) generatesRegex(alias, name string) bool {
	path, _ := c.file.Imported(alias)
	return path == regex.Module && name == regex.Generator
}

func (c *Checker) generatedRegex(e *ast.Call) {
	if len(e.Args) != 1 {
		c.fail(e.Pos, "regex.generated takes 1 argument, found %d", len(e.Args))
	}
	lit, ok := e.Args[0].(*ast.StrLit)
	if !ok {
		c.fail(e.Args[0].Position(), "regex.generated needs a string literal pattern")
	}
	key := fmt.Sprintf("regex:%s", lit.Value)
	fn, ok := c.derived[key]
	if !ok {
		fn = c.generateRegex(lit)
		c.derived[key] = fn
	}
	e.Args = nil
	e.Target = fn
	e.Type = fn.RetType
}

func (c *Checker) generateRegex(lit *ast.StrLit) *ast.Func {
	prog, err := regex.Compile(lit.Value)
	if err != nil {
		c.fail(lit.Pos, "invalid regex %v", err)
	}
	module := c.scope(regex.Module)
	name := regexName(lit.Value, module)
	source := regex.Generate(name, lit.Value, prog)
	file, err := parser.ParseSource([]byte(source), diag.Start(fmt.Sprintf("<%s>", name)), true)
	if err != nil {
		panic(fmt.Sprintf("generated regex %q does not parse: %v\n%s", lit.Value, err, source))
	}
	home := module.funcs["compile"].File
	for _, fn := range file.Funcs {
		fn.File = home
		module.funcs[fn.Name] = fn
	}
	c.within(home, nil, func() {
		for _, fn := range file.Funcs {
			c.signature(fn)
		}
	})
	home.Funcs = append(home.Funcs, file.Funcs...)
	c.pending = append(c.pending, file.Funcs...)
	return module.funcs[name]
}

func regexName(pattern string, module *scope) string {
	digest := fnv.New64a()
	digest.Write([]byte(pattern))
	base := fmt.Sprintf("__regex_%016x", digest.Sum64())
	name := base
	for n := 1; module.funcs[name] != nil; n++ {
		name = fmt.Sprintf("%s_%d", base, n)
	}
	return name
}
