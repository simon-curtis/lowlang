package asm

import (
	"fmt"
	"math"
	"strings"
)

type Options struct {
	AlignBranches bool
	ClangLayout   bool
	Embeds        map[string][]byte
}

type Object struct {
	Sections []*Section
	Symbols  []Symbol
}

type SectionFlags uint8

const (
	FlagAlloc SectionFlags = 1 << iota
	FlagWrite
	FlagExec
	FlagTLS
	FlagReadOnly
)

func (f SectionFlags) Has(flag SectionFlags) bool {
	return f&flag != 0
}

func (f SectionFlags) String() string {
	var letters strings.Builder
	for _, entry := range []struct {
		flag   SectionFlags
		letter byte
	}{{FlagAlloc, 'a'}, {FlagWrite, 'w'}, {FlagExec, 'x'}, {FlagTLS, 'T'}, {FlagReadOnly, 'r'}} {
		if f.Has(entry.flag) {
			letters.WriteByte(entry.letter)
		}
	}
	return letters.String()
}

type Section struct {
	Name   string
	Flags  SectionFlags
	Align  int64
	Data   []byte
	NoBits bool
	Size   int64
	Fixups []Fixup
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
}

type FixupKind int

const (
	Abs64 FixupKind = iota + 1
	Abs32
	Abs32S
	PC32
	PLT32
	GOTPCREL
	GOTPCRELX
	REXGOTPCRELX
	ImgRel32
)

type fixupProperties struct {
	name       string
	width      int
	signed     bool
	pcRelative bool
	got        bool
	imageRel   bool
}

var fixupTable = [...]fixupProperties{
	Abs64:        {name: "Abs64", width: 8},
	Abs32:        {name: "Abs32", width: 4},
	Abs32S:       {name: "Abs32S", width: 4, signed: true},
	PC32:         {name: "PC32", width: 4, signed: true, pcRelative: true},
	PLT32:        {name: "PLT32", width: 4, signed: true, pcRelative: true},
	GOTPCREL:     {name: "GOTPCREL", width: 4, signed: true, pcRelative: true, got: true},
	GOTPCRELX:    {name: "GOTPCRELX", width: 4, signed: true, pcRelative: true, got: true},
	REXGOTPCRELX: {name: "REX_GOTPCRELX", width: 4, signed: true, pcRelative: true, got: true},
	ImgRel32:     {name: "ImgRel32", width: 4, imageRel: true},
}

func (k FixupKind) properties() fixupProperties {
	if k > 0 && int(k) < len(fixupTable) {
		return fixupTable[k]
	}
	return fixupProperties{}
}

func (k FixupKind) String() string {
	if name := k.properties().name; name != "" {
		return name
	}
	return fmt.Sprintf("FixupKind(%d)", int(k))
}

func (k FixupKind) Width() int {
	return k.properties().width
}

func (k FixupKind) Signed() bool {
	return k.properties().signed
}

func (k FixupKind) PCRelative() bool {
	return k.properties().pcRelative
}

func (k FixupKind) GOT() bool {
	return k.properties().got
}

func (k FixupKind) ImageRelative() bool {
	return k.properties().imageRel
}

func (k FixupKind) Fits(value int64) bool {
	switch {
	case k.Width() != 4:
		return true
	case k.Signed():
		return value >= math.MinInt32 && value <= math.MaxInt32
	}
	return value >= 0 && value <= math.MaxUint32
}

type Fixup struct {
	Offset int64
	Kind   FixupKind
	Symbol string
	Addend int64
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
