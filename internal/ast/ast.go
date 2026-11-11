package ast

import (
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
	Funcs   []*Func
}

type Extern struct {
	Library string
	Funcs   []*Func
	Pos     lexer.Pos
}

type Defer struct {
	Body Stmt
	Pos  lexer.Pos
}

type Spawn struct {
	Call *Call
	Body *Block
	Pos  lexer.Pos
}

type Send struct {
	Chan  Expr
	Value Expr
	Pos   lexer.Pos
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
	Pos   lexer.Pos
}

type Select struct {
	Typed
	Cases []*SelectCase
	Pos   lexer.Pos
}

func fieldWidth(e *Selector) *types.Type {
	if types.StructOf(e.X.TypeOf()).C {
		return e.Field.Type.Width()
	}
	return nil
}

type Import struct {
	Alias string
	Path  string
	Pos   lexer.Pos
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
	Raw      bool
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
	Pos      lexer.Pos
}

type Place struct {
	Local  *Local
	Ptr    Expr
	Index  *Index
	Offset int
	Width  *types.Type
	Tail   int
}

func PlaceOf(e Expr) (*Place, bool) {
	switch e := e.(type) {
	case *Receiver:
		return PlaceOf(e.X)
	case *Ident:
		if e.Local != nil {
			return &Place{Local: e.Local}, true
		}
	case *Selector:
		if e.Field == nil {
			return nil, false
		}
		width := fieldWidth(e)
		if e.X.TypeOf().Kind == types.PointerKind {
			return &Place{Ptr: e.X, Offset: e.Field.Bytes, Width: width, Tail: e.Field.Type.Tail()}, true
		}
		p, ok := PlaceOf(e.X)
		if !ok {
			return nil, false
		}
		return &Place{Local: p.Local, Ptr: p.Ptr, Index: p.Index, Offset: p.Offset + e.Field.Bytes, Width: width, Tail: e.Field.Type.Tail()}, true
	case *Index:
		if e.X.TypeOf().Kind == types.SliceKind {
			return &Place{Index: e, Tail: e.Type.Tail()}, true
		}
	case *Unary:
		if e.Op == lexer.Star {
			return &Place{Ptr: e.X, Tail: e.Type.Tail()}, true
		}
	}
	return nil, false
}

type TypeParam struct {
	Name        string
	Constraints []string
	Pos         lexer.Pos
}

type EnumDecl struct {
	Name       string
	TypeParams []*TypeParam
	Pub        bool
	File       *File
	Variants   []*VariantDecl
	Pos        lexer.Pos
	Type       *types.Type
}

type VariantDecl struct {
	Name   string
	Fields []*TypeName
	Labels []string
	Pos    lexer.Pos
}

type StructDecl struct {
	Name       string
	Drop       Stmt
	TypeParams []*TypeParam
	C          bool
	Pub        bool
	File       *File
	Fields     []*FieldDecl
	Pos        lexer.Pos
	Type       *types.Type
}

type FieldDecl struct {
	Name     string
	Pub      bool
	TypeName *TypeName
	Default  Expr
	Pos      lexer.Pos
}

type Param struct {
	Name     string
	Mut      bool
	TypeName *TypeName
	Pos      lexer.Pos
}

type Func struct {
	Name        string
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
	Pos         lexer.Pos
	ParamTypes  []*types.Type
	RetType     *types.Type
	ParamLocals []*Local
	Bound       map[string]*types.Type
	Trusted     bool
}

func (f *Func) ParamSize(i int) int {
	if f.Params[i].Mut {
		return 1
	}
	return f.ParamTypes[i].Size()
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

func (l *Local) Boxed() bool {
	if !l.Captured {
		return false
	}
	return l.Assigned || l.Addressed || l.Type.Kind == types.VectorKind || (l.Type.Kind == types.StructKind && l.Type.C)
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
	Using    bool
	Spawned  bool
	TypeName *TypeName
	Value    Expr
	Pos      lexer.Pos
	Local    *Local
	Flag     *Local
	Release  Stmt
}

type Assign struct {
	Target  Expr
	Op      lexer.Kind
	Value   Expr
	Pos     lexer.Pos
	Place   *Place
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
	Pos   lexer.Pos
}

type Break struct {
	Label string
	Depth int
	Pos   lexer.Pos
}

type Continue struct {
	Label string
	Depth int
	Pos   lexer.Pos
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
	Char  bool
	Wide  bool
	Pos   lexer.Pos
}

type FloatLit struct {
	Typed
	Value float64
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
	Null    bool
	Func    *Func
	Moved   bool
}

type Selector struct {
	Typed
	X         Expr
	Name      string
	Pos       lexer.Pos
	Variant   *types.Variant
	Field     *types.Field
	Reference *types.Field
	Length    bool
	Func      *Func
}

type Closure struct {
	Typed
	Params   []*Param
	Result   *TypeName
	Error    *TypeName
	Body     *Block
	Pos      lexer.Pos
	Func     *Func
	Captures []*Local
}

type Instance struct {
	Typed
	X        Expr
	Types    []*TypeName
	Fallback Expr
	Pos      lexer.Pos
}

type Index struct {
	Typed
	X     Expr
	Index Expr
	Pos   lexer.Pos
}

type Slicing struct {
	Typed
	X   Expr
	Lo  Expr
	Hi  Expr
	Pos lexer.Pos
}

type SliceLit struct {
	Typed
	Elems    []Expr
	ReadOnly bool
	Pos      lexer.Pos
}

type MapLit struct {
	Typed
	Keys     []Expr
	Values   []Expr
	ReadOnly bool
	Pos      lexer.Pos
}

type Interp struct {
	Typed
	Parts []Expr
	Pos   lexer.Pos
}

type ForIn struct {
	Label      string
	Key        string
	Value      string
	KeyLocal   *Local
	ValueLocal *Local
	Iter       Expr
	End        Expr
	Body       *Block
	Pos        lexer.Pos
}

type Ref struct {
	Typed
	X     Expr
	Pos   lexer.Pos
	Place *Place
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
	Pos      lexer.Pos
}

type FieldInit struct {
	Name  string
	Value Expr
	Pos   lexer.Pos
	Field *types.Field
}

type Unary struct {
	Typed
	Op  lexer.Kind
	X   Expr
	Pos lexer.Pos
}

type Binary struct {
	Typed
	Op     lexer.Kind
	X      Expr
	Y      Expr
	SplatX bool
	SplatY bool
	Pos    lexer.Pos
}

type Call struct {
	Typed
	Fn        Expr
	Args      []Expr
	Pos       lexer.Pos
	Prepended bool
	Target    *Func
	Builtin   string
	Variant   *types.Variant
	Embed     string
	Size      int64
	Digest    uint64
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
	Test     Expr
	Pos      lexer.Pos
	Variant  *types.Variant
	Local    *Local
}

type Subject struct {
	Typed
	Pos lexer.Pos
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
}

func (n *Block) Position() lexer.Pos     { return n.Pos }
func (n *Let) Position() lexer.Pos       { return n.Pos }
func (n *Assign) Position() lexer.Pos    { return n.Pos }
func (n *ExprStmt) Position() lexer.Pos  { return n.X.Position() }
func (n *For) Position() lexer.Pos       { return n.Pos }
func (n *Break) Position() lexer.Pos     { return n.Pos }
func (n *Continue) Position() lexer.Pos  { return n.Pos }
func (n *Return) Position() lexer.Pos    { return n.Pos }
func (n *Fail) Position() lexer.Pos      { return n.Pos }
func (n *Defer) Position() lexer.Pos     { return n.Pos }
func (n *Spawn) Position() lexer.Pos     { return n.Pos }
func (n *Send) Position() lexer.Pos      { return n.Pos }
func (n *Select) Position() lexer.Pos    { return n.Pos }
func (n *Subject) Position() lexer.Pos   { return n.Pos }
func (n *Receiver) Position() lexer.Pos  { return n.X.Position() }
func (n *IntLit) Position() lexer.Pos    { return n.Pos }
func (n *FloatLit) Position() lexer.Pos  { return n.Pos }
func (n *StrLit) Position() lexer.Pos    { return n.Pos }
func (n *BoolLit) Position() lexer.Pos   { return n.Pos }
func (n *Ident) Position() lexer.Pos     { return n.Pos }
func (n *Selector) Position() lexer.Pos  { return n.Pos }
func (n *StructLit) Position() lexer.Pos { return n.Pos }
func (n *Ref) Position() lexer.Pos       { return n.Pos }
func (n *Index) Position() lexer.Pos     { return n.Pos }
func (n *Instance) Position() lexer.Pos  { return n.Pos }
func (n *Closure) Position() lexer.Pos   { return n.Pos }
func (n *Slicing) Position() lexer.Pos   { return n.Pos }
func (n *SliceLit) Position() lexer.Pos  { return n.Pos }
func (n *Interp) Position() lexer.Pos    { return n.Pos }
func (n *MapLit) Position() lexer.Pos    { return n.Pos }
func (n *ForIn) Position() lexer.Pos     { return n.Pos }
func (n *Unary) Position() lexer.Pos     { return n.Pos }
func (n *Binary) Position() lexer.Pos    { return n.Pos }
func (n *Call) Position() lexer.Pos      { return n.Pos }
func (n *If) Position() lexer.Pos        { return n.Pos }
func (n *Match) Position() lexer.Pos     { return n.Pos }
func (n *Try) Position() lexer.Pos       { return n.Pos }
func (n *Catch) Position() lexer.Pos     { return n.Pos }

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
