package types

import "fmt"

type Kind int

const (
	VoidKind Kind = iota
	NeverKind
	I64Kind
	BoolKind
	StrKind
	EnumKind
	ResultKind
)

type Type struct {
	Kind     Kind
	Name     string
	Variants []*Variant
	Ok       *Type
	Err      *Type
	size     int
}

type Variant struct {
	Name    string
	Tag     int
	Fields  []*Type
	Offsets []int
}

var (
	Void  = &Type{Kind: VoidKind, Name: "void"}
	Never = &Type{Kind: NeverKind, Name: "never"}
	I64   = &Type{Kind: I64Kind, Name: "i64", size: 1}
	Bool  = &Type{Kind: BoolKind, Name: "bool", size: 1}
	Str   = &Type{Kind: StrKind, Name: "str", size: 1}
)

func NewEnum(name string) *Type {
	return &Type{Kind: EnumKind, Name: name, size: -1}
}

func NewResult(ok, err *Type) *Type {
	return &Type{Kind: ResultKind, Ok: ok, Err: err, size: 1 + max(ok.Size(), err.Size())}
}

func (t *Type) String() string {
	if t.Kind != ResultKind {
		return t.Name
	}
	if t.Ok == Void {
		return fmt.Sprintf("! %s", t.Err)
	}
	return fmt.Sprintf("%s ! %s", t.Ok, t.Err)
}

func (t *Type) Size() int {
	return t.size
}

func (t *Type) Scalar() bool {
	return t.size == 1
}

func (t *Type) StackWords() int {
	if t.size > 1 {
		return t.size
	}
	return 0
}

func (t *Type) Variant(name string) *Variant {
	for _, v := range t.Variants {
		if v.Name == name {
			return v
		}
	}
	return nil
}

type CycleError struct {
	Enum *Type
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("enum %s contains itself and has infinite size", e.Enum.Name)
}

func (t *Type) Layout() error {
	if t.Kind != EnumKind || t.size > 0 {
		return nil
	}
	if t.size == 0 {
		return &CycleError{Enum: t}
	}
	t.size = 0
	payload := 0
	for _, v := range t.Variants {
		words, err := v.layout()
		if err != nil {
			return err
		}
		payload = max(payload, words)
	}
	t.size = 1 + payload
	return nil
}

func (v *Variant) layout() (int, error) {
	words := 0
	v.Offsets = make([]int, len(v.Fields))
	for i, f := range v.Fields {
		if err := f.Layout(); err != nil {
			return 0, err
		}
		v.Offsets[i] = 1 + words
		words += f.Size()
	}
	return words, nil
}
