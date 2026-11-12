package parser

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/diag"
	"lowlang/internal/lexer"
)

type Parser struct {
	tokens   []lexer.Token
	i        int
	noStruct bool
	testing  bool
	taken    map[int]bool
}

func ParseSource(src []byte, at diag.Pos, privileged bool) (*ast.File, error) {
	tokens, err := lexer.Lex(src, at, privileged)
	if err != nil {
		return nil, err
	}
	p := &Parser{tokens: tokens, taken: map[int]bool{}}
	var file *ast.File
	if failure := diag.Try(func() { file = p.file() }); failure != nil {
		return nil, failure
	}
	return file, nil
}

func (p *Parser) tok() lexer.Token {
	return p.tokens[p.i]
}

func (p *Parser) doc() string {
	t := p.tok()
	if t.Doc != "" {
		p.taken[p.i] = true
	}
	return t.Doc
}

func (p *Parser) strayDocs() {
	for i, t := range p.tokens {
		if t.Doc != "" && !p.taken[i] {
			p.fail(t.DocAt, "a /// comment must come before a declaration, struct field or enum variant")
		}
	}
}

func (p *Parser) peek() lexer.Kind {
	return p.tokens[min(p.i+1, len(p.tokens)-1)].Kind
}

func (p *Parser) at(k lexer.Kind) bool {
	return p.tok().Kind == k
}

func (p *Parser) next() lexer.Token {
	t := p.tok()
	if t.Kind != lexer.EOF {
		p.i++
	}
	return t
}

func (p *Parser) fail(pos diag.Pos, format string, args ...any) {
	diag.Fail(pos, format, args...)
}

func (p *Parser) expect(k lexer.Kind) lexer.Token {
	t := p.tok()
	if t.Kind != k {
		p.fail(t.Pos, "expected %s, found %s", k, describe(t))
	}
	return p.next()
}

func describe(t lexer.Token) string {
	switch t.Kind {
	case lexer.Ident, lexer.Int, lexer.Float:
		return fmt.Sprintf("%s %s", t.Kind, t.Text)
	case lexer.String:
		return fmt.Sprintf("string %q", t.Text)
	}
	return t.Kind.String()
}

func (p *Parser) skipNewlines() {
	for p.at(lexer.Newline) {
		p.next()
	}
}

func (p *Parser) nested(parse func()) {
	saved := p.noStruct
	p.noStruct = false
	parse()
	p.noStruct = saved
}

func (p *Parser) condition() ast.Expr {
	saved := p.noStruct
	p.noStruct = true
	x := p.expr()
	p.noStruct = saved
	return x
}

func (p *Parser) list(item func()) {
	p.expect(lexer.LParen)
	p.nested(func() {
		for !p.at(lexer.RParen) {
			item()
			if !p.at(lexer.RParen) {
				p.expect(lexer.Comma)
			}
		}
	})
	p.expect(lexer.RParen)
}

func (p *Parser) lines(item func()) {
	p.expect(lexer.LBrace)
	p.nested(func() {
		p.skipNewlines()
		for !p.at(lexer.RBrace) {
			item()
			if p.at(lexer.RBrace) {
				break
			}
			p.expect(lexer.Newline)
			p.skipNewlines()
		}
	})
	p.expect(lexer.RBrace)
}

func (p *Parser) file() *ast.File {
	f := &ast.File{}
	p.skipNewlines()
	for p.at(lexer.Import) {
		f.Imports = append(f.Imports, p.importDecl())
		p.skipNewlines()
	}
	for !p.at(lexer.EOF) {
		p.declaration(f)
		p.skipNewlines()
	}
	p.strayDocs()
	return f
}

func (p *Parser) declaration(f *ast.File) {
	doc := p.doc()
	pub := p.at(lexer.Pub)
	if pub {
		p.next()
	}
	switch t := p.tok(); t.Kind {
	case lexer.Fn:
		d := p.fn()
		d.Pub, d.File, d.Doc = pub, f, doc
		f.Funcs = append(f.Funcs, d)
	case lexer.Enum:
		d := p.enum()
		d.Pub, d.File, d.Doc = pub, f, doc
		f.Enums = append(f.Enums, d)
	case lexer.Struct:
		d := p.structDecl()
		d.Pub, d.File, d.Doc = pub, f, doc
		f.Structs = append(f.Structs, d)
	case lexer.Extern:
		if p.peek() == lexer.Struct {
			p.next()
			d := p.structDecl()
			d.Pub, d.File, d.C, d.Doc = pub, f, true, doc
			f.Structs = append(f.Structs, d)
			return
		}
		if pub {
			p.fail(t.Pos, "extern blocks cannot be pub")
		}
		e := p.extern()
		for _, fn := range e.Funcs {
			fn.File = f
		}
		f.Externs = append(f.Externs, e)
	case lexer.Import:
		p.fail(t.Pos, "imports must come before declarations")
	default:
		p.fail(t.Pos, "expected fn, enum or struct, found %s", describe(t))
	}
}

func (p *Parser) extern() *ast.Extern {
	pos := p.expect(lexer.Extern).Pos
	name := p.expect(lexer.String)
	if !validLibrary(name.Text) {
		p.fail(name.Pos, "invalid library name %q", name.Text)
	}
	e := &ast.Extern{Library: name.Text, Pos: pos}
	p.lines(func() { e.Funcs = append(e.Funcs, p.externFn(e)) })
	return e
}

func (p *Parser) externFn(e *ast.Extern) *ast.Func {
	doc := p.doc()
	pub := p.at(lexer.Pub)
	if pub {
		p.next()
	}
	pos := p.expect(lexer.Fn).Pos
	f := &ast.Func{Name: p.expect(lexer.Ident).Text, Doc: doc, Pub: pub, Extern: e, Pos: pos}
	p.list(func() {
		if f.Variadic {
			p.fail(p.tok().Pos, "... must be the last parameter")
		}
		if p.at(lexer.Ellipsis) {
			p.next()
			f.Variadic = true
			return
		}
		f.Params = append(f.Params, p.param())
	})
	if p.at(lexer.Arrow) {
		p.next()
		f.Result = p.typeName()
	}
	f.Symbol = f.Name
	if p.at(lexer.Assign) {
		p.next()
		symbol := p.expect(lexer.String)
		if !validSymbol(symbol.Text) {
			p.fail(symbol.Pos, "invalid C symbol %q", symbol.Text)
		}
		f.Symbol = symbol.Text
	}
	return f
}

func validSymbol(name string) bool {
	if name == "" || (name[0] >= '0' && name[0] <= '9') {
		return false
	}
	for _, c := range name {
		if c != '_' && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

func validLibrary(name string) bool {
	if name == "" || name[0] == '-' {
		return false
	}
	for _, c := range name {
		ok := c == '_' || c == '.' || c == '+' || c == '-' ||
			(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if !ok {
			return false
		}
	}
	return true
}

func (p *Parser) importDecl() *ast.Import {
	pos := p.expect(lexer.Import).Pos
	first := p.expect(lexer.Ident).Text
	alias := ""
	if p.at(lexer.Assign) {
		p.next()
		alias = first
		first = p.expect(lexer.Ident).Text
	}
	segments := []string{first}
	for p.at(lexer.Slash) {
		p.next()
		segments = append(segments, p.expect(lexer.Ident).Text)
	}
	if alias == "" {
		alias = segments[len(segments)-1]
	}
	return &ast.Import{Alias: alias, Path: strings.Join(segments, "/"), Pos: pos}
}

func (p *Parser) enum() *ast.EnumDecl {
	pos := p.expect(lexer.Enum).Pos
	d := &ast.EnumDecl{Name: p.expect(lexer.Ident).Text, Pos: pos}
	d.TypeParams = p.typeParams()
	p.lines(func() { d.Variants = append(d.Variants, p.variant()) })
	return d
}

func (p *Parser) variant() *ast.VariantDecl {
	doc := p.doc()
	name := p.expect(lexer.Ident)
	v := &ast.VariantDecl{Name: name.Text, Doc: doc, Pos: name.Pos}
	if !p.at(lexer.LParen) {
		return v
	}
	p.list(func() {
		label := ""
		if p.peek() == lexer.Colon {
			label = p.expect(lexer.Ident).Text
			p.expect(lexer.Colon)
		}
		v.Labels = append(v.Labels, label)
		v.Fields = append(v.Fields, p.typeName())
	})
	return v
}

func (p *Parser) structDecl() *ast.StructDecl {
	pos := p.expect(lexer.Struct).Pos
	d := &ast.StructDecl{Name: p.expect(lexer.Ident).Text, Pos: pos}
	d.TypeParams = p.typeParams()
	p.lines(func() {
		if t := p.tok(); t.Kind == lexer.Drop {
			p.next()
			if d.Drop != nil {
				p.fail(t.Pos, "struct %s already has a drop", d.Name)
			}
			d.Drop = p.stmt()
			return
		}
		doc := p.doc()
		pub := p.at(lexer.Pub)
		if pub {
			p.next()
		}
		name := p.expect(lexer.Ident)
		p.expect(lexer.Colon)
		field := &ast.FieldDecl{Name: name.Text, Doc: doc, Pub: pub, TypeName: p.typeName(), Pos: name.Pos}
		if p.at(lexer.Assign) {
			p.next()
			field.Default = p.expr()
		}
		d.Fields = append(d.Fields, field)
	})
	return d
}

func (p *Parser) trailing() *ast.SliceLit {
	if !p.at(lexer.LBrace) || p.noStruct {
		return nil
	}
	lit := &ast.SliceLit{Pos: p.next().Pos}
	p.nested(func() {
		p.skipNewlines()
		for !p.at(lexer.RBrace) {
			lit.Elems = append(lit.Elems, p.expr())
			if p.at(lexer.Comma) {
				p.next()
			} else if !p.at(lexer.RBrace) {
				p.expect(lexer.Newline)
			}
			p.skipNewlines()
		}
	})
	p.expect(lexer.RBrace)
	return lit
}

func (p *Parser) fn() *ast.Func {
	pos := p.expect(lexer.Fn).Pos
	f := &ast.Func{Name: p.expect(lexer.Ident).Text, Pos: pos}
	f.TypeParams = p.typeParams()
	p.list(func() { f.Params = append(f.Params, p.param()) })
	if p.at(lexer.Arrow) {
		p.next()
		f.Result = p.typeName()
	}
	if p.at(lexer.Not) {
		p.next()
		f.Error = p.typeName()
	}
	f.Body = p.block()
	return f
}

func (p *Parser) typeParams() []*ast.TypeParam {
	if !p.at(lexer.LBracket) {
		return nil
	}
	var params []*ast.TypeParam
	p.brackets(func() {
		name := p.expect(lexer.Ident)
		param := &ast.TypeParam{Name: name.Text, Pos: name.Pos}
		if p.at(lexer.Colon) {
			p.next()
			param.Constraints = append(param.Constraints, p.expect(lexer.Ident).Text)
			for p.at(lexer.Plus) {
				p.next()
				param.Constraints = append(param.Constraints, p.expect(lexer.Ident).Text)
			}
		}
		params = append(params, param)
	})
	if len(params) == 0 {
		p.fail(p.tok().Pos, "expected type parameters")
	}
	return params
}

func (p *Parser) brackets(item func()) {
	p.expect(lexer.LBracket)
	p.nested(func() {
		for !p.at(lexer.RBracket) {
			item()
			if !p.at(lexer.RBracket) {
				p.expect(lexer.Comma)
			}
		}
	})
	p.expect(lexer.RBracket)
}

func (p *Parser) typeArgs() []*ast.TypeName {
	var args []*ast.TypeName
	p.brackets(func() { args = append(args, p.typeName()) })
	if len(args) == 0 {
		p.fail(p.tok().Pos, "expected type arguments")
	}
	return args
}

func (p *Parser) attempt(parse func()) bool {
	start, noStruct, testing := p.i, p.noStruct, p.testing
	if diag.Try(parse) != nil {
		p.i, p.noStruct, p.testing = start, noStruct, testing
		return false
	}
	return true
}

func (p *Parser) genericLit(name lexer.Token) *ast.StructLit {
	if !p.at(lexer.LBracket) || p.noStruct {
		return nil
	}
	var args []*ast.TypeName
	found := p.attempt(func() {
		args = p.typeArgs()
		if !p.at(lexer.LBrace) {
			p.fail(p.tok().Pos, "expected {")
		}
	})
	if !found {
		return nil
	}
	lit := p.structLit(name)
	lit.TypeArgs = args
	return lit
}

func (p *Parser) instance(x ast.Expr) *ast.Instance {
	start, pos := p.i, p.tok().Pos
	var args []*ast.TypeName
	found := p.attempt(func() {
		args = p.typeArgs()
		if !p.at(lexer.LParen) {
			p.fail(p.tok().Pos, "expected (")
		}
	})
	if !found {
		return nil
	}
	end := p.i
	p.i = start
	var fallback ast.Expr
	if !p.attempt(func() {
		fallback = p.index(x)
		if p.i != end {
			p.fail(p.tok().Pos, "expected (")
		}
	}) {
		fallback = nil
	}
	p.i = end
	return &ast.Instance{X: x, Types: args, Fallback: fallback, Pos: pos}
}

func (p *Parser) param() *ast.Param {
	mut := p.at(lexer.Mut)
	if mut {
		p.next()
	}
	name := p.expect(lexer.Ident)
	p.expect(lexer.Colon)
	return &ast.Param{Name: name.Text, Mut: mut, TypeName: p.typeName(), Pos: name.Pos}
}

func (p *Parser) funcType(pos diag.Pos, c bool) *ast.TypeName {
	p.expect(lexer.Fn)
	t := &ast.TypeName{Func: true, CFunc: c, Pos: pos}
	p.list(func() {
		mut := p.at(lexer.Mut)
		if mut {
			p.next()
		}
		t.Muts = append(t.Muts, mut)
		t.Params = append(t.Params, p.typeName())
	})
	if p.at(lexer.Arrow) {
		p.next()
		t.Result = p.typeName()
	}
	if p.at(lexer.Not) {
		p.next()
		t.Error = p.typeName()
	}
	return t
}

func (p *Parser) closure() *ast.Closure {
	pos := p.expect(lexer.Fn).Pos
	c := &ast.Closure{Pos: pos}
	p.list(func() {
		mut := p.at(lexer.Mut)
		if mut {
			p.next()
		}
		name := p.expect(lexer.Ident)
		param := &ast.Param{Name: name.Text, Mut: mut, Pos: name.Pos}
		if p.at(lexer.Colon) {
			p.next()
			param.TypeName = p.typeName()
		}
		c.Params = append(c.Params, param)
	})
	if p.at(lexer.Arrow) {
		p.next()
		c.Result = p.typeName()
	}
	if p.at(lexer.Not) {
		p.next()
		c.Error = p.typeName()
	}
	c.Body = p.block()
	return c
}

func (p *Parser) typeName() *ast.TypeName {
	t := p.typeTerm()
	for p.at(lexer.Question) {
		p.next()
		t = &ast.TypeName{Optional: true, Elem: t, Pos: t.Pos}
	}
	return t
}

func (p *Parser) typeTerm() *ast.TypeName {
	pos := p.tok().Pos
	switch {
	case p.at(lexer.Fn):
		return p.funcType(pos, false)
	case p.at(lexer.Extern) && p.peek() == lexer.Fn:
		p.next()
		return p.funcType(pos, true)
	case p.at(lexer.Star):
		p.next()
		return &ast.TypeName{Pointer: true, Elem: p.typeTerm(), Pos: pos}
	case p.at(lexer.LBracket):
		p.next()
		if p.at(lexer.RBracket) {
			p.next()
			return &ast.TypeName{Slice: true, Elem: p.typeTerm(), Pos: pos}
		}
		key := p.typeName()
		p.expect(lexer.Colon)
		value := p.typeName()
		p.expect(lexer.RBracket)
		return &ast.TypeName{Map: true, Key: key, Elem: value, Pos: pos}
	}
	t := &ast.TypeName{Name: p.expect(lexer.Ident).Text, Pos: pos}
	if p.at(lexer.Dot) {
		p.next()
		t.Module, t.Name = t.Name, p.expect(lexer.Ident).Text
	}
	if p.at(lexer.LBracket) {
		t.Args = p.typeArgs()
	}
	return t
}

func (p *Parser) block() *ast.Block {
	b := &ast.Block{Pos: p.tok().Pos}
	p.lines(func() { b.Stmts = append(b.Stmts, p.stmt()) })
	if n := len(b.Stmts); n > 0 {
		if es, ok := b.Stmts[n-1].(*ast.ExprStmt); ok {
			b.Tail = es.X
			b.Stmts = b.Stmts[:n-1]
		}
	}
	return b
}

var assignOps = map[lexer.Kind]bool{
	lexer.Assign: true, lexer.PlusAssign: true, lexer.MinusAssign: true,
	lexer.StarAssign: true, lexer.SlashAssign: true, lexer.PercentAssign: true,
	lexer.AmpAssign: true, lexer.PipeAssign: true, lexer.CaretAssign: true,
	lexer.ShlAssign: true, lexer.ShrAssign: true,
}

func (p *Parser) stmt() ast.Stmt {
	t := p.tok()
	switch t.Kind {
	case lexer.Let, lexer.Var:
		return p.let()
	case lexer.Using:
		p.next()
		mutable := p.at(lexer.Var)
		if mutable {
			p.next()
		}
		s := p.binding(t.Pos, mutable)
		s.Using = true
		return s
	case lexer.For:
		return p.loop("")
	case lexer.Break:
		p.next()
		return &ast.Break{Label: p.label(), Pos: t.Pos}
	case lexer.Continue:
		p.next()
		return &ast.Continue{Label: p.label(), Pos: t.Pos}
	case lexer.Ident:
		if p.peek() == lexer.Colon {
			return p.labelled(t)
		}
	case lexer.Return:
		p.next()
		var value ast.Expr
		if !p.atEnd() {
			value = p.expr()
		}
		return &ast.Return{Value: value, Pos: t.Pos}
	case lexer.Fail:
		p.next()
		return &ast.Fail{Value: p.expr(), Pos: t.Pos}
	case lexer.Defer:
		p.next()
		return &ast.Defer{Body: p.stmt(), Pos: t.Pos}
	case lexer.Spawn:
		p.next()
		call, ok := p.expr().(*ast.Call)
		if !ok {
			p.fail(t.Pos, "spawn needs a function call")
		}
		return &ast.Spawn{Call: call, Pos: t.Pos}
	}
	x := p.expr()
	if p.at(lexer.LArrow) {
		p.next()
		return &ast.Send{Chan: x, Value: p.expr(), Pos: t.Pos}
	}
	if op := p.tok().Kind; assignOps[op] {
		p.next()
		return &ast.Assign{Target: x, Op: op, Value: p.expr(), Pos: t.Pos}
	}
	return &ast.ExprStmt{X: x}
}

func (p *Parser) loop(label string) ast.Stmt {
	pos := p.expect(lexer.For).Pos
	if p.at(lexer.Ident) && (p.peek() == lexer.In || p.peek() == lexer.Comma) {
		return p.forIn(label, pos)
	}
	var cond ast.Expr
	if !p.at(lexer.LBrace) {
		cond = p.condition()
	}
	return &ast.For{Label: label, Cond: cond, Body: p.block(), Pos: pos}
}

func (p *Parser) labelled(name lexer.Token) ast.Stmt {
	p.next()
	p.next()
	if !p.at(lexer.For) {
		p.fail(name.Pos, "label %s must be followed by a for loop", name.Text)
	}
	return p.loop(name.Text)
}

func (p *Parser) label() string {
	if !p.at(lexer.Ident) {
		return ""
	}
	return p.next().Text
}

func (p *Parser) forIn(label string, pos diag.Pos) *ast.ForIn {
	loop := &ast.ForIn{Label: label, Value: p.expect(lexer.Ident).Text, Pos: pos}
	if p.at(lexer.Comma) {
		p.next()
		loop.Key = loop.Value
		loop.Value = p.expect(lexer.Ident).Text
	}
	p.expect(lexer.In)
	loop.Iter = p.condition()
	if p.at(lexer.DotDot) {
		p.next()
		loop.End = p.condition()
	}
	loop.Body = p.block()
	return loop
}

func (p *Parser) let() *ast.Let {
	t := p.next()
	return p.binding(t.Pos, t.Kind == lexer.Var)
}

func (p *Parser) binding(pos diag.Pos, mutable bool) *ast.Let {
	name := p.expect(lexer.Ident).Text
	var typeName *ast.TypeName
	if p.at(lexer.Colon) {
		p.next()
		typeName = p.typeName()
	}
	p.expect(lexer.Assign)
	return &ast.Let{Name: name, Mutable: mutable, TypeName: typeName, Value: p.expr(), Pos: pos}
}

var precedence = map[lexer.Kind]int{
	lexer.PipeArrow: 1,
	lexer.OrOr:      2,
	lexer.AndAnd:    3,
	lexer.Eq:        4, lexer.NotEq: 4, lexer.Lt: 4, lexer.LtEq: 4, lexer.Gt: 4, lexer.GtEq: 4,
	lexer.Coalesce: 5,
	lexer.Plus:     6, lexer.Minus: 6, lexer.Pipe: 6, lexer.Caret: 6,
	lexer.Star: 7, lexer.Slash: 7, lexer.Percent: 7, lexer.Amp: 7, lexer.Shl: 7, lexer.Shr: 7,
}

func (p *Parser) expr() ast.Expr {
	return p.binary(1)
}

func (p *Parser) binary(minPrec int) ast.Expr {
	return p.binaryFrom(p.unary(), minPrec)
}

func (p *Parser) binaryFrom(x ast.Expr, minPrec int) ast.Expr {
	for {
		op := p.tok()
		prec, ok := precedence[op.Kind]
		if !ok || prec < minPrec {
			return x
		}
		p.next()
		next := prec + 1
		if op.Kind == lexer.Coalesce {
			next = prec
		}
		y := p.binary(next)
		if op.Kind == lexer.PipeArrow {
			x = p.pipe(x, y, op.Pos)
			continue
		}
		x = &ast.Binary{Op: op.Kind, X: x, Y: y, Pos: op.Pos}
	}
}

func (p *Parser) pipe(x, y ast.Expr, pos diag.Pos) ast.Expr {
	switch y := y.(type) {
	case *ast.Try:
		y.X = p.pipe(x, y.X, pos)
		return y
	case *ast.Call:
		y.Args = append([]ast.Expr{x}, y.Args...)
		y.Prepended = true
		return y
	case *ast.Ident, *ast.Selector, *ast.Instance, *ast.Closure:
		return &ast.Call{Fn: y, Args: []ast.Expr{x}, Pos: pos, Prepended: true}
	}
	p.fail(y.Position(), "the right side of |> must be a function or a call")
	return nil
}

func (p *Parser) unary() ast.Expr {
	t := p.tok()
	if p.testing && precedence[t.Kind] > 0 && t.Kind != lexer.Minus && t.Kind != lexer.Star {
		return p.binaryFrom(&ast.Subject{Pos: t.Pos}, precedence[lexer.Eq])
	}
	switch t.Kind {
	case lexer.Minus:
		p.next()
		if lit := p.tok(); lit.Kind == lexer.Int || lit.Kind == lexer.Float {
			p.next()
			return p.postfix(p.number(lit, true, t.Pos))
		}
		return &ast.Unary{Op: t.Kind, X: p.unary(), Pos: t.Pos}
	case lexer.Not, lexer.Star, lexer.Tilde, lexer.LArrow:
		p.next()
		return &ast.Unary{Op: t.Kind, X: p.unary(), Pos: t.Pos}
	}
	return p.postfix(p.primary())
}

func (p *Parser) selectExpr() *ast.Select {
	s := &ast.Select{Pos: p.expect(lexer.Select).Pos}
	p.lines(func() { s.Cases = append(s.Cases, p.selectCase()) })
	return s
}

func (p *Parser) selectCase() *ast.SelectCase {
	t := p.tok()
	c := &ast.SelectCase{Pos: t.Pos}
	switch {
	case p.at(lexer.Else):
		p.next()
		c.Kind = ast.ElseCase
	case p.at(lexer.Ident) && t.Text == "after" && p.peek() == lexer.LParen:
		p.next()
		p.list(func() { c.Value = p.expr() })
		c.Kind = ast.AfterCase
	case p.at(lexer.Ident) && p.peek() == lexer.Assign:
		c.Name = p.next().Text
		p.next()
		p.expect(lexer.LArrow)
		c.Chan = p.expr()
	case p.at(lexer.LArrow):
		p.next()
		c.Chan = p.expr()
	default:
		c.Chan = p.expr()
		p.expect(lexer.LArrow)
		c.Value = p.expr()
		c.Kind = ast.SendCase
	}
	p.expect(lexer.FatArrow)
	c.Body = p.expr()
	return c
}

func (p *Parser) number(t lexer.Token, negative bool, pos diag.Pos) ast.Expr {
	if t.Kind == lexer.Float {
		v, err := strconv.ParseFloat(t.Text, 64)
		if err != nil {
			p.fail(t.Pos, "invalid float literal %s", t.Text)
		}
		if negative {
			v = -v
		}
		return &ast.FloatLit{Value: v, Pos: pos}
	}
	u, err := strconv.ParseUint(t.Text, 0, 64)
	if err != nil || (negative && u > uint64(math.MaxInt64)+1) {
		p.fail(t.Pos, "integer %s out of range", t.Text)
	}
	v := int64(u)
	if negative {
		v = -v
	}
	return &ast.IntLit{Value: v, Wide: !negative && u > uint64(math.MaxInt64), Pos: pos}
}

func (p *Parser) primary() ast.Expr {
	t := p.tok()
	switch t.Kind {
	case lexer.Break, lexer.Continue, lexer.Return, lexer.Fail:
		return &ast.Block{Stmts: []ast.Stmt{p.stmt()}, Pos: t.Pos}
	case lexer.Int, lexer.Float:
		p.next()
		return p.number(t, false, t.Pos)
	case lexer.String:
		p.next()
		if t.Parts != nil {
			return p.interpolation(t)
		}
		return &ast.StrLit{Value: t.Text, Pos: t.Pos}
	case lexer.Char:
		p.next()
		return &ast.IntLit{Value: int64(t.Char), Char: true, Pos: t.Pos}
	case lexer.LBracket:
		return p.sliceLit()
	case lexer.True, lexer.False:
		p.next()
		return &ast.BoolLit{Value: t.Kind == lexer.True, Pos: t.Pos}
	case lexer.Ident:
		p.next()
		if lit := p.genericLit(t); lit != nil {
			return lit
		}
		if p.at(lexer.LBrace) && !p.noStruct {
			return p.structLit(t)
		}
		return &ast.Ident{Name: t.Text, Pos: t.Pos}
	case lexer.LParen:
		p.next()
		var x ast.Expr
		p.nested(func() { x = p.expr() })
		p.expect(lexer.RParen)
		return x
	case lexer.If:
		return p.ifExpr()
	case lexer.LBrace:
		return p.block()
	case lexer.Fn:
		return p.closure()
	case lexer.Select:
		return p.selectExpr()
	case lexer.Match:
		p.next()
		x := p.condition()
		return &ast.Match{X: x, Arms: p.arms(), Pos: t.Pos}
	}
	p.fail(t.Pos, "expected expression, found %s", describe(t))
	return nil
}

func (p *Parser) interpolation(t lexer.Token) *ast.Interp {
	n := &ast.Interp{Pos: t.Pos}
	for _, part := range t.Parts {
		if part.Tokens == nil {
			n.Parts = append(n.Parts, &ast.StrLit{Value: part.Text, Pos: t.Pos})
			n.Specs = append(n.Specs, nil)
			continue
		}
		inner := &Parser{tokens: part.Tokens}
		x := inner.expr()
		inner.skipNewlines()
		if !inner.at(lexer.EOF) {
			inner.fail(inner.tok().Pos, "unexpected %s in interpolation", describe(inner.tok()))
		}
		n.Parts = append(n.Parts, x)
		n.Specs = append(n.Specs, p.spec(part))
	}
	return n
}

func (p *Parser) spec(part lexer.Part) *ast.Spec {
	if part.Spec == "" {
		return nil
	}
	spec, failure := ast.ParseSpec(part.Spec, part.SpecPos)
	if failure != nil {
		p.fail(failure.Pos, "%s", failure.Msg)
	}
	return spec
}

func (p *Parser) sliceLit() ast.Expr {
	pos := p.expect(lexer.LBracket).Pos
	if p.at(lexer.Colon) {
		p.next()
		p.expect(lexer.RBracket)
		return &ast.MapLit{Pos: pos}
	}
	var keys, values []ast.Expr
	mapped := false
	p.nested(func() {
		p.skipNewlines()
		for !p.at(lexer.RBracket) {
			keys = append(keys, p.expr())
			if len(keys) == 1 && p.at(lexer.Colon) {
				mapped = true
			}
			if mapped {
				p.expect(lexer.Colon)
				values = append(values, p.expr())
			}
			if p.at(lexer.Comma) {
				p.next()
			} else if !p.at(lexer.RBracket) {
				p.expect(lexer.Newline)
			}
			p.skipNewlines()
		}
	})
	p.expect(lexer.RBracket)
	if mapped {
		return &ast.MapLit{Keys: keys, Values: values, Pos: pos}
	}
	return &ast.SliceLit{Elems: keys, Pos: pos}
}

func (p *Parser) index(x ast.Expr) ast.Expr {
	pos := p.expect(lexer.LBracket).Pos
	var lo, hi ast.Expr
	sliced := false
	p.nested(func() {
		if !p.at(lexer.DotDot) {
			lo = p.expr()
		}
		if p.at(lexer.DotDot) {
			p.next()
			sliced = true
			if !p.at(lexer.RBracket) {
				hi = p.expr()
			}
		}
	})
	p.expect(lexer.RBracket)
	if sliced {
		return &ast.Slicing{X: x, Lo: lo, Hi: hi, Pos: pos}
	}
	if lo == nil {
		p.fail(pos, "missing index")
	}
	return &ast.Index{X: x, Index: lo, Pos: pos}
}

func (p *Parser) structLit(name lexer.Token) *ast.StructLit {
	lit := &ast.StructLit{Name: name.Text, Pos: name.Pos}
	p.expect(lexer.LBrace)
	p.nested(func() {
		p.skipNewlines()
		for !p.at(lexer.RBrace) {
			lit.Fields = append(lit.Fields, p.fieldInit())
			if p.at(lexer.Comma) {
				p.next()
			} else if !p.at(lexer.RBrace) {
				p.expect(lexer.Newline)
			}
			p.skipNewlines()
		}
	})
	p.expect(lexer.RBrace)
	if children := p.trailing(); children != nil {
		lit.Fields = append(lit.Fields, &ast.FieldInit{Name: "children", Value: children, Pos: children.Pos})
	}
	return lit
}

func (p *Parser) fieldInit() *ast.FieldInit {
	name := p.expect(lexer.Ident)
	init := &ast.FieldInit{Name: name.Text, Pos: name.Pos}
	if !p.at(lexer.Colon) {
		init.Value = &ast.Ident{Name: name.Text, Pos: name.Pos}
		return init
	}
	p.next()
	init.Value = p.expr()
	return init
}

func (p *Parser) postfix(x ast.Expr) ast.Expr {
	for {
		t := p.tok()
		switch t.Kind {
		case lexer.LParen:
			call := &ast.Call{Fn: x, Pos: x.Position()}
			p.list(func() { call.Args = append(call.Args, p.arg()) })
			if children := p.trailing(); children != nil {
				call.Args = append(call.Args, children)
			}
			x = call
		case lexer.Dot:
			p.next()
			name := p.expect(lexer.Ident)
			if module, ok := x.(*ast.Ident); ok {
				if lit := p.genericLit(name); lit != nil {
					lit.Module = module.Name
					x = lit
					continue
				}
			}
			if module, ok := x.(*ast.Ident); ok && p.at(lexer.LBrace) && !p.noStruct {
				x = p.structLit(name)
				x.(*ast.StructLit).Module = module.Name
				continue
			}
			x = &ast.Selector{X: x, Name: name.Text, Pos: name.Pos}
		case lexer.Question:
			p.next()
			x = &ast.Try{X: x, Pos: t.Pos}
		case lexer.LBracket:
			if inst := p.instance(x); inst != nil {
				x = inst
				continue
			}
			x = p.index(x)
		case lexer.Catch:
			p.next()
			x = p.catch(x, t.Pos)
		default:
			return x
		}
	}
}

func (p *Parser) arg() ast.Expr {
	if !p.at(lexer.Mut) {
		return p.expr()
	}
	pos := p.next().Pos
	return &ast.Ref{X: p.expr(), Pos: pos}
}

func (p *Parser) catch(x ast.Expr, pos diag.Pos) *ast.Catch {
	c := &ast.Catch{X: x, Pos: pos}
	if p.at(lexer.LBrace) {
		c.Arms = p.arms()
		return c
	}
	c.Fallback = p.unary()
	return c
}

func (p *Parser) arms() []*ast.Arm {
	var arms []*ast.Arm
	p.expect(lexer.LBrace)
	p.nested(func() {
		p.skipNewlines()
		for !p.at(lexer.RBrace) {
			pattern := p.pattern()
			p.expect(lexer.FatArrow)
			arms = append(arms, &ast.Arm{Pattern: pattern, Body: p.expr()})
			if p.at(lexer.Comma) {
				p.next()
			} else if !p.at(lexer.RBrace) {
				p.expect(lexer.Newline)
			}
			p.skipNewlines()
		}
	})
	p.expect(lexer.RBrace)
	return arms
}

func (p *Parser) atEnd() bool {
	switch p.tok().Kind {
	case lexer.Newline, lexer.RBrace, lexer.RParen, lexer.RBracket, lexer.Comma:
		return true
	}
	return false
}

func (p *Parser) valueArm() bool {
	switch p.tok().Kind {
	case lexer.Int, lexer.Float, lexer.Char, lexer.String, lexer.True, lexer.False, lexer.LParen, lexer.Not, lexer.Tilde:
		return true
	case lexer.Minus:
		next := p.peek()
		return next == lexer.Int || next == lexer.Float
	case lexer.Ident:
		return p.peek() == lexer.Dot
	}
	return false
}

func (p *Parser) pattern() *ast.Pattern {
	if t := p.tok(); p.valueArm() {
		subject := &ast.Subject{Pos: t.Pos}
		test := &ast.Binary{Op: lexer.Eq, X: subject, Y: p.expr(), Pos: t.Pos}
		return &ast.Pattern{Test: test, Pos: t.Pos}
	}
	if t := p.tok(); precedence[t.Kind] > 0 {
		p.testing = true
		test := p.binaryFrom(&ast.Subject{Pos: t.Pos}, 0)
		p.testing = false
		return &ast.Pattern{Test: test, Pos: t.Pos}
	}
	name := p.expect(lexer.Ident)
	pat := &ast.Pattern{Name: name.Text, Pos: name.Pos}
	if !p.at(lexer.LParen) {
		return pat
	}
	p.list(func() {
		b := p.expect(lexer.Ident)
		pat.Bindings = append(pat.Bindings, &ast.Binding{Name: b.Text, Pos: b.Pos})
	})
	return pat
}

func (p *Parser) ifExpr() *ast.If {
	pos := p.expect(lexer.If).Pos
	cond := p.condition()
	then := p.block()
	n := &ast.If{Cond: cond, Then: then, Pos: pos}
	if !p.at(lexer.Else) {
		return n
	}
	p.next()
	if p.at(lexer.If) {
		n.Else = p.ifExpr()
		return n
	}
	n.Else = p.block()
	return n
}
