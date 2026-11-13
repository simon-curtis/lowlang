package ast

import (
	"iter"

	"lowlang/internal/builtin"
	"lowlang/internal/diag"
	"lowlang/internal/lexer"
	"lowlang/internal/types"
)

type File struct {
	Path    string
	Module  string
	Imports []*Import
	Externs []*Extern
	Enums   []*EnumDecl
	Structs []*StructDecl
	Consts  []*Const
	Funcs   []*Func
}

type Const struct {
	Name     string
	Doc      string
	Pub      bool
	File     *File
	TypeName *TypeName
	Value    Expr
	Text     string
	Folded   Expr
	Pos      diag.Pos
}

func ConstOf(e Expr) *Const {
	switch e := e.(type) {
	case *Ident:
		return e.Const
	case *Selector:
		return e.Const
	}
	return nil
}

func Literal(e Expr) Expr {
	if k := ConstOf(e); k != nil {
		return k.Folded
	}
	return e
}

type Extern struct {
	Library string
	Funcs   []*Func
	Pos     diag.Pos
}

type Defer struct {
	Body Stmt
	Pos  diag.Pos
}

type Spawn struct {
	Call *Call
	Body *Block
	Pos  diag.Pos
}

type Send struct {
	Chan  Expr
	Value Expr
	Pos   diag.Pos
}

type SelectKind int

const (
	RecvCase SelectKind = iota
	SendCase
	AfterCase
	ElseCase
)

type SelectCase struct {
	Kind  SelectKind
	Chan  Expr
	Value Expr
	Name  string
	Local *Local
	Body  Expr
	Pos   diag.Pos
}

type Select struct {
	Typed
	Cases []*SelectCase
	Pos   diag.Pos
}

type Import struct {
	Alias string
	Path  string
	Pos   diag.Pos
}

func (f *File) Imported(alias string) (string, bool) {
	for _, imp := range f.Imports {
		if imp.Alias == alias {
			return imp.Path, true
		}
	}
	return "", false
}

type TypeName struct {
	Module   string
	Name     string
	Pointer  bool
	Slice    bool
	Optional bool
	Map      bool
	Key      *TypeName
	Args     []*TypeName
	Func     bool
	CFunc    bool
	Params   []*TypeName
	Muts     []bool
	Result   *TypeName
	Error    *TypeName
	Elem     *TypeName
	Pos      diag.Pos
}

type TypeParam struct {
	Name        string
	Constraints []string
	Pos         diag.Pos
}

type EnumDecl struct {
	Name       string
	Doc        string
	TypeParams []*TypeParam
	Pub        bool
	File       *File
	Variants   []*VariantDecl
	Pos        diag.Pos
	Type       *types.Type
}

type VariantDecl struct {
	Name   string
	Doc    string
	Fields []*TypeName
	Labels []string
	Index  *IntLit
	Pos    diag.Pos
}

type StructDecl struct {
	Name       string
	Doc        string
	Drop       Stmt
	TypeParams []*TypeParam
	C          bool
	Pub        bool
	File       *File
	Fields     []*FieldDecl
	Pos        diag.Pos
	Type       *types.Type
}

type FieldDecl struct {
	Name     string
	Doc      string
	Pub      bool
	TypeName *TypeName
	Default  Expr
	Pos      diag.Pos
}

type Param struct {
	Name     string
	Mut      bool
	TypeName *TypeName
	Pos      diag.Pos
}

type Func struct {
	Name        string
	Doc         string
	TypeParams  []*TypeParam
	Template    *Func
	TypeArgs    []*types.Type
	Pub         bool
	File        *File
	Extern      *Extern
	Symbol      string
	Variadic    bool
	Closure     *Closure
	AsValue     bool
	Callback    bool
	Params      []*Param
	Result      *TypeName
	Error       *TypeName
	Body        *Block
	Pos         diag.Pos
	ParamTypes  []*types.Type
	RetType     *types.Type
	ParamLocals []*Local
	Bound       map[string]*types.Type
	Trusted     bool
	Derived     bool
}

func (f *Func) Declared() bool {
	return f.Template == nil && f.Closure == nil
}

func (f *Func) Source() *Func {
	if f.Template != nil {
		return f.Template
	}
	return f
}

func (f *File) Declared() iter.Seq[*Func] {
	return func(yield func(*Func) bool) {
		for _, e := range f.Externs {
			for _, fn := range e.Funcs {
				if !yield(fn) {
					return
				}
			}
		}
		for _, fn := range f.Funcs {
			if fn.Declared() && !yield(fn) {
				return
			}
		}
	}
}

type Local struct {
	Name      string
	Type      *types.Type
	Mutable   bool
	Param     bool
	Ref       bool
	Addressed bool
	Captured  bool
	Assigned  bool
	Owner     *Func
	Flag      *Local
}

type Node interface {
	Position() diag.Pos
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
	Pos   diag.Pos
}

type Let struct {
	Name     string
	Pattern  *StructPattern
	Mutable  bool
	Using    bool
	TypeName *TypeName
	Value    Expr
	Pos      diag.Pos
	Local    *Local
	Release  Stmt
}

type Assign struct {
	Target  Expr
	Op      lexer.Kind
	Value   Expr
	Pos     diag.Pos
	Entry   *Index
	Release Stmt
	Flag    *Local
	Splat   bool
}

type ExprStmt struct {
	X Expr
}

type For struct {
	Label string
	Cond  Expr
	Body  *Block
	Pos   diag.Pos
}

type Break struct {
	Label string
	Depth int
	Pos   diag.Pos
}

type Continue struct {
	Label string
	Depth int
	Pos   diag.Pos
}

type Return struct {
	Value Expr
	Pos   diag.Pos
}

type Fail struct {
	Value Expr
	Pos   diag.Pos
}

type IntLit struct {
	Typed
	Value int64
	Char  bool
	Wide  bool
	Pos   diag.Pos
}

type FloatLit struct {
	Typed
	Value float64
	Pos   diag.Pos
}

type StrLit struct {
	Typed
	Value string
	Pos   diag.Pos
}

type BoolLit struct {
	Typed
	Value bool
	Pos   diag.Pos
}

type Ident struct {
	Typed
	Name    string
	Pos     diag.Pos
	Local   *Local
	Variant *types.Variant
	Null    bool
	Func    *Func
	Const   *Const
	Moved   bool
}

type Selector struct {
	Typed
	X         Expr
	Name      string
	Pos       diag.Pos
	Variant   *types.Variant
	Field     *types.Field
	Reference *types.Field
	Property  builtin.ID
	Func      *Func
	Const     *Const
}

type Closure struct {
	Typed
	Params   []*Param
	Result   *TypeName
	Error    *TypeName
	Body     *Block
	Inline   bool
	Pos      diag.Pos
	Func     *Func
	Captures []*Local
}

type Instance struct {
	Typed
	X        Expr
	Types    []*TypeName
	Fallback Expr
	Pos      diag.Pos
}

type Index struct {
	Typed
	X     Expr
	Index Expr
	Pos   diag.Pos
}

type Slicing struct {
	Typed
	X   Expr
	Lo  Expr
	Hi  Expr
	Pos diag.Pos
}

type SliceLit struct {
	Typed
	Elems    []Expr
	ReadOnly bool
	Pos      diag.Pos
}

type MapLit struct {
	Typed
	Keys     []Expr
	Values   []Expr
	ReadOnly bool
	Pos      diag.Pos
}

type Interp struct {
	Typed
	Parts   []Expr
	Specs   []*Spec
	Renders []*Func
	Pos     diag.Pos
}

type ForIn struct {
	Label      string
	Key        string
	Value      string
	Pattern    *StructPattern
	KeyLocal   *Local
	ValueLocal *Local
	Iter       Expr
	End        Expr
	Body       *Block
	Pos        diag.Pos
}

type Ref struct {
	Typed
	X   Expr
	Pos diag.Pos
}

type Receiver struct {
	Typed
	X Expr
}

type StructLit struct {
	Typed
	Module   string
	Name     string
	TypeArgs []*TypeName
	Fields   []*FieldInit
	Pos      diag.Pos
}

type FieldForm uint8

const (
	FieldWritten FieldForm = iota
	FieldShorthand
	FieldImplied
)

type FieldInit struct {
	Name  string
	Value Expr
	Pos   diag.Pos
	Form  FieldForm
	Field *types.Field
}

type Unary struct {
	Typed
	Op  lexer.Kind
	X   Expr
	Pos diag.Pos
}

type Binary struct {
	Typed
	Op     lexer.Kind
	X      Expr
	Y      Expr
	SplatX bool
	SplatY bool
	Pos    diag.Pos
}

type Call struct {
	Typed
	Fn        Expr
	Args      []Expr
	Pos       diag.Pos
	Prepended bool
	Target    *Func
	Render    *Func
	Builtin   builtin.ID
	Variant   *types.Variant
	Embed     string
	Size      int64
	Digest    uint64
	Dir       *MapLit
}

type If struct {
	Typed
	Cond Expr
	Then *Block
	Else Expr
	Pos  diag.Pos
}

type Pattern struct {
	Name     string
	Bindings []*Binding
	Test     Expr
	Pos      diag.Pos
	Variant  *types.Variant
	Local    *Local
}

type StructPattern struct {
	Fields []*FieldPattern
	Pos    diag.Pos
	Type   *types.Type
	Temp   *Local
}

type FieldPattern struct {
	Name      string
	Pos       diag.Pos
	Shorthand bool
	Bind      *Binding
	Nested    *StructPattern
	Field     *types.Field
	Source    Expr
}

type Subject struct {
	Typed
	Pos diag.Pos
}

type Binding struct {
	Name  string
	Pos   diag.Pos
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
	Pos  diag.Pos
}

type Try struct {
	Typed
	X   Expr
	Pos diag.Pos
}

type Catch struct {
	Typed
	X    Expr
	Arms []*Arm
	Map  *Call
	Pos  diag.Pos
}

type Or struct {
	Typed
	X        Expr
	Fallback Expr
	Pos      diag.Pos
}

type When struct {
	Typed
	X    Expr
	Cond Expr
	Pos  diag.Pos
}

type Region struct {
	Typed
	Body *Block
	Copy *Func
	Pos  diag.Pos
}

func (n *File) Position() diag.Pos          { return diag.Start(n.Path) }
func (n *Import) Position() diag.Pos        { return n.Pos }
func (n *Extern) Position() diag.Pos        { return n.Pos }
func (n *Func) Position() diag.Pos          { return n.Pos }
func (n *Param) Position() diag.Pos         { return n.Pos }
func (n *TypeParam) Position() diag.Pos     { return n.Pos }
func (n *TypeName) Position() diag.Pos      { return n.Pos }
func (n *StructDecl) Position() diag.Pos    { return n.Pos }
func (n *EnumDecl) Position() diag.Pos      { return n.Pos }
func (n *Const) Position() diag.Pos         { return n.Pos }
func (n *FieldDecl) Position() diag.Pos     { return n.Pos }
func (n *VariantDecl) Position() diag.Pos   { return n.Pos }
func (n *FieldInit) Position() diag.Pos     { return n.Pos }
func (n *SelectCase) Position() diag.Pos    { return n.Pos }
func (n *Pattern) Position() diag.Pos       { return n.Pos }
func (n *Binding) Position() diag.Pos       { return n.Pos }
func (n *Block) Position() diag.Pos         { return n.Pos }
func (n *Let) Position() diag.Pos           { return n.Pos }
func (n *StructPattern) Position() diag.Pos { return n.Pos }
func (n *FieldPattern) Position() diag.Pos  { return n.Pos }
func (n *Assign) Position() diag.Pos        { return n.Pos }
func (n *ExprStmt) Position() diag.Pos      { return n.X.Position() }
func (n *For) Position() diag.Pos           { return n.Pos }
func (n *Break) Position() diag.Pos         { return n.Pos }
func (n *Continue) Position() diag.Pos      { return n.Pos }
func (n *Return) Position() diag.Pos        { return n.Pos }
func (n *Fail) Position() diag.Pos          { return n.Pos }
func (n *Defer) Position() diag.Pos         { return n.Pos }
func (n *Spawn) Position() diag.Pos         { return n.Pos }
func (n *Send) Position() diag.Pos          { return n.Pos }
func (n *Select) Position() diag.Pos        { return n.Pos }
func (n *Subject) Position() diag.Pos       { return n.Pos }
func (n *Receiver) Position() diag.Pos      { return n.X.Position() }
func (n *IntLit) Position() diag.Pos        { return n.Pos }
func (n *FloatLit) Position() diag.Pos      { return n.Pos }
func (n *StrLit) Position() diag.Pos        { return n.Pos }
func (n *BoolLit) Position() diag.Pos       { return n.Pos }
func (n *Ident) Position() diag.Pos         { return n.Pos }
func (n *Selector) Position() diag.Pos      { return n.Pos }
func (n *StructLit) Position() diag.Pos     { return n.Pos }
func (n *Ref) Position() diag.Pos           { return n.Pos }
func (n *Index) Position() diag.Pos         { return n.Pos }
func (n *Instance) Position() diag.Pos      { return n.Pos }
func (n *Closure) Position() diag.Pos       { return n.Pos }
func (n *Slicing) Position() diag.Pos       { return n.Pos }
func (n *SliceLit) Position() diag.Pos      { return n.Pos }
func (n *Interp) Position() diag.Pos        { return n.Pos }
func (n *MapLit) Position() diag.Pos        { return n.Pos }
func (n *ForIn) Position() diag.Pos         { return n.Pos }
func (n *Unary) Position() diag.Pos         { return n.Pos }
func (n *Binary) Position() diag.Pos        { return n.Pos }
func (n *Call) Position() diag.Pos          { return n.Pos }
func (n *If) Position() diag.Pos            { return n.Pos }
func (n *Match) Position() diag.Pos         { return n.Pos }
func (n *Try) Position() diag.Pos           { return n.Pos }
func (n *Catch) Position() diag.Pos         { return n.Pos }
func (n *Or) Position() diag.Pos            { return n.Pos }
func (n *When) Position() diag.Pos          { return n.Pos }
func (n *Region) Position() diag.Pos        { return n.Pos }

func (*Let) stmt()      {}
func (*Assign) stmt()   {}
func (*ExprStmt) stmt() {}
func (*For) stmt()      {}
func (*Break) stmt()    {}
func (*Continue) stmt() {}
func (*Return) stmt()   {}
func (*Fail) stmt()     {}
func (*Defer) stmt()    {}
func (*Spawn) stmt()    {}
func (*Send) stmt()     {}
func (*ForIn) stmt()    {}

func (*Block) expr()     {}
func (*IntLit) expr()    {}
func (*FloatLit) expr()  {}
func (*StrLit) expr()    {}
func (*BoolLit) expr()   {}
func (*Ident) expr()     {}
func (*Selector) expr()  {}
func (*StructLit) expr() {}
func (*Ref) expr()       {}
func (*Index) expr()     {}
func (*Instance) expr()  {}
func (*Closure) expr()   {}
func (*Slicing) expr()   {}
func (*SliceLit) expr()  {}
func (*Interp) expr()    {}
func (*MapLit) expr()    {}
func (*Unary) expr()     {}
func (*Binary) expr()    {}
func (*Call) expr()      {}
func (*If) expr()        {}
func (*Match) expr()     {}
func (*Select) expr()    {}
func (*Subject) expr()   {}
func (*Receiver) expr()  {}
func (*Try) expr()       {}
func (*Catch) expr()     {}
func (*Or) expr()        {}
func (*When) expr()      {}
func (*Region) expr()    {}
