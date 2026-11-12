package types

import (
	"fmt"
	"strings"
)

type Constraint int

const (
	Eq Constraint = 1 << iota
	Ord
	Hash
	Number
	Integer
	Float
	Vector
)

var constraints = []struct {
	name  string
	bit   Constraint
	bits  Constraint
	holds func(*Type) bool
}{
	{"Eq", Eq, Eq, (*Type).Comparable},
	{"Ord", Ord, Ord | Eq, (*Type).Ordered},
	{"Hash", Hash, Hash | Eq, (*Type).Hashable},
	{"Number", Number, Number | Ord | Eq, (*Type).Number},
	{"Integer", Integer, Integer | Number | Ord | Eq | Hash, (*Type).Integer},
	{"Float", Float, Float | Number | Ord | Eq, (*Type).Float},
	{"Vector", Vector, Vector, func(t *Type) bool { return t.Kind == VectorKind }},
}

func ParseConstraint(name string) (Constraint, bool) {
	for _, c := range constraints {
		if c.name == name {
			return c.bits, true
		}
	}
	return 0, false
}

func ConstraintNames() []string {
	names := make([]string, len(constraints))
	for i, c := range constraints {
		names[i] = c.name
	}
	return names
}

func NewParam(name string, c Constraint) *Type {
	return &Type{Kind: ParamKind, Name: name, Constraint: c, size: 1}
}

func (t *Type) has(c Constraint) bool {
	return t.Kind == ParamKind && t.Constraint&c != 0
}

func (t *Type) Satisfies(c Constraint) bool {
	for _, k := range constraints {
		if c&k.bit != 0 && !k.holds(t) {
			return false
		}
	}
	return true
}

func (t *Type) Comparable() bool {
	switch t.Kind {
	case BoolKind, StrKind, PointerKind, RawKind, ChanKind:
		return true
	case StructKind:
		for _, f := range t.Fields {
			if !f.Type.Comparable() {
				return false
			}
		}
		return true
	case EnumKind:
		for _, v := range t.Variants {
			for _, f := range v.Fields {
				if !f.Comparable() {
					return false
				}
			}
		}
		return true
	}
	return t.Number() || t.has(Eq)
}

func (t *Type) Ordered() bool {
	return t.Number() || t == Str || t.has(Ord)
}

func (t *Type) Abstract() bool {
	switch {
	case t == nil:
		return false
	case t.Kind == ParamKind:
		return true
	case t.Elem.Abstract() || t.Key.Abstract() || t.Ret.Abstract() || t.Ok.Abstract() || t.Err.Abstract():
		return true
	}
	for _, list := range [][]*Type{t.Params, t.TypeArgs} {
		for _, p := range list {
			if p.Abstract() {
				return true
			}
		}
	}
	return false
}

func InstanceName(base string, args []*Type) string {
	names := make([]string, len(args))
	for i, a := range args {
		names[i] = a.String()
	}
	return fmt.Sprintf("%s[%s]", base, strings.Join(names, ", "))
}
