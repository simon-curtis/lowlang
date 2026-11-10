package ast

import "lowlang/internal/lexer"

type Type int

const (
	Unknown Type = iota
	Void
	Never
	I64
	Bool
	Str
)

var typeNames = map[Type]string{Unknown: "unknown", Void: "void", Never: "never", I64: "i64", Bool: "bool", Str: "str"}

func (t Type) String() string {
	return typeNames[t]
}

type File struct {
	Funcs []*Func
}

type Param struct {
	Name     string
	TypeName *TypeName
	Pos      lexer.Pos
}

type TypeName struct {
	Name string
	Pos  lexer.Pos
}

type Func struct {
	Name    string
	Params  []*Param
	Result  *TypeName
	Body    *Block
	Pos     lexer.Pos
	Locals  []*Local
	RetType Type
}

type Local struct {
	Name    string
	Type    Type
	Mutable bool
	Param   bool
	Index   int
}

type Node interface {
	Position() lexer.Pos
}

type Stmt interface {
	Node
	stmt()
}

type Expr interface {
	Node
	TypeOf() Type
	expr()
}

type Typed struct {
	Type Type
}

func (t *Typed) TypeOf() Type {
	return t.Type
}

type Block struct {
	Typed
	Stmts []Stmt
	Tail  Expr
	Pos   lexer.Pos
}

type Let struct {
	Name     string
	Mutable  bool
	TypeName *TypeName
	Value    Expr
	Pos      lexer.Pos
	Local    *Local
}

type Assign struct {
	Name  string
	Op    lexer.Kind
	Value Expr
	Pos   lexer.Pos
	Local *Local
}

type ExprStmt struct {
	X Expr
}

type For struct {
	Cond Expr
	Body *Block
	Pos  lexer.Pos
}

type Break struct {
	Pos lexer.Pos
}

type Continue struct {
	Pos lexer.Pos
}

type Return struct {
	Value Expr
	Pos   lexer.Pos
}

type IntLit struct {
	Typed
	Value int64
	Pos   lexer.Pos
}

type StrLit struct {
	Typed
	Value string
	Pos   lexer.Pos
}

type BoolLit struct {
	Typed
	Value bool
	Pos   lexer.Pos
}

type Ident struct {
	Typed
	Name  string
	Pos   lexer.Pos
	Local *Local
}

type Unary struct {
	Typed
	Op  lexer.Kind
	X   Expr
	Pos lexer.Pos
}

type Binary struct {
	Typed
	Op  lexer.Kind
	X   Expr
	Y   Expr
	Pos lexer.Pos
}

type Call struct {
	Typed
	Name    string
	Args    []Expr
	Pos     lexer.Pos
	Target  *Func
	Builtin string
}

type If struct {
	Typed
	Cond Expr
	Then *Block
	Else Expr
	Pos  lexer.Pos
}

func (n *Block) Position() lexer.Pos    { return n.Pos }
func (n *Let) Position() lexer.Pos      { return n.Pos }
func (n *Assign) Position() lexer.Pos   { return n.Pos }
func (n *ExprStmt) Position() lexer.Pos { return n.X.Position() }
func (n *For) Position() lexer.Pos      { return n.Pos }
func (n *Break) Position() lexer.Pos    { return n.Pos }
func (n *Continue) Position() lexer.Pos { return n.Pos }
func (n *Return) Position() lexer.Pos   { return n.Pos }
func (n *IntLit) Position() lexer.Pos   { return n.Pos }
func (n *StrLit) Position() lexer.Pos   { return n.Pos }
func (n *BoolLit) Position() lexer.Pos  { return n.Pos }
func (n *Ident) Position() lexer.Pos    { return n.Pos }
func (n *Unary) Position() lexer.Pos    { return n.Pos }
func (n *Binary) Position() lexer.Pos   { return n.Pos }
func (n *Call) Position() lexer.Pos     { return n.Pos }
func (n *If) Position() lexer.Pos       { return n.Pos }

func (*Let) stmt()      {}
func (*Assign) stmt()   {}
func (*ExprStmt) stmt() {}
func (*For) stmt()      {}
func (*Break) stmt()    {}
func (*Continue) stmt() {}
func (*Return) stmt()   {}

func (*Block) expr()   {}
func (*IntLit) expr()  {}
func (*StrLit) expr()  {}
func (*BoolLit) expr() {}
func (*Ident) expr()   {}
func (*Unary) expr()   {}
func (*Binary) expr()  {}
func (*Call) expr()    {}
func (*If) expr()      {}
