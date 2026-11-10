package parser

import (
	"errors"
	"fmt"
	"math"
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
	tokens   []lexer.Token
	i        int
	noStruct bool
	prelude  bool
}

func Parse(tokens []lexer.Token) (*ast.File, error) {
	return parse(&Parser{tokens: tokens})
}

func ParsePrelude(tokens []lexer.Token) (*ast.File, error) {
	return parse(&Parser{tokens: tokens, prelude: true})
}

func parse(p *Parser) (file *ast.File, err error) {
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
	for !p.at(lexer.EOF) {
		switch t := p.tok(); t.Kind {
		case lexer.Fn:
			f.Funcs = append(f.Funcs, p.fn())
		case lexer.Enum:
			f.Enums = append(f.Enums, p.enum())
		case lexer.Struct:
			f.Structs = append(f.Structs, p.structDecl())
		default:
			p.fail(t.Pos, "expected fn, enum or struct, found %s", describe(t))
		}
		p.skipNewlines()
	}
	return f
}

func (p *Parser) enum() *ast.EnumDecl {
	pos := p.expect(lexer.Enum).Pos
	d := &ast.EnumDecl{Name: p.expect(lexer.Ident).Text, Pos: pos}
	p.lines(func() { d.Variants = append(d.Variants, p.variant()) })
	return d
}

func (p *Parser) variant() *ast.VariantDecl {
	name := p.expect(lexer.Ident)
	v := &ast.VariantDecl{Name: name.Text, Pos: name.Pos}
	if !p.at(lexer.LParen) {
		return v
	}
	p.list(func() {
		if p.peek() == lexer.Colon {
			p.next()
			p.next()
		}
		v.Fields = append(v.Fields, p.typeName())
	})
	return v
}

func (p *Parser) structDecl() *ast.StructDecl {
	pos := p.expect(lexer.Struct).Pos
	d := &ast.StructDecl{Name: p.expect(lexer.Ident).Text, Pos: pos}
	p.lines(func() {
		name := p.expect(lexer.Ident)
		p.expect(lexer.Colon)
		d.Fields = append(d.Fields, &ast.FieldDecl{Name: name.Text, TypeName: p.typeName(), Pos: name.Pos})
	})
	return d
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
	mut := p.at(lexer.Mut)
	if mut {
		p.next()
	}
	name := p.expect(lexer.Ident)
	p.expect(lexer.Colon)
	return &ast.Param{Name: name.Text, Mut: mut, TypeName: p.typeName(), Pos: name.Pos}
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
	case p.at(lexer.Star):
		p.next()
		return &ast.TypeName{Pointer: true, Elem: p.typeTerm(), Pos: pos}
	case p.at(lexer.LBracket):
		p.next()
		p.expect(lexer.RBracket)
		return &ast.TypeName{Slice: true, Elem: p.typeTerm(), Pos: pos}
	}
	return &ast.TypeName{Name: p.expect(lexer.Ident).Text, Pos: pos}
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
	case lexer.For:
		p.next()
		if p.at(lexer.Ident) && (p.peek() == lexer.In || p.peek() == lexer.Comma) {
			return p.forIn(t.Pos)
		}
		var cond ast.Expr
		if !p.at(lexer.LBrace) {
			cond = p.condition()
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
	}
	x := p.expr()
	if op := p.tok().Kind; assignOps[op] {
		p.next()
		return &ast.Assign{Target: x, Op: op, Value: p.expr(), Pos: t.Pos}
	}
	return &ast.ExprStmt{X: x}
}

func (p *Parser) forIn(pos lexer.Pos) *ast.ForIn {
	loop := &ast.ForIn{Value: p.expect(lexer.Ident).Text, Pos: pos}
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
	lexer.Coalesce: 4,
	lexer.Plus:     5, lexer.Minus: 5, lexer.Pipe: 5, lexer.Caret: 5,
	lexer.Star: 6, lexer.Slash: 6, lexer.Percent: 6, lexer.Amp: 6, lexer.Shl: 6, lexer.Shr: 6,
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
		next := prec + 1
		if op.Kind == lexer.Coalesce {
			next = prec
		}
		y := p.binary(next)
		x = &ast.Binary{Op: op.Kind, X: x, Y: y, Pos: op.Pos}
	}
}

func (p *Parser) unary() ast.Expr {
	t := p.tok()
	switch t.Kind {
	case lexer.Minus:
		p.next()
		if lit := p.tok(); lit.Kind == lexer.Int || lit.Kind == lexer.Float {
			p.next()
			return p.postfix(p.number(lit, true, t.Pos))
		}
		return &ast.Unary{Op: t.Kind, X: p.unary(), Pos: t.Pos}
	case lexer.Not, lexer.Star, lexer.Tilde:
		p.next()
		return &ast.Unary{Op: t.Kind, X: p.unary(), Pos: t.Pos}
	}
	return p.postfix(p.primary())
}

func (p *Parser) number(t lexer.Token, negative bool, pos lexer.Pos) ast.Expr {
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
	limit := uint64(math.MaxInt64)
	if negative {
		limit++
	}
	if err != nil || u > limit {
		p.fail(t.Pos, "integer %s out of range", t.Text)
	}
	v := int64(u)
	if negative {
		v = -v
	}
	return &ast.IntLit{Value: v, Pos: pos}
}

func (p *Parser) primary() ast.Expr {
	t := p.tok()
	switch t.Kind {
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
		code, _ := strconv.ParseInt(t.Text, 10, 64)
		return &ast.IntLit{Value: code, Char: true, Pos: t.Pos}
	case lexer.LBracket:
		return p.sliceLit()
	case lexer.True, lexer.False:
		p.next()
		return &ast.BoolLit{Value: t.Kind == lexer.True, Pos: t.Pos}
	case lexer.Ident:
		p.next()
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
		if !part.Expr {
			n.Parts = append(n.Parts, &ast.StrLit{Value: part.Text, Pos: t.Pos})
			continue
		}
		tokens, err := lexer.LexAt([]byte(part.Text), part.Pos, p.prelude)
		if err != nil {
			var lexErr *lexer.Error
			if errors.As(err, &lexErr) {
				p.fail(lexErr.Pos, "%s", lexErr.Msg)
			}
			p.fail(part.Pos, "%v", err)
		}
		inner := &Parser{tokens: tokens, prelude: p.prelude}
		x := inner.expr()
		inner.skipNewlines()
		if !inner.at(lexer.EOF) {
			inner.fail(inner.tok().Pos, "unexpected %s in interpolation", describe(inner.tok()))
		}
		n.Parts = append(n.Parts, x)
	}
	return n
}

func (p *Parser) sliceLit() *ast.SliceLit {
	lit := &ast.SliceLit{Pos: p.expect(lexer.LBracket).Pos}
	p.nested(func() {
		p.skipNewlines()
		for !p.at(lexer.RBracket) {
			lit.Elems = append(lit.Elems, p.expr())
			if p.at(lexer.Comma) {
				p.next()
			} else if !p.at(lexer.RBracket) {
				p.expect(lexer.Newline)
			}
			p.skipNewlines()
		}
	})
	p.expect(lexer.RBracket)
	return lit
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
			x = call
		case lexer.Dot:
			p.next()
			name := p.expect(lexer.Ident)
			x = &ast.Selector{X: x, Name: name.Text, Pos: name.Pos}
		case lexer.Question:
			p.next()
			x = &ast.Try{X: x, Pos: t.Pos}
		case lexer.LBracket:
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
	p.lines(func() {
		pattern := p.pattern()
		p.expect(lexer.FatArrow)
		arms = append(arms, &ast.Arm{Pattern: pattern, Body: p.armBody()})
	})
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
