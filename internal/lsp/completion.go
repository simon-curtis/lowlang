package lsp

import (
	"fmt"
	"math"
	"regexp"

	"lowlang/internal/ast"
	"lowlang/internal/compiler"
	"lowlang/internal/lexer"
	"lowlang/internal/regex"
	"lowlang/internal/types"
)

const (
	completionMethod     = 2
	completionFunction   = 3
	completionField      = 5
	completionVariable   = 6
	completionModule     = 9
	completionProperty   = 10
	completionEnum       = 13
	completionKeyword    = 14
	completionEnumMember = 20
	completionConstant   = 21
	completionStruct     = 22
	completionType       = 25
)

var builtinFuncs = []string{
	"print", "new", "push", "pop", "truncate", "delete", "bytes", "cstr", "str", "view", "load", "store", "pin", "unpin",
	"embed", "sqrt", "floor", "exp", "close", "sleep", "wait_readable", "wait_writable",
}

var builtinTypes = []string{"i64", "i32", "u32", "u16", "u8", "f64", "f32", "bool", "str", "ptr", "chan"}

var builtinConstants = []string{"true", "false", "none", "null"}

var funcHeader = regexp.MustCompile(`^(?:pub\s+)?fn\s+([A-Za-z_][A-Za-z0-9_]*)`)

type cursor struct {
	unit     *compiler.Unit
	file     *ast.File
	text     text
	compiled text
	line     int
	col      int
	items    []CompletionItem
	seen     map[string]bool
}

func (c *cursor) compiledLine() int {
	old, now := c.compiled.lines, c.text.lines
	prefix := 0
	for prefix < len(old) && prefix < len(now) && old[prefix] == now[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(old)-prefix && suffix < len(now)-prefix && old[len(old)-1-suffix] == now[len(now)-1-suffix] {
		suffix++
	}
	switch {
	case c.line < prefix:
		return c.line
	case c.line >= len(now)-suffix:
		return c.line - len(now) + len(old)
	}
	return prefix - 1
}

func (c *cursor) add(label string, kind int, detail string) {
	if c.seen[label] {
		return
	}
	c.seen[label] = true
	c.items = append(c.items, CompletionItem{Label: label, Kind: kind, Detail: detail})
}

func (c *cursor) complete() []CompletionItem {
	segments, member := c.receiver()
	if member {
		c.members(segments)
		return c.items
	}
	c.locals()
	c.module()
	c.imports()
	c.builtins()
	return c.items
}

func (c *cursor) receiver() ([]segment, bool) {
	src := c.text.line(c.line)
	at := min(c.col, len(src))
	for at > 0 && isWord(src[at-1]) {
		at--
	}
	if at == 0 || src[at-1] != '.' {
		return nil, false
	}
	segments, _ := segmentsBefore(src, at-1)
	return segments, true
}

func chainBefore(src string, end int) []string {
	var chain []string
	for {
		start := end
		for start > 0 && isWord(src[start-1]) {
			start--
		}
		if start == end {
			return chain
		}
		chain = append([]string{src[start:end]}, chain...)
		if start == 0 || src[start-1] != '.' {
			return chain
		}
		end = start - 1
	}
}

func (c *cursor) files(module string) []*ast.File {
	var files []*ast.File
	if c.unit == nil {
		return files
	}
	for _, f := range c.unit.Files {
		if f.Path != "" && f.Module == module {
			files = append(files, f)
		}
	}
	return files
}

func (c *cursor) members(segments []segment) {
	if c.file == nil || len(segments) == 0 {
		return
	}
	chain := make([]string, len(segments))
	for i, s := range segments {
		if !s.plain() {
			c.fields(c.typeOf(segments), nil)
			return
		}
		chain[i] = s.name
	}
	first, rest := chain[0], chain[1:]
	if local := c.local(first); local != nil {
		c.fields(local.Type, rest)
		return
	}
	if path, ok := c.file.Imported(first); ok {
		c.moduleMembers(path, rest)
		return
	}
	if len(rest) == 0 {
		c.variants(c.file.Module, first)
	}
}

func (c *cursor) moduleMembers(path string, rest []string) {
	if len(rest) == 1 {
		c.variants(path, rest[0])
		return
	}
	if len(rest) > 1 {
		return
	}
	for _, f := range c.files(path) {
		c.declarations(f, true)
	}
	if path == regex.Module {
		c.add(regex.Generator, completionFunction, fmt.Sprintf("fn %s(pattern: str) -> regex.Regex", regex.Generator))
	}
}

func (c *cursor) variants(module, name string) {
	for _, f := range c.files(module) {
		for _, d := range f.Enums {
			if d.Name != name {
				continue
			}
			for _, v := range d.Variants {
				c.add(v.Name, completionEnumMember, variantDeclText(v))
			}
		}
	}
}

func (c *cursor) fields(t *types.Type, path []string) {
	for _, name := range path {
		if t == nil {
			return
		}
		f := types.StructOf(t).Field(name)
		if f == nil {
			return
		}
		t = f.Type
	}
	if t == nil {
		return
	}
	s := types.StructOf(t)
	external := s.Module != c.file.Module
	for _, f := range s.Fields {
		if external && !f.Pub {
			continue
		}
		c.add(f.Name, completionField, f.Type.String())
	}
	switch t.Kind {
	case types.SliceKind, types.MapKind, types.StrKind:
		c.add("len", completionProperty, "i64")
	}
	c.methods(t)
}

func (c *cursor) headerLine() (int, string) {
	for line := c.line; line >= 0; line-- {
		if m := funcHeader.FindStringSubmatch(c.text.line(line)); m != nil {
			return line, m[1]
		}
	}
	return 0, ""
}

func (c *cursor) enclosing() *ast.Func {
	_, name := c.headerLine()
	if c.file == nil || name == "" {
		return nil
	}
	for _, f := range c.file.Funcs {
		if f.Name == name && f.Template == nil && f.Closure == nil {
			return f
		}
	}
	return nil
}

func (c *cursor) span(f *ast.Func) (int, int) {
	end := math.MaxInt
	var starts []lexer.Pos
	for _, g := range c.file.Funcs {
		if g.Template == nil && g.Closure == nil {
			starts = append(starts, g.Pos)
		}
	}
	for _, d := range c.file.Structs {
		starts = append(starts, d.Pos)
	}
	for _, d := range c.file.Enums {
		starts = append(starts, d.Pos)
	}
	for _, e := range c.file.Externs {
		starts = append(starts, e.Pos)
	}
	for _, p := range starts {
		if p.Line > f.Pos.Line && p.Line < end {
			end = p.Line
		}
	}
	return f.Pos.Line, end
}

func (c *cursor) scope() []*ast.Local {
	f := c.enclosing()
	if f == nil {
		return nil
	}
	from, to := c.span(f)
	to = min(to, c.compiledLine()+2)
	x := newIndex(c.file, c.compiled)
	var locals []*ast.Local
	for _, o := range x.occurrences {
		if o.local != nil && o.pos.Line >= from && o.pos.Line < to {
			locals = append(locals, o.local)
		}
	}
	return locals
}

func (c *cursor) local(name string) *ast.Local {
	var found *ast.Local
	for _, l := range c.scope() {
		if l.Name == name {
			found = l
		}
	}
	return found
}

func (c *cursor) locals() {
	locals := c.scope()
	for i := len(locals) - 1; i >= 0; i-- {
		l := locals[i]
		detail := ""
		if l.Type != nil {
			detail = l.Type.String()
		}
		c.add(l.Name, completionVariable, detail)
	}
}

func (c *cursor) module() {
	if c.file == nil {
		return
	}
	for _, f := range c.files(c.file.Module) {
		c.declarations(f, false)
	}
}

func (c *cursor) declarations(f *ast.File, public bool) {
	for _, fn := range f.Funcs {
		if fn.Template == nil && fn.Closure == nil && (fn.Pub || !public) {
			c.add(fn.Name, completionFunction, signature(fn))
		}
	}
	for _, e := range f.Externs {
		for _, fn := range e.Funcs {
			if fn.Pub || !public {
				c.add(fn.Name, completionFunction, signature(fn))
			}
		}
	}
	for _, d := range f.Structs {
		if d.Pub || !public {
			c.add(d.Name, completionStruct, "struct")
		}
	}
	for _, d := range f.Enums {
		if !d.Pub && public {
			continue
		}
		c.add(d.Name, completionEnum, "enum")
		if public {
			continue
		}
		for _, v := range d.Variants {
			detail := fmt.Sprintf("%s.%s", d.Name, variantDeclText(v))
			c.add(v.Name, completionEnumMember, detail)
		}
	}
}

func (c *cursor) imports() {
	if c.file == nil {
		return
	}
	for _, imp := range c.file.Imports {
		c.add(imp.Alias, completionModule, imp.Path)
	}
}

func (c *cursor) builtins() {
	for _, name := range builtinFuncs {
		c.add(name, completionFunction, "builtin")
	}
	for _, name := range builtinTypes {
		c.add(name, completionType, "type")
	}
	for _, name := range builtinConstants {
		c.add(name, completionConstant, "")
	}
	for _, word := range keywords() {
		c.add(word, completionKeyword, "")
	}
}
