package amd64

import (
	_ "embed"
	"fmt"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/lexer"
)

//go:embed runtime.asm
var runtime string

type loopLabels struct {
	top string
	end string
}

type Generator struct {
	text    strings.Builder
	data    strings.Builder
	labels  int
	strings map[string]string
	fn      *ast.Func
	ret     string
	loops   []loopLabels
}

func Generate(file *ast.File) string {
	g := &Generator{strings: map[string]string{}}
	for _, fn := range file.Funcs {
		g.function(fn)
	}
	var out strings.Builder
	out.WriteString(runtime)
	out.WriteString("\n\t.section .rodata\n")
	out.WriteString(g.data.String())
	out.WriteString("\n\t.text\n")
	out.WriteString(g.text.String())
	return out.String()
}

func (g *Generator) emit(format string, args ...any) {
	fmt.Fprintf(&g.text, "\t%s\n", fmt.Sprintf(format, args...))
}

func (g *Generator) label(name string) {
	fmt.Fprintf(&g.text, "%s:\n", name)
}

func (g *Generator) newLabel() string {
	g.labels++
	return fmt.Sprintf(".L%d", g.labels)
}

func (g *Generator) function(fn *ast.Func) {
	g.fn = fn
	g.ret = g.newLabel()
	frame := (len(fn.Locals)*8 + 15) &^ 15
	g.label(fmt.Sprintf("fn_%s", fn.Name))
	g.emit("push rbp")
	g.emit("mov rbp, rsp")
	if frame > 0 {
		g.emit("sub rsp, %d", frame)
	}
	g.expr(fn.Body)
	g.label(g.ret)
	g.emit("mov rsp, rbp")
	g.emit("pop rbp")
	g.emit("ret")
}

func slot(l *ast.Local) string {
	if l.Param {
		return fmt.Sprintf("qword ptr [rbp+%d]", 16+8*l.Index)
	}
	return fmt.Sprintf("qword ptr [rbp-%d]", 8*(l.Index+1))
}

func (g *Generator) stringLabel(s string) string {
	if l, ok := g.strings[s]; ok {
		return l
	}
	l := fmt.Sprintf(".Lstr%d", len(g.strings))
	g.strings[s] = l
	fmt.Fprintf(&g.data, "\t.p2align 3\n%s:\n\t.quad %d\n", l, len(s))
	if len(s) == 0 {
		return l
	}
	bytes := make([]string, len(s))
	for i := range len(s) {
		bytes[i] = fmt.Sprintf("%d", s[i])
	}
	fmt.Fprintf(&g.data, "\t.byte %s\n", strings.Join(bytes, ","))
	return l
}

func (g *Generator) block(b *ast.Block) {
	for _, s := range b.Stmts {
		g.stmt(s)
	}
	if b.Tail != nil {
		g.expr(b.Tail)
	}
}

func (g *Generator) stmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.Let:
		g.expr(s.Value)
		g.emit("mov %s, rax", slot(s.Local))
	case *ast.Assign:
		g.assign(s)
	case *ast.ExprStmt:
		g.expr(s.X)
	case *ast.For:
		g.forStmt(s)
	case *ast.Break:
		g.emit("jmp %s", g.loops[len(g.loops)-1].end)
	case *ast.Continue:
		g.emit("jmp %s", g.loops[len(g.loops)-1].top)
	case *ast.Return:
		if s.Value != nil {
			g.expr(s.Value)
		}
		g.emit("jmp %s", g.ret)
	}
}

var compoundOps = map[lexer.Kind]lexer.Kind{
	lexer.PlusAssign: lexer.Plus, lexer.MinusAssign: lexer.Minus,
	lexer.StarAssign: lexer.Star, lexer.SlashAssign: lexer.Slash,
}

func (g *Generator) assign(s *ast.Assign) {
	g.expr(s.Value)
	if op, ok := compoundOps[s.Op]; ok {
		g.emit("mov rcx, rax")
		g.emit("mov rax, %s", slot(s.Local))
		g.arith(op)
	}
	g.emit("mov %s, rax", slot(s.Local))
}

func (g *Generator) forStmt(s *ast.For) {
	l := loopLabels{top: g.newLabel(), end: g.newLabel()}
	g.label(l.top)
	if s.Cond != nil {
		g.expr(s.Cond)
		g.emit("test rax, rax")
		g.emit("jz %s", l.end)
	}
	g.loops = append(g.loops, l)
	g.block(s.Body)
	g.loops = g.loops[:len(g.loops)-1]
	g.emit("jmp %s", l.top)
	g.label(l.end)
}

func (g *Generator) expr(e ast.Expr) {
	switch e := e.(type) {
	case *ast.IntLit:
		g.emit("mov rax, %d", e.Value)
	case *ast.StrLit:
		g.emit("lea rax, [rip+%s]", g.stringLabel(e.Value))
	case *ast.BoolLit:
		g.emit("mov eax, %d", boolInt(e.Value))
	case *ast.Ident:
		g.emit("mov rax, %s", slot(e.Local))
	case *ast.Unary:
		g.unary(e)
	case *ast.Binary:
		g.binary(e)
	case *ast.Call:
		g.call(e)
	case *ast.If:
		g.ifExpr(e)
	case *ast.Block:
		g.block(e)
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (g *Generator) unary(e *ast.Unary) {
	g.expr(e.X)
	if e.Op == lexer.Not {
		g.emit("xor rax, 1")
		return
	}
	g.emit("neg rax")
}

var setcc = map[lexer.Kind]string{
	lexer.Eq: "sete", lexer.NotEq: "setne", lexer.Lt: "setl",
	lexer.LtEq: "setle", lexer.Gt: "setg", lexer.GtEq: "setge",
}

func (g *Generator) binary(e *ast.Binary) {
	if e.Op == lexer.AndAnd || e.Op == lexer.OrOr {
		g.logical(e)
		return
	}
	g.expr(e.Y)
	g.emit("push rax")
	g.expr(e.X)
	g.emit("pop rcx")
	if e.X.TypeOf() == ast.Str {
		g.stringCompare(e.Op)
		return
	}
	if cc, ok := setcc[e.Op]; ok {
		g.emit("cmp rax, rcx")
		g.emit("%s al", cc)
		g.emit("movzx eax, al")
		return
	}
	g.arith(e.Op)
}

func (g *Generator) stringCompare(op lexer.Kind) {
	g.emit("mov rdi, rax")
	g.emit("mov rsi, rcx")
	g.emit("call rt_str_eq")
	if op == lexer.NotEq {
		g.emit("xor rax, 1")
	}
}

func (g *Generator) arith(op lexer.Kind) {
	switch op {
	case lexer.Plus:
		g.emit("add rax, rcx")
	case lexer.Minus:
		g.emit("sub rax, rcx")
	case lexer.Star:
		g.emit("imul rax, rcx")
	case lexer.Slash, lexer.Percent:
		g.divide(op)
	}
}

func (g *Generator) divide(op lexer.Kind) {
	normal := g.newLabel()
	done := g.newLabel()
	g.emit("test rcx, rcx")
	g.emit("jz rt_panic_divzero")
	g.emit("cmp rcx, -1")
	g.emit("jne %s", normal)
	if op == lexer.Slash {
		g.emit("neg rax")
	} else {
		g.emit("xor eax, eax")
	}
	g.emit("jmp %s", done)
	g.label(normal)
	g.emit("cqo")
	g.emit("idiv rcx")
	if op == lexer.Percent {
		g.emit("mov rax, rdx")
	}
	g.label(done)
}

func (g *Generator) logical(e *ast.Binary) {
	done := g.newLabel()
	g.expr(e.X)
	g.emit("test rax, rax")
	if e.Op == lexer.AndAnd {
		g.emit("jz %s", done)
	} else {
		g.emit("jnz %s", done)
	}
	g.expr(e.Y)
	g.label(done)
}

func (g *Generator) call(e *ast.Call) {
	if e.Builtin != "" {
		g.expr(e.Args[0])
		g.emit("mov rdi, rax")
		g.emit("call rt_%s", e.Builtin)
		return
	}
	for i := len(e.Args) - 1; i >= 0; i-- {
		g.expr(e.Args[i])
		g.emit("push rax")
	}
	g.emit("call fn_%s", e.Name)
	if n := len(e.Args); n > 0 {
		g.emit("add rsp, %d", 8*n)
	}
}

func (g *Generator) ifExpr(e *ast.If) {
	otherwise := g.newLabel()
	done := g.newLabel()
	g.expr(e.Cond)
	g.emit("test rax, rax")
	g.emit("jz %s", otherwise)
	g.block(e.Then)
	g.emit("jmp %s", done)
	g.label(otherwise)
	if e.Else != nil {
		g.expr(e.Else)
	}
	g.label(done)
}
