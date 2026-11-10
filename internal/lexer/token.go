package lexer

import "fmt"

type Kind int

const (
	EOF Kind = iota
	Newline
	Ident
	Int
	Float
	Char
	String

	Fn
	Let
	Var
	If
	Else
	For
	Break
	Continue
	Return
	True
	False
	Enum
	Match
	Fail
	Catch
	Struct
	Mut
	In
	Import
	Pub
	Extern
	Defer
	Spawn
	Select

	LParen
	RParen
	LBracket
	RBracket
	DotDot
	Ellipsis
	LBrace
	RBrace
	Comma
	Colon
	Arrow
	FatArrow
	Dot
	Question
	Coalesce
	LArrow

	Assign
	PlusAssign
	MinusAssign
	StarAssign
	SlashAssign
	PercentAssign
	AmpAssign
	PipeAssign
	CaretAssign
	ShlAssign
	ShrAssign

	Plus
	Minus
	Star
	Slash
	Percent
	Amp
	Pipe
	Caret
	Tilde
	Shl
	Shr
	Not
	AndAnd
	OrOr
	Eq
	NotEq
	Lt
	LtEq
	Gt
	GtEq
)

var names = map[Kind]string{
	EOF: "end of file", Newline: "newline", Ident: "identifier", Int: "integer", Float: "float", Char: "character", String: "string",
	Fn: "fn", Let: "let", Var: "var", If: "if", Else: "else", For: "for",
	Break: "break", Continue: "continue", Return: "return", True: "true", False: "false",
	Enum: "enum", Match: "match", Fail: "fail", Catch: "catch", Struct: "struct", Mut: "mut", In: "in", Import: "import", Pub: "pub", Extern: "extern", Defer: "defer", Spawn: "spawn", Select: "select",
	LParen: "(", RParen: ")", LBracket: "[", RBracket: "]", DotDot: "..", Ellipsis: "...", LBrace: "{", RBrace: "}", Comma: ",", Colon: ":", Arrow: "->", FatArrow: "=>", Dot: ".", Question: "?", Coalesce: "??", LArrow: "<-",
	Assign: "=", PlusAssign: "+=", MinusAssign: "-=", StarAssign: "*=", SlashAssign: "/=",
	PercentAssign: "%=", AmpAssign: "&=", PipeAssign: "|=", CaretAssign: "^=", ShlAssign: "<<=", ShrAssign: ">>=",
	Plus: "+", Minus: "-", Star: "*", Slash: "/", Percent: "%", Amp: "&", Pipe: "|", Caret: "^", Tilde: "~", Shl: "<<", Shr: ">>", Not: "!",
	AndAnd: "&&", OrOr: "||", Eq: "==", NotEq: "!=", Lt: "<", LtEq: "<=", Gt: ">", GtEq: ">=",
}

func (k Kind) String() string {
	if n, ok := names[k]; ok {
		return n
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

var keywords = map[string]Kind{
	"fn": Fn, "let": Let, "var": Var, "if": If, "else": Else, "for": For,
	"break": Break, "continue": Continue, "return": Return, "true": True, "false": False,
	"enum": Enum, "match": Match, "fail": Fail, "catch": Catch, "struct": Struct, "mut": Mut, "in": In, "import": Import, "pub": Pub, "extern": Extern, "defer": Defer, "spawn": Spawn, "select": Select,
}

type Pos struct {
	File string
	Line int
	Col  int
}

func (p Pos) String() string {
	if p.File == "" {
		return fmt.Sprintf("%d:%d", p.Line, p.Col)
	}
	return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Col)
}

type Part struct {
	Text string
	Expr bool
	Pos  Pos
}

type Token struct {
	Kind  Kind
	Text  string
	Parts []Part
	Pos   Pos
}
