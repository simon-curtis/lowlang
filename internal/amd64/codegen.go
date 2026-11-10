package amd64

import (
	_ "embed"
	"fmt"
	"strings"

	"lowlang/internal/ast"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

//go:embed runtime.asm
var runtime string

type loopLabels struct {
	top   string
	end   string
	depth int
}

type Generator struct {
	text    strings.Builder
	data    strings.Builder
	labels  int
	strings map[string]string
	fn      *ast.Func
	frame   int
	depth   int
	ret     string
	loops   []loopLabels
}

func Generate(file *ast.File) string {
	g := &Generator{strings: map[string]string{}}
	for _, fn := range file.Funcs {
		if fn.Name == "main" {
			g.start(fn)
		}
	}
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

func mem(offset int) string {
	return fmt.Sprintf("qword ptr [rbp%+d]", offset)
}

func (g *Generator) start(main *ast.Func) {
	ret := main.RetType
	g.text.WriteString("\t.globl _start\n_start:\n")
	if ret.Kind == types.ResultKind {
		exit := g.newLabel()
		g.emit("sub rsp, %d", 8*ret.Size())
		g.emit("call fn_main")
		g.emit("cmp qword ptr [rsp], 0")
		g.emit("je %s", exit)
		g.emit("mov rax, [rsp+8]")
		g.emit("lea rcx, [rip+%s]", g.variantNames(ret.Err))
		g.emit("mov rdi, [rcx+8*rax]")
		g.emit("call rt_error")
		g.label(exit)
	} else {
		g.emit("call fn_main")
	}
	g.emit("xor edi, edi")
	g.emit("mov eax, 60")
	g.emit("syscall")
}

func (g *Generator) variantNames(enum *types.Type) string {
	table := g.newLabel()
	names := make([]string, len(enum.Variants))
	for i, v := range enum.Variants {
		names[i] = g.stringLabel(v.Name)
	}
	fmt.Fprintf(&g.data, "\t.p2align 3\n%s:\n\t.quad %s\n", table, strings.Join(names, ","))
	return table
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

func (g *Generator) function(fn *ast.Func) {
	g.fn = fn
	g.ret = g.newLabel()
	g.depth = 0
	g.frame = (fn.FrameWords*8 + 15) &^ 15
	g.label(symbol(fn))
	g.emit("push rbp")
	g.emit("mov rbp, rsp")
	if g.frame > 0 {
		g.emit("sub rsp, %d", g.frame)
	}
	g.expr(fn.Body)
	g.returnValue(fn.Body.Type)
	g.label(g.ret)
	g.emit("mov rsp, rbp")
	g.emit("pop rbp")
	g.emit("ret")
}

func symbol(fn *ast.Func) string {
	return fmt.Sprintf("fn_%s", fn.Name)
}

func (g *Generator) retOffset() int {
	return 16 + 8*g.fn.ParamWords
}

func (g *Generator) returnValue(t *types.Type) {
	ret := g.fn.RetType
	switch {
	case t == types.Never:
	case ret == types.Void:
		g.discard(t)
	case t == ret && t.Scalar():
	case t == ret:
		g.store(t, g.retOffset())
	case ret.Kind == types.ResultKind && t == ret.Ok:
		g.store(t, g.retOffset()+8)
		g.emit("mov %s, 0", mem(g.retOffset()))
	default:
		g.discard(t)
		g.emit("mov %s, 0", mem(g.retOffset()))
	}
}

func (g *Generator) push(operand string) {
	g.emit("push %s", operand)
	g.depth++
}

func (g *Generator) pop(reg string) {
	g.emit("pop %s", reg)
	g.depth--
}

func (g *Generator) reserve(words int) int {
	if words > 0 {
		g.emit("sub rsp, %d", 8*words)
		g.depth += words
	}
	return -(g.frame + 8*g.depth)
}

func (g *Generator) release(words int) {
	if words > 0 {
		g.emit("add rsp, %d", 8*words)
		g.depth -= words
	}
}

func (g *Generator) discard(t *types.Type) {
	g.release(t.StackWords())
}

func (g *Generator) coerce(from, to *types.Type) {
	if to == types.Void {
		g.discard(from)
	}
}

func (g *Generator) unwind(depth int) {
	if g.depth > depth {
		g.emit("add rsp, %d", 8*(g.depth-depth))
	}
}

type address func(word int) string

func frame(offset int) address {
	return func(word int) string { return mem(offset + 8*word) }
}

func (g *Generator) at(l *ast.Local, offset int) address {
	if !l.Ref {
		return frame(l.Offset + offset)
	}
	g.emit("mov rdx, %s", mem(l.Offset))
	return func(word int) string { return fmt.Sprintf("qword ptr [rdx+%d]", offset+8*word) }
}

func (g *Generator) address(l *ast.Local, offset int) {
	if !l.Ref {
		g.emit("lea rax, [rbp%+d]", l.Offset+offset)
		return
	}
	g.emit("mov rax, %s", mem(l.Offset))
	if offset > 0 {
		g.emit("add rax, %d", offset)
	}
}

func (g *Generator) store(t *types.Type, offset int) {
	g.storeTo(t, frame(offset))
}

func (g *Generator) storeTo(t *types.Type, at address) {
	if t.Scalar() {
		g.emit("mov %s, rax", at(0))
		return
	}
	for k := range t.StackWords() {
		g.pop("rax")
		g.emit("mov %s, rax", at(k))
	}
}

func (g *Generator) loadFrom(t *types.Type, at address) {
	if t.Scalar() {
		g.emit("mov rax, %s", at(0))
		return
	}
	for k := t.Size() - 1; k >= 0; k-- {
		g.push(at(k))
	}
}

func (g *Generator) copyWords(words, from, to int) {
	for k := range words {
		g.emit("mov rax, %s", mem(from+8*k))
		g.emit("mov %s, rax", mem(to+8*k))
	}
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
		g.store(s.Local.Type, s.Local.Offset)
	case *ast.Assign:
		g.assign(s)
	case *ast.ExprStmt:
		g.expr(s.X)
		g.discard(s.X.TypeOf())
	case *ast.For:
		g.forStmt(s)
	case *ast.Break:
		l := g.loops[len(g.loops)-1]
		g.unwind(l.depth)
		g.emit("jmp %s", l.end)
	case *ast.Continue:
		l := g.loops[len(g.loops)-1]
		g.unwind(l.depth)
		g.emit("jmp %s", l.top)
	case *ast.Return:
		g.returnStmt(s)
	case *ast.Fail:
		g.expr(s.Value)
		g.store(s.Value.TypeOf(), g.retOffset()+8)
		g.emit("mov %s, 1", mem(g.retOffset()))
		g.emit("jmp %s", g.ret)
	}
}

func (g *Generator) returnStmt(s *ast.Return) {
	t := types.Void
	if s.Value != nil {
		g.expr(s.Value)
		t = s.Value.TypeOf()
	}
	g.returnValue(t)
	g.emit("jmp %s", g.ret)
}

var compoundOps = map[lexer.Kind]lexer.Kind{
	lexer.PlusAssign: lexer.Plus, lexer.MinusAssign: lexer.Minus,
	lexer.StarAssign: lexer.Star, lexer.SlashAssign: lexer.Slash,
}

func (g *Generator) assign(s *ast.Assign) {
	g.expr(s.Value)
	if op, ok := compoundOps[s.Op]; ok {
		g.emit("mov rcx, rax")
		g.emit("mov rax, %s", g.at(s.Root, s.Offset)(0))
		g.arith(op)
	}
	g.storeTo(s.Target.TypeOf(), g.at(s.Root, s.Offset))
}

func (g *Generator) forStmt(s *ast.For) {
	l := loopLabels{top: g.newLabel(), end: g.newLabel(), depth: g.depth}
	g.label(l.top)
	if s.Cond != nil {
		g.expr(s.Cond)
		g.emit("test rax, rax")
		g.emit("jz %s", l.end)
	}
	g.loops = append(g.loops, l)
	g.block(s.Body)
	g.discard(s.Body.Type)
	g.loops = g.loops[:len(g.loops)-1]
	g.emit("jmp %s", l.top)
	g.label(l.end)
	g.depth = l.depth
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
		if e.Variant != nil {
			g.construct(e.Type, e.Variant, nil)
			return
		}
		g.loadFrom(e.Type, g.at(e.Local, 0))
	case *ast.Selector:
		g.selector(e)
	case *ast.StructLit:
		g.structLit(e)
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
	case *ast.Match:
		g.expr(e.X)
		g.store(e.Temp.Type, e.Temp.Offset)
		g.dispatch(e.Arms, e.Temp, e.Type)
	case *ast.Try:
		g.try(e)
	case *ast.Catch:
		g.catch(e)
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
	g.expr(e.X)
	g.push("rax")
	g.expr(e.Y)
	g.emit("mov rcx, rax")
	g.pop("rax")
	if e.X.TypeOf() == types.Str {
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
	switch {
	case e.Builtin != "":
		g.expr(e.Args[0])
		g.emit("mov rdi, rax")
		g.emit("call rt_%s", e.Builtin)
	case e.Variant != nil:
		g.construct(e.Type, e.Variant, e.Args)
	default:
		g.callFunc(e)
	}
}

func (g *Generator) callFunc(e *ast.Call) {
	fn := e.Target
	offset := g.reserve(fn.RetType.StackWords() + fn.ParamWords)
	for i, arg := range e.Args {
		if r, ok := arg.(*ast.Ref); ok {
			g.address(r.Root, r.Offset)
			g.emit("mov %s, rax", mem(offset))
		} else {
			g.expr(arg)
			g.store(fn.ParamTypes[i], offset)
		}
		offset += 8 * fn.ParamSize(i)
	}
	g.emit("call %s", symbol(fn))
	g.release(fn.ParamWords)
}

func (g *Generator) construct(enum *types.Type, v *types.Variant, args []ast.Expr) {
	if enum.Scalar() {
		g.emit("mov eax, %d", v.Tag)
		return
	}
	base := g.reserve(enum.Size())
	g.emit("mov %s, %d", mem(base), v.Tag)
	for i, arg := range args {
		g.expr(arg)
		g.store(v.Fields[i], base+8*v.Offsets[i])
	}
}

func (g *Generator) selector(e *ast.Selector) {
	if e.Variant != nil {
		g.construct(e.Type, e.Variant, nil)
		return
	}
	if root, offset, ok := place(e); ok {
		g.loadFrom(e.Type, g.at(root, offset))
		return
	}
	g.expr(e.X)
	g.extract(e.X.TypeOf(), e.Field.Offset, e.Type)
}

func place(e ast.Expr) (*ast.Local, int, bool) {
	switch e := e.(type) {
	case *ast.Ident:
		if e.Local != nil {
			return e.Local, 0, true
		}
	case *ast.Selector:
		if e.Field == nil {
			return nil, 0, false
		}
		if root, offset, ok := place(e.X); ok {
			return root, offset + 8*e.Field.Offset, true
		}
	}
	return nil, 0, false
}

func (g *Generator) extract(whole *types.Type, offset int, part *types.Type) {
	if whole.Scalar() {
		return
	}
	n := whole.Size()
	size := part.Size()
	if part.Scalar() {
		g.emit("mov rax, [rsp+%d]", 8*offset)
		g.release(n)
		return
	}
	for k := size - 1; k >= 0; k-- {
		g.emit("mov rax, [rsp+%d]", 8*(offset+k))
		g.emit("mov [rsp+%d], rax", 8*(n-size+k))
	}
	g.release(n - size)
}

func (g *Generator) structLit(e *ast.StructLit) {
	if e.Type.Scalar() {
		g.expr(e.Fields[0].Value)
		return
	}
	base := g.reserve(e.Type.Size())
	for _, init := range e.Fields {
		g.expr(init.Value)
		g.store(init.Field.Type, base+8*init.Field.Offset)
	}
}

func (g *Generator) ifExpr(e *ast.If) {
	depth := g.depth
	otherwise := g.newLabel()
	done := g.newLabel()
	g.expr(e.Cond)
	g.emit("test rax, rax")
	g.emit("jz %s", otherwise)
	g.block(e.Then)
	g.coerce(e.Then.Type, e.Type)
	g.emit("jmp %s", done)
	g.label(otherwise)
	g.depth = depth
	if e.Else != nil {
		g.expr(e.Else)
		g.coerce(e.Else.TypeOf(), e.Type)
	}
	g.label(done)
	g.depth = depth + e.Type.StackWords()
}

func (g *Generator) dispatch(arms []*ast.Arm, subject *ast.Local, result *types.Type) {
	depth := g.depth
	done := g.newLabel()
	targets := make([]string, len(arms))
	g.emit("mov rax, %s", mem(subject.Offset))
	for i, arm := range arms {
		targets[i] = g.newLabel()
		if arm.Pattern.Variant == nil {
			g.emit("jmp %s", targets[i])
			continue
		}
		g.emit("cmp rax, %d", arm.Pattern.Variant.Tag)
		g.emit("je %s", targets[i])
	}
	g.emit("ud2")
	for i, arm := range arms {
		g.label(targets[i])
		g.depth = depth
		g.bind(arm.Pattern, subject)
		g.expr(arm.Body)
		g.coerce(arm.Body.TypeOf(), result)
		g.emit("jmp %s", done)
	}
	g.label(done)
	g.depth = depth + result.StackWords()
}

func (g *Generator) bind(p *ast.Pattern, subject *ast.Local) {
	for i, b := range p.Bindings {
		if b.Local == nil {
			continue
		}
		from := subject.Offset + 8*p.Variant.Offsets[i]
		g.copyWords(b.Local.Type.Size(), from, b.Local.Offset)
	}
}

func (g *Generator) try(e *ast.Try) {
	result := e.X.TypeOf()
	ok := g.newLabel()
	g.expr(e.X)
	g.emit("cmp qword ptr [rsp], 0")
	g.emit("je %s", ok)
	for k := range result.Err.Size() {
		g.emit("mov rax, [rsp+%d]", 8+8*k)
		g.emit("mov %s, rax", mem(g.retOffset()+8+8*k))
	}
	g.emit("mov %s, 1", mem(g.retOffset()))
	g.emit("jmp %s", g.ret)
	g.label(ok)
	g.extract(result, 1, result.Ok)
}

func (g *Generator) catch(e *ast.Catch) {
	result := e.X.TypeOf()
	failed := g.newLabel()
	done := g.newLabel()
	g.expr(e.X)
	depth := g.depth
	g.emit("cmp qword ptr [rsp], 0")
	g.emit("jne %s", failed)
	g.extract(result, 1, result.Ok)
	g.emit("jmp %s", done)
	g.label(failed)
	g.depth = depth
	if e.Fallback != nil {
		g.release(result.Size())
		g.expr(e.Fallback)
	} else {
		for k := range result.Err.Size() {
			g.emit("mov rax, [rsp+%d]", 8+8*k)
			g.emit("mov %s, rax", mem(e.Temp.Offset+8*k))
		}
		g.release(result.Size())
		g.dispatch(e.Arms, e.Temp, e.Type)
	}
	g.label(done)
	g.depth = depth - result.Size() + e.Type.StackWords()
}
