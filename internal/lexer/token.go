package lexer

import "fmt"

type Kind int

const (
	EOF Kind = iota
	Newline
	Ident
	Int
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

	LParen
	RParen
	LBrace
	RBrace
	Comma
	Colon
	Arrow
	FatArrow
	Dot
	Question

	Assign
	PlusAssign
	MinusAssign
	StarAssign
	SlashAssign

	Plus
	Minus
	Star
	Slash
	Percent
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
	EOF: "end of file", Newline: "newline", Ident: "identifier", Int: "integer", String: "string",
	Fn: "fn", Let: "let", Var: "var", If: "if", Else: "else", For: "for",
	Break: "break", Continue: "continue", Return: "return", True: "true", False: "false",
	Enum: "enum", Match: "match", Fail: "fail", Catch: "catch",
	LParen: "(", RParen: ")", LBrace: "{", RBrace: "}", Comma: ",", Colon: ":", Arrow: "->", FatArrow: "=>", Dot: ".", Question: "?",
	Assign: "=", PlusAssign: "+=", MinusAssign: "-=", StarAssign: "*=", SlashAssign: "/=",
	Plus: "+", Minus: "-", Star: "*", Slash: "/", Percent: "%", Not: "!",
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
	"enum": Enum, "match": Match, "fail": Fail, "catch": Catch,
}

type Pos struct {
	Line int
	Col  int
}

func (p Pos) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Col)
}

type Token struct {
	Kind Kind
	Text string
	Pos  Pos
}
