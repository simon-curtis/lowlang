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
		f.Funcs = append(f.Funcs, p.fn())
		p.skipNewlines()
	}
	return f
}

func (p *Parser) fn() *ast.Func {
	pos := p.expect(lexer.Fn).Pos
	name := p.expect(lexer.Ident).Text
	p.expect(lexer.LParen)
	var params []*ast.Param
	for !p.at(lexer.RParen) {
		params = append(params, p.param())
		if !p.at(lexer.RParen) {
			p.expect(lexer.Comma)
		}
	}
	p.expect(lexer.RParen)
	var result *ast.TypeName
	if p.at(lexer.Arrow) {
		p.next()
		result = p.typeName()
	}
	body := p.block()
	return &ast.Func{Name: name, Params: params, Result: result, Body: body, Pos: pos}
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
	return p.primary()
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
		if p.at(lexer.LParen) {
			return p.call(t)
		}
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
	}
	p.fail(t.Pos, "expected expression, found %s", describe(t))
	return nil
}

func (p *Parser) call(name lexer.Token) *ast.Call {
	p.expect(lexer.LParen)
	var args []ast.Expr
	for !p.at(lexer.RParen) {
		args = append(args, p.expr())
		if !p.at(lexer.RParen) {
			p.expect(lexer.Comma)
		}
	}
	p.expect(lexer.RParen)
	return &ast.Call{Name: name.Text, Args: args, Pos: name.Pos}
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
