package main

import (
	"fmt"
	"io"
	"math"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/diag"
	"lowlang/internal/lexer"
)

type astDumper struct {
	out io.Writer
}

func (d astDumper) line(depth int, label, head string) {
	fmt.Fprintf(d.out, "%s%s%s\n", strings.Repeat("  ", depth), label, head)
}

func at(p diag.Pos) string {
	return fmt.Sprintf("%d:%d", p.Line, p.Col)
}

func flag(name string, on bool) string {
	if !on {
		return ""
	}
	return " " + name
}

func named(name, value string) string {
	if value == "" {
		return ""
	}
	return fmt.Sprintf(" %s=%s", name, value)
}

func quoted(name, value string) string {
	if value == "" {
		return ""
	}
	return fmt.Sprintf(" %s=%s", name, lexer.Quote(value))
}

func (d astDumper) file(f *ast.File) {
	for _, n := range f.Imports {
		d.line(0, "", fmt.Sprintf("Import %s alias=%s path=%s", at(n.Pos), n.Alias, n.Path))
	}
	for _, n := range f.Externs {
		d.line(0, "", fmt.Sprintf("Extern %s library=%s", at(n.Pos), lexer.Quote(n.Library)))
		for _, fn := range n.Funcs {
			d.function(1, fn)
		}
	}
	for _, n := range f.Enums {
		d.line(0, "", fmt.Sprintf("Enum %s name=%s%s%s%s", at(n.Pos), n.Name, flag("pub", n.Pub), quoted("doc", n.Doc), typed("type", n.Type)))
		d.typeParams(1, n.TypeParams)
		for _, v := range n.Variants {
			d.variant(1, v)
		}
		d.layout(1, n.Type)
	}
	for _, n := range f.Structs {
		d.line(0, "", fmt.Sprintf("Struct %s name=%s%s%s%s%s", at(n.Pos), n.Name, flag("pub", n.Pub), flag("c", n.C), quoted("doc", n.Doc), typed("type", n.Type)))
		d.typeParams(1, n.TypeParams)
		d.stmt(1, "drop: ", n.Drop)
		for _, field := range n.Fields {
			d.line(1, "", fmt.Sprintf("Field %s name=%s%s%s", at(field.Pos), field.Name, flag("pub", field.Pub), quoted("doc", field.Doc)))
			d.typeName(2, "type: ", field.TypeName)
			d.expr(2, "default: ", field.Default)
		}
		d.layout(1, n.Type)
	}
	for _, n := range f.Consts {
		d.line(0, "", fmt.Sprintf("Const %s name=%s%s%s%s", at(n.Pos), n.Name, flag("pub", n.Pub), quoted("doc", n.Doc), quoted("text", n.Text)))
		d.typeName(1, "type: ", n.TypeName)
		d.expr(1, "value: ", n.Value)
		d.expr(1, "folded: ", n.Folded)
	}
	for _, n := range f.Funcs {
		d.function(0, n)
	}
}

func (d astDumper) function(depth int, f *ast.Func) {
	var head strings.Builder
	fmt.Fprintf(&head, "Func %s name=%s", at(f.Pos), f.Name)
	head.WriteString(flag("pub", f.Pub))
	head.WriteString(flag("extern", f.Extern != nil))
	head.WriteString(flag("variadic", f.Variadic))
	head.WriteString(named("symbol", f.Symbol))
	head.WriteString(quoted("doc", f.Doc))
	head.WriteString(funcRef("template", f.Template))
	head.WriteString(typeList("type_args", f.TypeArgs))
	head.WriteString(flag("closure", f.Closure != nil))
	head.WriteString(flag("as_value", f.AsValue))
	head.WriteString(flag("callback", f.Callback))
	head.WriteString(typeList("param_types", f.ParamTypes))
	head.WriteString(typed("ret", f.RetType))
	head.WriteString(bound(f.Bound))
	head.WriteString(flag("trusted", f.Trusted))
	head.WriteString(flag("derived", f.Derived))
	d.line(depth, "", head.String())
	d.typeParams(depth+1, f.TypeParams)
	d.params(depth+1, f.Params)
	d.locals(depth+1, "param_local: ", f.ParamLocals)
	d.typeName(depth+1, "result: ", f.Result)
	d.typeName(depth+1, "error: ", f.Error)
	if f.Body != nil {
		d.block(depth+1, "body: ", f.Body)
	}
}

func (d astDumper) typeParams(depth int, params []*ast.TypeParam) {
	for _, p := range params {
		d.line(depth, "", fmt.Sprintf("TypeParam %s name=%s%s", at(p.Pos), p.Name, named("constraints", strings.Join(p.Constraints, "+"))))
	}
}

func (d astDumper) params(depth int, params []*ast.Param) {
	for _, p := range params {
		d.line(depth, "", fmt.Sprintf("Param %s name=%s%s", at(p.Pos), p.Name, flag("mut", p.Mut)))
		d.typeName(depth+1, "type: ", p.TypeName)
	}
}

func (d astDumper) variant(depth int, v *ast.VariantDecl) {
	labels := ""
	if len(v.Labels) > 0 {
		labels = fmt.Sprintf(" labels=%s", strings.Join(v.Labels, ","))
	}
	d.line(depth, "", fmt.Sprintf("Variant %s name=%s%s%s", at(v.Pos), v.Name, labels, quoted("doc", v.Doc)))
	for _, field := range v.Fields {
		d.typeName(depth+1, "field: ", field)
	}
	if v.Index != nil {
		d.expr(depth+1, "index: ", v.Index)
	}
}

func (d astDumper) typeName(depth int, label string, t *ast.TypeName) {
	if t == nil {
		return
	}
	var muts strings.Builder
	for _, m := range t.Muts {
		if m {
			muts.WriteByte('m')
		} else {
			muts.WriteByte('-')
		}
	}
	var head strings.Builder
	fmt.Fprintf(&head, "Type %s%s%s", at(t.Pos), named("module", t.Module), named("name", t.Name))
	head.WriteString(flag("pointer", t.Pointer))
	head.WriteString(flag("slice", t.Slice))
	head.WriteString(flag("optional", t.Optional))
	head.WriteString(flag("map", t.Map))
	head.WriteString(flag("func", t.Func))
	head.WriteString(flag("cfunc", t.CFunc))
	head.WriteString(named("muts", muts.String()))
	d.line(depth, label, head.String())
	d.typeName(depth+1, "key: ", t.Key)
	for _, arg := range t.Args {
		d.typeName(depth+1, "arg: ", arg)
	}
	for _, param := range t.Params {
		d.typeName(depth+1, "param: ", param)
	}
	d.typeName(depth+1, "result: ", t.Result)
	d.typeName(depth+1, "error: ", t.Error)
	d.typeName(depth+1, "elem: ", t.Elem)
}

func (d astDumper) block(depth int, label string, b *ast.Block) {
	d.line(depth, label, fmt.Sprintf("Block %s%s", at(b.Pos), typed("type", b.Type)))
	for _, s := range b.Stmts {
		d.stmt(depth+1, "", s)
	}
	d.expr(depth+1, "tail: ", b.Tail)
}

func (d astDumper) structPattern(depth int, label string, p *ast.StructPattern) {
	if p == nil {
		return
	}
	d.line(depth, label, fmt.Sprintf("StructPattern %s%s%s", at(p.Pos), typed("type", p.Type), local("temp", p.Temp)))
	for _, field := range p.Fields {
		d.line(depth+1, "", fmt.Sprintf("FieldPattern %s name=%s%s%s", at(field.Pos), field.Name, flag("shorthand", field.Shorthand), fieldRef("field", field.Field)))
		if field.Bind != nil {
			d.binding(depth+2, "bind: ", field.Bind)
		}
		d.structPattern(depth+2, "nested: ", field.Nested)
		d.expr(depth+2, "source: ", field.Source)
	}
}

func (d astDumper) binding(depth int, label string, b *ast.Binding) {
	d.line(depth, label, fmt.Sprintf("Binding %s name=%s%s", at(b.Pos), b.Name, local("local", b.Local)))
}

func (d astDumper) stmt(depth int, label string, s ast.Stmt) {
	switch s := s.(type) {
	case nil:
	case *ast.Let:
		d.line(depth, label, fmt.Sprintf("Let %s%s%s%s%s", at(s.Pos), named("name", s.Name), flag("mut", s.Mutable), flag("using", s.Using), local("local", s.Local)))
		d.structPattern(depth+1, "pattern: ", s.Pattern)
		d.typeName(depth+1, "type: ", s.TypeName)
		d.expr(depth+1, "value: ", s.Value)
		d.stmt(depth+1, "release: ", s.Release)
	case *ast.Assign:
		d.line(depth, label, fmt.Sprintf("Assign %s op=%s%s%s", at(s.Pos), s.Op, flag("splat", s.Splat), local("flag", s.Flag)))
		d.expr(depth+1, "target: ", s.Target)
		d.expr(depth+1, "value: ", s.Value)
		if s.Entry != nil {
			d.expr(depth+1, "entry: ", s.Entry)
		}
		d.stmt(depth+1, "release: ", s.Release)
	case *ast.ExprStmt:
		d.line(depth, label, "ExprStmt")
		d.expr(depth+1, "x: ", s.X)
	case *ast.For:
		d.line(depth, label, fmt.Sprintf("For %s%s", at(s.Pos), named("label", s.Label)))
		d.expr(depth+1, "cond: ", s.Cond)
		d.block(depth+1, "body: ", s.Body)
	case *ast.Break:
		d.line(depth, label, fmt.Sprintf("Break %s%s%s", at(s.Pos), named("label", s.Label), number("depth", int64(s.Depth))))
	case *ast.Continue:
		d.line(depth, label, fmt.Sprintf("Continue %s%s%s", at(s.Pos), named("label", s.Label), number("depth", int64(s.Depth))))
	case *ast.Return:
		d.line(depth, label, fmt.Sprintf("Return %s", at(s.Pos)))
		d.expr(depth+1, "value: ", s.Value)
	case *ast.Fail:
		d.line(depth, label, fmt.Sprintf("Fail %s", at(s.Pos)))
		d.expr(depth+1, "value: ", s.Value)
	case *ast.Defer:
		d.line(depth, label, fmt.Sprintf("Defer %s", at(s.Pos)))
		d.stmt(depth+1, "body: ", s.Body)
	case *ast.Spawn:
		d.line(depth, label, fmt.Sprintf("Spawn %s", at(s.Pos)))
		d.expr(depth+1, "call: ", s.Call)
		if s.Body != nil {
			d.block(depth+1, "body: ", s.Body)
		}
	case *ast.Send:
		d.line(depth, label, fmt.Sprintf("Send %s", at(s.Pos)))
		d.expr(depth+1, "chan: ", s.Chan)
		d.expr(depth+1, "value: ", s.Value)
	case *ast.ForIn:
		var head strings.Builder
		fmt.Fprintf(&head, "ForIn %s%s%s%s", at(s.Pos), named("label", s.Label), named("key", s.Key), named("value", s.Value))
		head.WriteString(local("key_local", s.KeyLocal))
		head.WriteString(local("value_local", s.ValueLocal))
		d.line(depth, label, head.String())
		d.structPattern(depth+1, "pattern: ", s.Pattern)
		d.expr(depth+1, "iter: ", s.Iter)
		d.expr(depth+1, "end: ", s.End)
		d.block(depth+1, "body: ", s.Body)
	default:
		panic(fmt.Sprintf("lowdump: unhandled statement %T", s))
	}
}

func (d astDumper) exprs(depth int, label string, list []ast.Expr) {
	for _, e := range list {
		d.expr(depth, label, e)
	}
}

func (d astDumper) arms(depth int, arms []*ast.Arm) {
	for _, arm := range arms {
		p := arm.Pattern
		d.line(depth, "", fmt.Sprintf("Arm %s%s%s%s", at(p.Pos), named("name", p.Name), variantRef(p.Variant), local("local", p.Local)))
		for _, b := range p.Bindings {
			d.binding(depth+1, "", b)
		}
		d.expr(depth+1, "test: ", p.Test)
		d.expr(depth+1, "body: ", arm.Body)
	}
}

func (d astDumper) spec(depth int, s *ast.Spec) {
	if s == nil {
		return
	}
	d.line(depth, "", fmt.Sprintf("Spec width=%d at=%s", s.Width, at(s.WidthPos)))
	f := s.Format
	if f == nil {
		return
	}
	head := fmt.Sprintf("Format %s kind=%d text=%s precision=%d%s min_int=%d min_frac=%d max_frac=%d%s", at(f.Pos), f.Kind, lexer.Quote(f.Text), f.Precision, flag("explicit", f.Explicit), f.MinInt, f.MinFrac, f.MaxFrac, flag("group", f.Group))
	d.line(depth+1, "", head)
}

func (d astDumper) ident(depth int, label string, e *ast.Ident) {
	var head strings.Builder
	fmt.Fprintf(&head, "Ident %s name=%s%s", at(e.Pos), e.Name, typed("type", e.Type))
	head.WriteString(local("local", e.Local))
	head.WriteString(variantRef(e.Variant))
	head.WriteString(flag("null", e.Null))
	head.WriteString(funcRef("func", e.Func))
	head.WriteString(constRef(e.Const))
	head.WriteString(flag("moved", e.Moved))
	d.line(depth, label, head.String())
}

func (d astDumper) selector(depth int, label string, e *ast.Selector) {
	var head strings.Builder
	fmt.Fprintf(&head, "Selector %s name=%s%s", at(e.Pos), e.Name, typed("type", e.Type))
	head.WriteString(variantRef(e.Variant))
	head.WriteString(fieldRef("field", e.Field))
	head.WriteString(fieldRef("reference", e.Reference))
	head.WriteString(builtinRef("property", e.Property))
	head.WriteString(funcRef("func", e.Func))
	head.WriteString(constRef(e.Const))
	d.line(depth, label, head.String())
	d.expr(depth+1, "x: ", e.X)
}

func (d astDumper) call(depth int, label string, e *ast.Call) {
	var head strings.Builder
	fmt.Fprintf(&head, "Call %s%s%s", at(e.Pos), flag("prepended", e.Prepended), typed("type", e.Type))
	head.WriteString(funcRef("target", e.Target))
	head.WriteString(funcRef("render", e.Render))
	head.WriteString(builtinRef("builtin", e.Builtin))
	head.WriteString(variantRef(e.Variant))
	head.WriteString(quoted("embed", e.Embed))
	head.WriteString(number("size", e.Size))
	if e.Digest != 0 {
		fmt.Fprintf(&head, " digest=%d", e.Digest)
	}
	d.line(depth, label, head.String())
	d.expr(depth+1, "fn: ", e.Fn)
	d.exprs(depth+1, "arg: ", e.Args)
}

func (d astDumper) expr(depth int, label string, e ast.Expr) {
	switch e := e.(type) {
	case nil:
	case *ast.Block:
		d.block(depth, label, e)
	case *ast.IntLit:
		d.line(depth, label, fmt.Sprintf("IntLit %s value=%d%s%s%s", at(e.Pos), e.Value, flag("char", e.Char), flag("wide", e.Wide), typed("type", e.Type)))
	case *ast.FloatLit:
		d.line(depth, label, fmt.Sprintf("FloatLit %s bits=%d%s", at(e.Pos), math.Float64bits(e.Value), typed("type", e.Type)))
	case *ast.StrLit:
		d.line(depth, label, fmt.Sprintf("StrLit %s value=%s%s", at(e.Pos), lexer.Quote(e.Value), typed("type", e.Type)))
	case *ast.BoolLit:
		d.line(depth, label, fmt.Sprintf("BoolLit %s value=%t%s", at(e.Pos), e.Value, typed("type", e.Type)))
	case *ast.Ident:
		d.ident(depth, label, e)
	case *ast.Selector:
		d.selector(depth, label, e)
	case *ast.StructLit:
		d.line(depth, label, fmt.Sprintf("StructLit %s%s name=%s%s", at(e.Pos), named("module", e.Module), e.Name, typed("type", e.Type)))
		for _, arg := range e.TypeArgs {
			d.typeName(depth+1, "type_arg: ", arg)
		}
		for _, field := range e.Fields {
			d.line(depth+1, "", fmt.Sprintf("FieldInit %s name=%s form=%d%s", at(field.Pos), field.Name, field.Form, fieldRef("field", field.Field)))
			d.expr(depth+2, "value: ", field.Value)
		}
	case *ast.Ref:
		d.line(depth, label, fmt.Sprintf("Ref %s%s", at(e.Pos), typed("type", e.Type)))
		d.expr(depth+1, "x: ", e.X)
	case *ast.Index:
		d.line(depth, label, fmt.Sprintf("Index %s%s", at(e.Pos), typed("type", e.Type)))
		d.expr(depth+1, "x: ", e.X)
		d.expr(depth+1, "index: ", e.Index)
	case *ast.Instance:
		d.line(depth, label, fmt.Sprintf("Instance %s%s", at(e.Pos), typed("type", e.Type)))
		d.expr(depth+1, "x: ", e.X)
		for _, t := range e.Types {
			d.typeName(depth+1, "type: ", t)
		}
		d.expr(depth+1, "fallback: ", e.Fallback)
	case *ast.Closure:
		d.line(depth, label, fmt.Sprintf("Closure %s%s%s%s", at(e.Pos), flag("inline", e.Inline), typed("type", e.Type), funcRef("func", e.Func)))
		d.params(depth+1, e.Params)
		d.locals(depth+1, "capture: ", e.Captures)
		d.typeName(depth+1, "result: ", e.Result)
		d.typeName(depth+1, "error: ", e.Error)
		d.block(depth+1, "body: ", e.Body)
	case *ast.Slicing:
		d.line(depth, label, fmt.Sprintf("Slicing %s%s", at(e.Pos), typed("type", e.Type)))
		d.expr(depth+1, "x: ", e.X)
		d.expr(depth+1, "lo: ", e.Lo)
		d.expr(depth+1, "hi: ", e.Hi)
	case *ast.SliceLit:
		d.line(depth, label, fmt.Sprintf("SliceLit %s%s%s", at(e.Pos), flag("read_only", e.ReadOnly), typed("type", e.Type)))
		d.exprs(depth+1, "", e.Elems)
	case *ast.Interp:
		d.line(depth, label, fmt.Sprintf("Interp %s%s", at(e.Pos), typed("type", e.Type)))
		for i, part := range e.Parts {
			d.expr(depth+1, "part: ", part)
			d.spec(depth+2, e.Specs[i])
			if i < len(e.Renders) && e.Renders[i] != nil {
				d.line(depth+2, "", fmt.Sprintf("Render %s", lexer.Quote(funcName(e.Renders[i]))))
			}
		}
	case *ast.MapLit:
		d.line(depth, label, fmt.Sprintf("MapLit %s%s%s", at(e.Pos), flag("read_only", e.ReadOnly), typed("type", e.Type)))
		for i, key := range e.Keys {
			d.expr(depth+1, "key: ", key)
			d.expr(depth+1, "value: ", e.Values[i])
		}
	case *ast.Unary:
		d.line(depth, label, fmt.Sprintf("Unary %s op=%s%s", at(e.Pos), e.Op, typed("type", e.Type)))
		d.expr(depth+1, "x: ", e.X)
	case *ast.Binary:
		d.line(depth, label, fmt.Sprintf("Binary %s op=%s%s%s%s", at(e.Pos), e.Op, flag("splat_x", e.SplatX), flag("splat_y", e.SplatY), typed("type", e.Type)))
		d.expr(depth+1, "x: ", e.X)
		d.expr(depth+1, "y: ", e.Y)
	case *ast.Call:
		d.call(depth, label, e)
	case *ast.If:
		d.line(depth, label, fmt.Sprintf("If %s%s", at(e.Pos), typed("type", e.Type)))
		d.expr(depth+1, "cond: ", e.Cond)
		d.block(depth+1, "then: ", e.Then)
		d.expr(depth+1, "else: ", e.Else)
	case *ast.Match:
		d.line(depth, label, fmt.Sprintf("Match %s%s", at(e.Pos), typed("type", e.Type)))
		d.expr(depth+1, "x: ", e.X)
		d.arms(depth+1, e.Arms)
	case *ast.Select:
		d.line(depth, label, fmt.Sprintf("Select %s%s", at(e.Pos), typed("type", e.Type)))
		for _, c := range e.Cases {
			d.line(depth+1, "", fmt.Sprintf("SelectCase %s kind=%d%s%s", at(c.Pos), c.Kind, named("name", c.Name), local("local", c.Local)))
			d.expr(depth+2, "chan: ", c.Chan)
			d.expr(depth+2, "value: ", c.Value)
			d.expr(depth+2, "body: ", c.Body)
		}
	case *ast.Subject:
		d.line(depth, label, fmt.Sprintf("Subject %s%s", at(e.Pos), typed("type", e.Type)))
	case *ast.Receiver:
		d.line(depth, label, fmt.Sprintf("Receiver%s", typed("type", e.Type)))
		d.expr(depth+1, "x: ", e.X)
	case *ast.Try:
		d.line(depth, label, fmt.Sprintf("Try %s%s", at(e.Pos), typed("type", e.Type)))
		d.expr(depth+1, "x: ", e.X)
	case *ast.Catch:
		d.line(depth, label, fmt.Sprintf("Catch %s%s", at(e.Pos), typed("type", e.Type)))
		d.expr(depth+1, "x: ", e.X)
		d.arms(depth+1, e.Arms)
		if e.Map != nil {
			d.expr(depth+1, "map: ", e.Map)
		}
	case *ast.Or:
		d.line(depth, label, fmt.Sprintf("Or %s%s", at(e.Pos), typed("type", e.Type)))
		d.expr(depth+1, "x: ", e.X)
		d.expr(depth+1, "fallback: ", e.Fallback)
	default:
		panic(fmt.Sprintf("lowdump: unhandled expression %T", e))
	}
}
