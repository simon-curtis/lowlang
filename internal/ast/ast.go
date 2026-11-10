package ast

import (
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

type File struct {
	Enums []*EnumDecl
	Funcs []*Func
}

type TypeName struct {
	Name string
	Pos  lexer.Pos
}

type EnumDecl struct {
	Name     string
	Variants []*VariantDecl
	Pos      lexer.Pos
	Type     *types.Type
}

type VariantDecl struct {
	Name   string
	Fields []*TypeName
	Pos    lexer.Pos
}

type Param struct {
	Name     string
	TypeName *TypeName
	Pos      lexer.Pos
}

type Func struct {
	Name       string
	Params     []*Param
	Result     *TypeName
	Error      *TypeName
	Body       *Block
	Pos        lexer.Pos
	ParamTypes []*types.Type
	RetType    *types.Type
	Locals     []*Local
	FrameWords int
	ParamWords int
}

type Local struct {
	Name    string
	Type    *types.Type
	Mutable bool
	Offset  int
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
	TypeOf() *types.Type
	expr()
}

type Typed struct {
	Type *types.Type
}

func (t *Typed) TypeOf() *types.Type {
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

type Fail struct {
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
	Name    string
	Pos     lexer.Pos
	Local   *Local
	Variant *types.Variant
}

type Selector struct {
	Typed
	X       Expr
	Name    string
	Pos     lexer.Pos
	Variant *types.Variant
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
	Fn      Expr
	Args    []Expr
	Pos     lexer.Pos
	Target  *Func
	Builtin string
	Variant *types.Variant
}

type If struct {
	Typed
	Cond Expr
	Then *Block
	Else Expr
	Pos  lexer.Pos
}

type Pattern struct {
	Name     string
	Bindings []*Binding
	Pos      lexer.Pos
	Variant  *types.Variant
}

type Binding struct {
	Name  string
	Pos   lexer.Pos
	Local *Local
}

type Arm struct {
	Pattern *Pattern
	Body    Expr
}

type Match struct {
	Typed
	X    Expr
	Arms []*Arm
	Pos  lexer.Pos
	Temp *Local
}

type Try struct {
	Typed
	X   Expr
	Pos lexer.Pos
}

type Catch struct {
	Typed
	X        Expr
	Arms     []*Arm
	Fallback Expr
	Pos      lexer.Pos
	Temp     *Local
}

func (n *Block) Position() lexer.Pos    { return n.Pos }
func (n *Let) Position() lexer.Pos      { return n.Pos }
func (n *Assign) Position() lexer.Pos   { return n.Pos }
func (n *ExprStmt) Position() lexer.Pos { return n.X.Position() }
func (n *For) Position() lexer.Pos      { return n.Pos }
func (n *Break) Position() lexer.Pos    { return n.Pos }
func (n *Continue) Position() lexer.Pos { return n.Pos }
func (n *Return) Position() lexer.Pos   { return n.Pos }
func (n *Fail) Position() lexer.Pos     { return n.Pos }
func (n *IntLit) Position() lexer.Pos   { return n.Pos }
func (n *StrLit) Position() lexer.Pos   { return n.Pos }
func (n *BoolLit) Position() lexer.Pos  { return n.Pos }
func (n *Ident) Position() lexer.Pos    { return n.Pos }
func (n *Selector) Position() lexer.Pos { return n.Pos }
func (n *Unary) Position() lexer.Pos    { return n.Pos }
func (n *Binary) Position() lexer.Pos   { return n.Pos }
func (n *Call) Position() lexer.Pos     { return n.Pos }
func (n *If) Position() lexer.Pos       { return n.Pos }
func (n *Match) Position() lexer.Pos    { return n.Pos }
func (n *Try) Position() lexer.Pos      { return n.Pos }
func (n *Catch) Position() lexer.Pos    { return n.Pos }

func (*Let) stmt()      {}
func (*Assign) stmt()   {}
func (*ExprStmt) stmt() {}
func (*For) stmt()      {}
func (*Break) stmt()    {}
func (*Continue) stmt() {}
func (*Return) stmt()   {}
func (*Fail) stmt()     {}

func (*Block) expr()    {}
func (*IntLit) expr()   {}
func (*StrLit) expr()   {}
func (*BoolLit) expr()  {}
func (*Ident) expr()    {}
func (*Selector) expr() {}
func (*Unary) expr()    {}
func (*Binary) expr()   {}
func (*Call) expr()     {}
func (*If) expr()       {}
func (*Match) expr()    {}
func (*Try) expr()      {}
func (*Catch) expr()    {}
