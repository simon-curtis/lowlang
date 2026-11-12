package prelude

import (
	"fmt"
	"slices"

	"lowlang/internal/ast"
)

const (
	AppendStr         = "__append_str"
	AppendQuoted      = "__append_quoted"
	AppendHex         = "__append_hex"
	AppendBool        = "__append_bool"
	AppendU64         = "__append_u64"
	AppendI64         = "__append_i64"
	AppendF64         = "__append_f64"
	AppendF32         = "__append_f32"
	AppendDecimal     = "__append_decimal"
	AppendRadix       = "__append_radix"
	AppendIntLayout   = "__append_int_layout"
	AppendFloatLayout = "__append_float_layout"
	Align             = "__align"
	StrCmp            = "__str_cmp"
	Min               = "__min"
	Utf8Decode        = "__utf8_decode"
	Bytes             = "__bytes"
	Cstr              = "__cstr"
	FromCstr          = "__from_cstr"
	FromBytes         = "__from_bytes"
	HashStr           = "__hash_str"
	HashWord          = "__hash_word"
	MapNew            = "__map_new"
	MapClone          = "__map_clone"
	MapLen            = "__map_len"
	MapGet            = "__map_get"
	MapSet            = "__map_set"
	MapDelete         = "__map_delete"
	MapNext           = "__map_next"
	MapKey            = "__map_key"
	MapValue          = "__map_value"
	ChanNew           = "__chan_new"
	ChanSend          = "__chan_send"
	ChanRecv          = "__chan_recv"
	ChanClose         = "__chan_close"
	Box               = "__box"
	SelRecv           = "__sel_recv"
	SelSend           = "__sel_send"
	SelWaitRecv       = "__sel_wait_recv"
	SelWaitSend       = "__sel_wait_send"
	SelValue          = "__sel_value"
	Waiter            = "__waiter"
	Wake              = "__wake"
	WaiterIndex       = "index"
	WaiterOK          = "ok"
	ChanLock          = "lock"
	ProbeState        = "state"
	ProbeWake         = "wake"
	BoxValue          = "value"
	mapTypeParams     = 2
	chanTypeParams    = 1
)

type Role int

const (
	Plain Role = iota
	MapOp
	ChanOp
)

type Entry struct {
	Name   string
	Role   Role
	Params int
}

var Entries = []Entry{
	{AppendStr, Plain, 2},
	{AppendQuoted, Plain, 2},
	{AppendHex, Plain, 2},
	{AppendBool, Plain, 2},
	{AppendU64, Plain, 2},
	{AppendI64, Plain, 2},
	{AppendF64, Plain, 2},
	{AppendF32, Plain, 2},
	{AppendDecimal, Plain, 4},
	{AppendRadix, Plain, 6},
	{AppendIntLayout, Plain, 8},
	{AppendFloatLayout, Plain, 7},
	{Align, Plain, 3},
	{StrCmp, Plain, 2},
	{Min, Plain, 2},
	{Utf8Decode, Plain, 2},
	{Bytes, Plain, 1},
	{Cstr, Plain, 1},
	{FromCstr, Plain, 1},
	{FromBytes, Plain, 1},
	{HashStr, Plain, 1},
	{HashWord, Plain, 1},
	{Waiter, Plain, 0},
	{Wake, Plain, 1},
	{MapNew, MapOp, 0},
	{MapClone, MapOp, 1},
	{MapLen, MapOp, 1},
	{MapGet, MapOp, 2},
	{MapSet, MapOp, 3},
	{MapDelete, MapOp, 2},
	{MapNext, MapOp, 2},
	{MapKey, MapOp, 2},
	{MapValue, MapOp, 2},
	{ChanNew, ChanOp, 1},
	{ChanSend, ChanOp, 2},
	{ChanRecv, ChanOp, 1},
	{ChanClose, ChanOp, 1},
	{SelRecv, ChanOp, 2},
	{SelSend, ChanOp, 2},
	{SelWaitRecv, ChanOp, 4},
	{SelWaitSend, ChanOp, 4},
	{SelValue, ChanOp, 2},
}

type Layout struct {
	Func  string
	Field string
	First bool
}

var Layouts = []Layout{
	{Waiter, WaiterIndex, false},
	{Waiter, WaiterOK, false},
	{ChanNew, ChanLock, false},
	{SelRecv, ProbeState, false},
	{SelRecv, ProbeWake, false},
	{SelSend, ProbeState, false},
	{SelSend, ProbeWake, false},
	{Box, BoxValue, true},
}

func Instances(role Role) []string {
	var names []string
	for _, e := range Entries {
		if e.Role == role {
			names = append(names, e.Name)
		}
	}
	return names
}

func typeParams(role Role) int {
	switch role {
	case MapOp:
		return mapTypeParams
	case ChanOp:
		return chanTypeParams
	}
	return 0
}

func Validate(file *ast.File) error {
	funcs := make(map[string]*ast.Func, len(file.Funcs))
	for _, fn := range file.Funcs {
		funcs[fn.Name] = fn
	}
	structs := make(map[string]*ast.StructDecl, len(file.Structs))
	for _, s := range file.Structs {
		structs[s.Name] = s
	}
	for _, e := range Entries {
		fn, ok := funcs[e.Name]
		if !ok {
			return fmt.Errorf("prelude: the compiler calls %s but the prelude does not declare it", e.Name)
		}
		if want := typeParams(e.Role); len(fn.TypeParams) != want {
			return fmt.Errorf("prelude: %s takes %d type parameters, the compiler instantiates it with %d", e.Name, len(fn.TypeParams), want)
		}
		if len(fn.Params) != e.Params {
			return fmt.Errorf("prelude: %s takes %d parameters, the compiler passes %d", e.Name, len(fn.Params), e.Params)
		}
	}
	for _, l := range Layouts {
		if err := validateLayout(l, funcs, structs); err != nil {
			return err
		}
	}
	return nil
}

func validateLayout(l Layout, funcs map[string]*ast.Func, structs map[string]*ast.StructDecl) error {
	fn, ok := funcs[l.Func]
	if !ok {
		return fmt.Errorf("prelude: the compiler reads %s from the result of %s but the prelude does not declare it", l.Field, l.Func)
	}
	if fn.Result == nil || fn.Result.Pointer && fn.Result.Elem == nil {
		return fmt.Errorf("prelude: the compiler reads %s from the result of %s but it does not return a struct", l.Field, l.Func)
	}
	name := fn.Result.Name
	if fn.Result.Pointer {
		name = fn.Result.Elem.Name
	}
	s, ok := structs[name]
	if !ok {
		return fmt.Errorf("prelude: %s returns %s but the prelude does not declare that struct", l.Func, name)
	}
	at := slices.IndexFunc(s.Fields, func(f *ast.FieldDecl) bool { return f.Name == l.Field })
	if at < 0 {
		return fmt.Errorf("prelude: the compiler reads %s.%s from the result of %s but the struct has no such field", name, l.Field, l.Func)
	}
	if l.First && at != 0 {
		return fmt.Errorf("prelude: the compiler expects %s.%s to be the first field", name, l.Field)
	}
	return nil
}
