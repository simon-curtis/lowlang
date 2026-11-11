package model

type Op int

const (
	Set Op = iota
	Add
	Sub
	Mul
	Div
	Rem
	And
	Or
	Xor
	Shl
	Shr
	Eq
	Ne
	Lt
	Le
	Gt
	Ge
	LogAnd
	LogOr
	Neg
	Compl
	Not
)

var opSymbols = [...]string{
	Set: "=", Add: "+", Sub: "-", Mul: "*", Div: "/", Rem: "%", And: "&", Or: "|", Xor: "^", Shl: "<<", Shr: ">>",
	Eq: "==", Ne: "!=", Lt: "<", Le: "<=", Gt: ">", Ge: ">=", LogAnd: "&&", LogOr: "||", Neg: "-", Compl: "~", Not: "!",
}

func (o Op) Symbol() string {
	return opSymbols[o]
}

func (o Op) IsCompare() bool {
	return o >= Eq && o <= Ge
}

func (o Op) IsShift() bool {
	return o == Shl || o == Shr
}

func (o Op) IsLogical() bool {
	return o == LogAnd || o == LogOr
}

type MathOp int

const (
	Sqrt MathOp = iota
	Floor
)

var mathNames = [...]string{Sqrt: "sqrt", Floor: "floor"}

func (m MathOp) String() string {
	return mathNames[m]
}

type Expr interface {
	Type() Kind
}

type Lit struct {
	Kind  Kind
	Bits  uint64
	Float float64
}

type Var struct {
	Name string
	Kind Kind
}

type Unary struct {
	Op Op
	X  Expr
}

type Binary struct {
	Op Op
	L  Expr
	R  Expr
}

type Convert struct {
	To Kind
	X  Expr
}

type IfExpr struct {
	Cond Expr
	Then Expr
	Else Expr
}

type Call struct {
	Callee string
	Result Kind
	Args   []Expr
}

type Math struct {
	Op MathOp
	X  Expr
}

func (e *Lit) Type() Kind     { return e.Kind }
func (e *Var) Type() Kind     { return e.Kind }
func (e *Convert) Type() Kind { return e.To }
func (e *IfExpr) Type() Kind  { return e.Then.Type() }
func (e *Call) Type() Kind    { return e.Result }
func (e *Math) Type() Kind    { return e.X.Type() }

func (e *Unary) Type() Kind {
	if e.Op == Not {
		return Bool
	}
	return e.X.Type()
}

func (e *Binary) Type() Kind {
	if e.Op.IsCompare() || e.Op.IsLogical() {
		return Bool
	}
	return e.L.Type()
}

type Stmt interface {
	stmt()
}

type Let struct {
	Name    string
	Kind    Kind
	Mutable bool
	Init    Expr
}

type Assign struct {
	Name  string
	Kind  Kind
	Op    Op
	Value Expr
}

type IfStmt struct {
	Cond Expr
	Then []Stmt
	Else []Stmt
}

type Range struct {
	ID   int
	Var  string
	Kind Kind
	Lo   Expr
	Hi   Expr
	Body []Stmt
}

type While struct {
	ID      int
	Counter string
	Bound   int
	Cond    Expr
	Body    []Stmt
}

type Jump struct {
	Target   int
	Continue bool
}

type Return struct {
	Value Expr
}

type Print struct {
	Value Expr
}

func (*Let) stmt()    {}
func (*Assign) stmt() {}
func (*IfStmt) stmt() {}
func (*Range) stmt()  {}
func (*While) stmt()  {}
func (*Jump) stmt()   {}
func (*Return) stmt() {}
func (*Print) stmt()  {}

type Param struct {
	Name string
	Kind Kind
}

type Func struct {
	Name   string
	Params []Param
	Result Kind
	Body   []Stmt
	Tail   Expr
}

type Program struct {
	Funcs []*Func
	Main  []Stmt
}
