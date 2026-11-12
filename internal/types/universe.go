package types

import (
	"fmt"
	"strings"
)

const (
	RawName  = "ptr"
	ChanName = "chan"
)

var (
	builtinNames = indexNames()
	builtins     = indexBuiltins()
)

func indexNames() []string {
	var names []string
	for _, t := range Numeric {
		names = append(names, t.Name)
	}
	names = append(names, Bool.Name, Str.Name, RawName, ChanName)
	for _, t := range Vectors {
		names = append(names, t.Name)
	}
	return names
}

func indexBuiltins() map[string]*Type {
	found := map[string]*Type{RawName: nil, ChanName: nil}
	for _, group := range [][]*Type{Numeric, {Bool, Str}, Vectors} {
		for _, t := range group {
			found[t.Name] = t
		}
	}
	return found
}

func Builtin(name string) *Type {
	return builtins[name]
}

func Reserved(name string) bool {
	_, ok := builtins[name]
	return ok
}

func BuiltinNames() []string {
	return builtinNames
}

type Universe struct {
	OnCreate  func(*Type)
	pointers  map[*Type]*Type
	slices    map[*Type]*Type
	optionals map[*Type]*Type
	raws      map[*Type]*Type
	chans     map[*Type]*Type
	maps      map[[2]*Type]*Type
	results   map[[2]*Type]*Type
	funcs     map[string]*Type
}

func NewUniverse() *Universe {
	return &Universe{
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
	if u.OnCreate != nil {
		u.OnCreate(t)
	}
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
