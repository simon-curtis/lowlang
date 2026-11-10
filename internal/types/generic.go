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
)

var Constraints = map[string]Constraint{
	"Eq":      Eq,
	"Ord":     Ord | Eq,
	"Hash":    Hash | Eq,
	"Number":  Number | Ord | Eq,
	"Integer": Integer | Number | Ord | Eq | Hash,
	"Float":   Float | Number | Ord | Eq,
}

func NewParam(name string, c Constraint) *Type {
	return &Type{Kind: ParamKind, Name: name, Constraint: c, size: 1}
}

func (t *Type) has(c Constraint) bool {
	return t.Kind == ParamKind && t.Constraint&c != 0
}

func (t *Type) Satisfies(c Constraint) bool {
	checks := []struct {
		bit Constraint
		ok  func() bool
	}{
		{Eq, t.Comparable},
		{Ord, t.Ordered},
		{Hash, t.Hashable},
		{Number, t.Number},
		{Integer, t.Integer},
		{Float, t.Float},
	}
	for _, check := range checks {
		if c&check.bit != 0 && !check.ok() {
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
	return t.Number() || t.has(Ord)
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
