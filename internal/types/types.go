package types

import "fmt"

type Kind int

const (
	VoidKind Kind = iota
	NeverKind
	IntKind
	FloatKind
	BoolKind
	StrKind
	EnumKind
	StructKind
	ResultKind
	PointerKind
)

type Type struct {
	Kind     Kind
	Name     string
	Variants []*Variant
	Fields   []*Field
	Elem     *Type
	Ok       *Type
	Err      *Type
	Bits     int
	Signed   bool
	size     int
}

type Field struct {
	Name   string
	Type   *Type
	Offset int
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
	I64   = &Type{Kind: IntKind, Name: "i64", Bits: 64, Signed: true, size: 1}
	I32   = &Type{Kind: IntKind, Name: "i32", Bits: 32, Signed: true, size: 1}
	U32   = &Type{Kind: IntKind, Name: "u32", Bits: 32, size: 1}
	U16   = &Type{Kind: IntKind, Name: "u16", Bits: 16, size: 1}
	U8    = &Type{Kind: IntKind, Name: "u8", Bits: 8, size: 1}
	F64   = &Type{Kind: FloatKind, Name: "f64", Bits: 64, size: 1}
	F32   = &Type{Kind: FloatKind, Name: "f32", Bits: 32, size: 1}
	Bool  = &Type{Kind: BoolKind, Name: "bool", size: 1}
	Str   = &Type{Kind: StrKind, Name: "str", size: 1}
)

var Numeric = []*Type{I64, I32, U32, U16, U8, F64, F32}

func (t *Type) Integer() bool {
	return t.Kind == IntKind
}

func (t *Type) Float() bool {
	return t.Kind == FloatKind
}

func (t *Type) Number() bool {
	return t.Integer() || t.Float()
}

func (t *Type) Holds(n int64) bool {
	if t.Bits == 64 {
		return t.Signed || n >= 0
	}
	if t.Signed {
		limit := int64(1) << (t.Bits - 1)
		return n >= -limit && n < limit
	}
	return n >= 0 && n < int64(1)<<t.Bits
}

func NewEnum(name string) *Type {
	return &Type{Kind: EnumKind, Name: name, size: -1}
}

func NewStruct(name string) *Type {
	return &Type{Kind: StructKind, Name: name, size: -1}
}

func NewPointer(elem *Type) *Type {
	return &Type{Kind: PointerKind, Name: fmt.Sprintf("*%s", elem), Elem: elem, size: 1}
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

func (t *Type) Field(name string) *Field {
	for _, f := range t.Fields {
		if f.Name == name {
			return f
		}
	}
	return nil
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
	Type *Type
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("%s contains itself and has infinite size", e.Type.Name)
}

func (t *Type) Layout() error {
	if (t.Kind != EnumKind && t.Kind != StructKind) || t.size > 0 {
		return nil
	}
	if t.size == 0 {
		return &CycleError{Type: t}
	}
	t.size = 0
	if t.Kind == StructKind {
		return t.structLayout()
	}
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

func (t *Type) structLayout() error {
	words := 0
	for _, f := range t.Fields {
		if err := f.Type.Layout(); err != nil {
			return err
		}
		f.Offset = words
		words += f.Type.Size()
	}
	t.size = words
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
