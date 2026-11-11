package asm

import (
	"fmt"
)

type Options struct {
	AlignBranches bool
}

type Object struct {
	Sections    []*Section
	Symbols     []Symbol
	TextEncoded bool
}

type Section struct {
	Name       string
	Flags      string
	Align      int64
	Data       []byte
	NoBits     bool
	Size       int64
	Fixups     []Fixup
	Items      []Item
	Unresolved []Span
}

const (
	SectionUndefined = -1
	SectionAbsolute  = -2
)

type Symbol struct {
	Name    string
	Section int
	Value   int64
	Global  bool
	Item    int
}

type FixupKind int

const (
	Abs64 FixupKind = iota + 1
	Abs32
	Abs32S
	PC32
	GOTPCREL
	ImgRel32
)

func (k FixupKind) String() string {
	switch k {
	case Abs64:
		return "Abs64"
	case Abs32:
		return "Abs32"
	case Abs32S:
		return "Abs32S"
	case PC32:
		return "PC32"
	case GOTPCREL:
		return "GOTPCREL"
	case ImgRel32:
		return "ImgRel32"
	}
	return fmt.Sprintf("FixupKind(%d)", int(k))
}

type Fixup struct {
	Offset int64
	Kind   FixupKind
	Symbol string
	Addend int64
	Line   int
}

type Span struct {
	Offset int64
	Size   int64
}

type ItemKind uint8

const (
	ItemInst ItemKind = iota + 1
	ItemData
	ItemAlign
)

type Item struct {
	Kind  ItemKind
	Line  int
	Inst  *Inst
	Data  []byte
	Exprs []DataExpr
	Align int64
}

type DataExpr struct {
	Offset int64
	Size   int
	Value  Value
	Line   int
}

type Error struct {
	Line    int
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("asm: line %d: %s: %v", e.Line, e.Message, e.Err)
	}
	return fmt.Sprintf("asm: line %d: %s", e.Line, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Err
}

func Assemble(source string, opts Options) (*Object, error) {
	a := newAssembler(opts)
	if err := a.run(source); err != nil {
		return nil, err
	}
	return a.object(), nil
}

func (a *assembler) run(source string) (err error) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		asmErr, ok := r.(*Error)
		if !ok {
			panic(r)
		}
		err = asmErr
	}()
	a.runFrame(&frame{text: source, main: true})
	a.finish()
	return nil
}

func (a *assembler) fail(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	panic(&Error{Line: a.line, Message: message})
}

func (a *assembler) failWrap(err error, format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	panic(&Error{Line: a.line, Message: message, Err: err})
}
