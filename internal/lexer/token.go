package lexer

import (
	"fmt"
	"maps"
)

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
	Using
	Drop

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
	PipeArrow

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

type spelling struct {
	text string
	kind Kind
}

var keywordSpellings = []spelling{
	{"fn", Fn}, {"let", Let}, {"var", Var}, {"if", If}, {"else", Else}, {"for", For},
	{"break", Break}, {"continue", Continue}, {"return", Return}, {"true", True}, {"false", False},
	{"enum", Enum}, {"match", Match}, {"fail", Fail}, {"catch", Catch}, {"struct", Struct}, {"mut", Mut}, {"in", In}, {"import", Import},
	{"pub", Pub}, {"extern", Extern}, {"defer", Defer}, {"spawn", Spawn}, {"select", Select}, {"using", Using}, {"drop", Drop},
}

var operatorSpellings = []spelling{
	{"...", Ellipsis}, {"<<=", ShlAssign}, {">>=", ShrAssign},
	{"??", Coalesce}, {"<-", LArrow}, {"|>", PipeArrow}, {"->", Arrow}, {"=>", FatArrow}, {"%=", PercentAssign}, {"&=", AmpAssign}, {"|=", PipeAssign}, {"^=", CaretAssign},
	{"<<", Shl}, {">>", Shr}, {"+=", PlusAssign}, {"-=", MinusAssign}, {"*=", StarAssign}, {"/=", SlashAssign},
	{"&&", AndAnd}, {"||", OrOr}, {"==", Eq}, {"!=", NotEq}, {"<=", LtEq}, {">=", GtEq},
	{"..", DotDot}, {"(", LParen}, {")", RParen}, {"[", LBracket}, {"]", RBracket}, {"{", LBrace}, {"}", RBrace}, {",", Comma}, {":", Colon}, {".", Dot}, {"?", Question},
	{"=", Assign}, {"+", Plus}, {"-", Minus}, {"*", Star}, {"/", Slash}, {"%", Percent},
	{"!", Not}, {"<", Lt}, {">", Gt}, {"&", Amp}, {"|", Pipe}, {"^", Caret}, {"~", Tilde},
}

var descriptions = map[Kind]string{
	EOF: "end of file", Newline: "newline", Ident: "identifier", Int: "integer", Float: "float", Char: "character", String: "string",
}

var keywordKinds = kindsOf(keywordSpellings)

var spellings = merge(kindTexts(keywordSpellings), kindTexts(operatorSpellings), descriptions)

func kindsOf(table []spelling) map[string]Kind {
	kinds := make(map[string]Kind, len(table))
	for _, s := range table {
		kinds[s.text] = s.kind
	}
	return kinds
}

func kindTexts(table []spelling) map[Kind]string {
	texts := make(map[Kind]string, len(table))
	for _, s := range table {
		texts[s.kind] = s.text
	}
	return texts
}

func merge(tables ...map[Kind]string) map[Kind]string {
	merged := map[Kind]string{}
	for _, table := range tables {
		maps.Copy(merged, table)
	}
	return merged
}

func (k Kind) String() string {
	if text, ok := spellings[k]; ok {
		return text
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

func Keywords() []string {
	return texts(keywordSpellings)
}

func Operators() []string {
	return texts(operatorSpellings)
}

func texts(table []spelling) []string {
	words := make([]string, len(table))
	for i, s := range table {
		words[i] = s.text
	}
	return words
}

func IsKeyword(name string) bool {
	_, ok := keywordKinds[name]
	return ok
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
	Text      string
	Expr      bool
	Pos       Pos
	Aligned   bool
	Width     string
	WidthPos  Pos
	Formatted bool
	Format    string
	FormatPos Pos
}

type Token struct {
	Kind  Kind
	Text  string
	Parts []Part
	Pos   Pos
	Doc   string
	DocAt Pos
}
