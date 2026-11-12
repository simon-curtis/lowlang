package types

import (
	"fmt"
	"strings"
)

type BuiltinType struct {
	Name string
	Type *Type
}

var builtinTypes = indexBuiltinTypes()

func indexBuiltinTypes() []BuiltinType {
	var list []BuiltinType
	for _, t := range Numeric {
		list = append(list, BuiltinType{t.Name, t})
	}
	list = append(list, BuiltinType{Bool.Name, Bool}, BuiltinType{Str.Name, Str})
	for _, t := range Vectors {
		list = append(list, BuiltinType{t.Name, t})
	}
	return list
}

func BuiltinTypes() []BuiltinType {
	return builtinTypes
}

type Universe struct {
	created   func(*Type)
	pointers  map[*Type]*Type
	slices    map[*Type]*Type
	optionals map[*Type]*Type
	raws      map[*Type]*Type
	chans     map[*Type]*Type
	maps      map[[2]*Type]*Type
	results   map[[2]*Type]*Type
	funcs     map[string]*Type
}

func NewUniverse(created func(*Type)) *Universe {
	return &Universe{
		created:   created,
		pointers:  map[*Type]*Type{},
		slices:    map[*Type]*Type{},
		optionals: map[*Type]*Type{},
		raws:      map[*Type]*Type{},
		chans:     map[*Type]*Type{},
		maps:      map[[2]*Type]*Type{},
		results:   map[[2]*Type]*Type{},
		funcs:     map[string]*Type{},
	}
}

func intern[K comparable](u *Universe, cache map[K]*Type, key K, build func() *Type) *Type {
	if t, ok := cache[key]; ok {
		return t
	}
	t := build()
	cache[key] = t
	u.created(t)
	return t
}

func (u *Universe) Pointer(elem *Type) *Type {
	return intern(u, u.pointers, elem, func() *Type { return newPointer(elem) })
}

func (u *Universe) Slice(elem *Type) *Type {
	return intern(u, u.slices, elem, func() *Type { return newSlice(elem) })
}

func (u *Universe) Optional(elem *Type) *Type {
	return intern(u, u.optionals, elem, func() *Type { return newOptional(elem) })
}

func (u *Universe) Raw(elem *Type) *Type {
	return intern(u, u.raws, elem, func() *Type { return newRaw(elem) })
}

func (u *Universe) Chan(elem *Type) *Type {
	return intern(u, u.chans, elem, func() *Type { return newChan(elem) })
}

func (u *Universe) Map(key, value *Type) *Type {
	return intern(u, u.maps, [2]*Type{key, value}, func() *Type { return newMap(key, value) })
}

func (u *Universe) Result(ok, err *Type) *Type {
	return intern(u, u.results, [2]*Type{ok, err}, func() *Type { return newResult(ok, err) })
}

func (u *Universe) Func(params []*Type, muts []bool, ret *Type, c bool) *Type {
	var key strings.Builder
	fmt.Fprintf(&key, "%t %p", c, ret)
	for i, p := range params {
		fmt.Fprintf(&key, " %t %p", muts[i], p)
	}
	return intern(u, u.funcs, key.String(), func() *Type { return newFunc(params, muts, ret, c) })
}
