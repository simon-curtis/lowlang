package parser

import (
	"fmt"
	"strconv"

	"lowlang/internal/ast"
	"lowlang/internal/lexer"
)

type Error struct {
	Pos lexer.Pos
	Msg string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Pos, e.Msg)
}

type bailout struct {
	err *Error
}

type Parser struct {
	tokens []lexer.Token
	i      int
}

func Parse(tokens []lexer.Token) (file *ast.File, err error) {
	p := &Parser{tokens: tokens}
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		b, ok := r.(bailout)
		if !ok {
			panic(r)
		}
		err = b.err
	}()
	return p.file(), nil
}

func (p *Parser) tok() lexer.Token {
	return p.tokens[p.i]
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

func (p *Parser) fail(pos lexer.Pos, format string, args ...any) {
	panic(bailout{&Error{Pos: pos, Msg: fmt.Sprintf(format, args...)}})
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
	case lexer.Ident, lexer.Int:
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

func (p *Parser) file() *ast.File {
	f := &ast.File{}
	p.skipNewlines()
	for !p.at(lexer.EOF) {
		switch t := p.tok(); t.Kind {
		case lexer.Fn:
			f.Funcs = append(f.Funcs, p.fn())
		case lexer.Enum:
			f.Enums = append(f.Enums, p.enum())
		default:
			p.fail(t.Pos, "expected fn or enum, found %s", describe(t))
		}
		p.skipNewlines()
	}
	return f
}

func (p *Parser) list(item func()) {
	p.expect(lexer.LParen)
	for !p.at(lexer.RParen) {
		item()
		if !p.at(lexer.RParen) {
			p.expect(lexer.Comma)
		}
	}
	p.expect(lexer.RParen)
}

func (p *Parser) enum() *ast.EnumDecl {
	pos := p.expect(lexer.Enum).Pos
	d := &ast.EnumDecl{Name: p.expect(lexer.Ident).Text, Pos: pos}
	p.expect(lexer.LBrace)
	p.skipNewlines()
	for !p.at(lexer.RBrace) {
		d.Variants = append(d.Variants, p.variant())
		if p.at(lexer.RBrace) {
			break
		}
		p.expect(lexer.Newline)
		p.skipNewlines()
	}
	p.expect(lexer.RBrace)
	return d
}

func (p *Parser) variant() *ast.VariantDecl {
	name := p.expect(lexer.Ident)
	v := &ast.VariantDecl{Name: name.Text, Pos: name.Pos}
	if !p.at(lexer.LParen) {
		return v
	}
	p.list(func() {
		if p.tokens[p.i+1].Kind == lexer.Colon {
			p.next()
			p.next()
		}
		v.Fields = append(v.Fields, p.typeName())
	})
	return v
}

func (p *Parser) fn() *ast.Func {
	pos := p.expect(lexer.Fn).Pos
	f := &ast.Func{Name: p.expect(lexer.Ident).Text, Pos: pos}
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

func (p *Parser) param() *ast.Param {
	name := p.expect(lexer.Ident)
	p.expect(lexer.Colon)
	return &ast.Param{Name: name.Text, TypeName: p.typeName(), Pos: name.Pos}
}

func (p *Parser) typeName() *ast.TypeName {
	t := p.expect(lexer.Ident)
	return &ast.TypeName{Name: t.Text, Pos: t.Pos}
}

func (p *Parser) block() *ast.Block {
	b := &ast.Block{Pos: p.expect(lexer.LBrace).Pos}
	p.skipNewlines()
	for !p.at(lexer.RBrace) {
		b.Stmts = append(b.Stmts, p.stmt())
		if p.at(lexer.RBrace) {
			break
		}
		p.expect(lexer.Newline)
		p.skipNewlines()
	}
	p.expect(lexer.RBrace)
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
	lexer.StarAssign: true, lexer.SlashAssign: true,
}

func (p *Parser) stmt() ast.Stmt {
	t := p.tok()
	switch t.Kind {
	case lexer.Let, lexer.Var:
		return p.let()
	case lexer.For:
		p.next()
		var cond ast.Expr
		if !p.at(lexer.LBrace) {
			cond = p.expr()
		}
		return &ast.For{Cond: cond, Body: p.block(), Pos: t.Pos}
	case lexer.Break:
		p.next()
		return &ast.Break{Pos: t.Pos}
	case lexer.Continue:
		p.next()
		return &ast.Continue{Pos: t.Pos}
	case lexer.Return:
		p.next()
		var value ast.Expr
		if !p.at(lexer.Newline) && !p.at(lexer.RBrace) {
			value = p.expr()
		}
		return &ast.Return{Value: value, Pos: t.Pos}
	case lexer.Fail:
		p.next()
		return &ast.Fail{Value: p.expr(), Pos: t.Pos}
	case lexer.Ident:
		if op := p.tokens[p.i+1].Kind; assignOps[op] {
			p.next()
			p.next()
			return &ast.Assign{Name: t.Text, Op: op, Value: p.expr(), Pos: t.Pos}
		}
	}
	return &ast.ExprStmt{X: p.expr()}
}

func (p *Parser) let() *ast.Let {
	t := p.next()
	name := p.expect(lexer.Ident).Text
	var typeName *ast.TypeName
	if p.at(lexer.Colon) {
		p.next()
		typeName = p.typeName()
	}
	p.expect(lexer.Assign)
	return &ast.Let{Name: name, Mutable: t.Kind == lexer.Var, TypeName: typeName, Value: p.expr(), Pos: t.Pos}
}

var precedence = map[lexer.Kind]int{
	lexer.OrOr:   1,
	lexer.AndAnd: 2,
	lexer.Eq:     3, lexer.NotEq: 3, lexer.Lt: 3, lexer.LtEq: 3, lexer.Gt: 3, lexer.GtEq: 3,
	lexer.Plus: 4, lexer.Minus: 4,
	lexer.Star: 5, lexer.Slash: 5, lexer.Percent: 5,
}

func (p *Parser) expr() ast.Expr {
	return p.binary(1)
}

func (p *Parser) binary(minPrec int) ast.Expr {
	x := p.unary()
	for {
		op := p.tok()
		prec, ok := precedence[op.Kind]
		if !ok || prec < minPrec {
			return x
		}
		p.next()
		y := p.binary(prec + 1)
		x = &ast.Binary{Op: op.Kind, X: x, Y: y, Pos: op.Pos}
	}
}

func (p *Parser) unary() ast.Expr {
	t := p.tok()
	if t.Kind == lexer.Minus || t.Kind == lexer.Not {
		p.next()
		return &ast.Unary{Op: t.Kind, X: p.unary(), Pos: t.Pos}
	}
	return p.postfix(p.primary())
}

func (p *Parser) primary() ast.Expr {
	t := p.tok()
	switch t.Kind {
	case lexer.Int:
		p.next()
		v, err := strconv.ParseInt(t.Text, 10, 64)
		if err != nil {
			p.fail(t.Pos, "integer %s out of range", t.Text)
		}
		return &ast.IntLit{Value: v, Pos: t.Pos}
	case lexer.String:
		p.next()
		return &ast.StrLit{Value: t.Text, Pos: t.Pos}
	case lexer.True, lexer.False:
		p.next()
		return &ast.BoolLit{Value: t.Kind == lexer.True, Pos: t.Pos}
	case lexer.Ident:
		p.next()
		return &ast.Ident{Name: t.Text, Pos: t.Pos}
	case lexer.LParen:
		p.next()
		x := p.expr()
		p.expect(lexer.RParen)
		return x
	case lexer.If:
		return p.ifExpr()
	case lexer.LBrace:
		return p.block()
	case lexer.Match:
		p.next()
		x := p.expr()
		return &ast.Match{X: x, Arms: p.arms(), Pos: t.Pos}
	}
	p.fail(t.Pos, "expected expression, found %s", describe(t))
	return nil
}

func (p *Parser) postfix(x ast.Expr) ast.Expr {
	for {
		t := p.tok()
		switch t.Kind {
		case lexer.LParen:
			call := &ast.Call{Fn: x, Pos: x.Position()}
			p.list(func() { call.Args = append(call.Args, p.expr()) })
			x = call
		case lexer.Dot:
			p.next()
			x = &ast.Selector{X: x, Name: p.expect(lexer.Ident).Text, Pos: x.Position()}
		case lexer.Question:
			p.next()
			x = &ast.Try{X: x, Pos: t.Pos}
		case lexer.Catch:
			p.next()
			x = p.catch(x, t.Pos)
		default:
			return x
		}
	}
}

func (p *Parser) catch(x ast.Expr, pos lexer.Pos) *ast.Catch {
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
	p.skipNewlines()
	for !p.at(lexer.RBrace) {
		pattern := p.pattern()
		p.expect(lexer.FatArrow)
		arms = append(arms, &ast.Arm{Pattern: pattern, Body: p.armBody()})
		if p.at(lexer.RBrace) {
			break
		}
		p.expect(lexer.Newline)
		p.skipNewlines()
	}
	p.expect(lexer.RBrace)
	return arms
}

func (p *Parser) armBody() ast.Expr {
	switch t := p.tok(); t.Kind {
	case lexer.Break, lexer.Continue, lexer.Return, lexer.Fail:
		return &ast.Block{Stmts: []ast.Stmt{p.stmt()}, Pos: t.Pos}
	}
	return p.expr()
}

func (p *Parser) pattern() *ast.Pattern {
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
	cond := p.expr()
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
