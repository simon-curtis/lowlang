package ast

import "reflect"

var astPackage = reflect.TypeFor[File]().PkgPath()

var shared = map[reflect.Type]bool{
	reflect.TypeFor[File]():   true,
	reflect.TypeFor[Extern](): true,
	reflect.TypeFor[Func]():   true,
	reflect.TypeFor[Const]():  true,
	reflect.TypeFor[Local]():  true,
}

func CloneFunc(f *Func) *Func {
	c := *f
	c.Params = clone(f.Params)
	c.Result = clone(f.Result)
	c.Error = clone(f.Error)
	c.Body = clone(f.Body)
	c.ParamTypes, c.RetType, c.ParamLocals = nil, nil, nil
	return &c
}

func CloneStmt(s Stmt) Stmt {
	return clone(s)
}

func CloneExpr(e Expr) Expr {
	return clone(e)
}

func clone[T any](v T) T {
	copied := deep(reflect.ValueOf(&v).Elem())
	return copied.Interface().(T)
}

func copyPointer(v reflect.Value) reflect.Value {
	out := reflect.New(v.Type().Elem())
	out.Elem().Set(deep(v.Elem()))
	return out
}

func cloneBinary(e *Binary) *Binary {
	spine := LeftSpine(e)
	last := len(spine) - 1
	x := clone(spine[last].X)
	for i := last; i >= 0; i-- {
		link := *spine[i]
		link.X = nil
		copied := copyPointer(reflect.ValueOf(&link)).Interface().(*Binary)
		copied.X = x
		x = copied
	}
	return x.(*Binary)
}

func cloneIf(e *If) *If {
	chain := ElseChain(e)
	last := len(chain) - 1
	var otherwise Expr
	if chain[last].Else != nil {
		otherwise = clone(chain[last].Else)
	}
	var copied *If
	for i := last; i >= 0; i-- {
		link := *chain[i]
		link.Else = nil
		copied = copyPointer(reflect.ValueOf(&link)).Interface().(*If)
		copied.Else = otherwise
		otherwise = copied
	}
	return copied
}

func deep(v reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() || v.Type().Elem().PkgPath() != astPackage || shared[v.Type().Elem()] {
			return v
		}
		switch e := v.Interface().(type) {
		case *Binary:
			return reflect.ValueOf(cloneBinary(e))
		case *If:
			return reflect.ValueOf(cloneIf(e))
		}
		return copyPointer(v)
	case reflect.Struct:
		if v.Type().PkgPath() != astPackage {
			return v
		}
		out := reflect.New(v.Type()).Elem()
		for i := range v.NumField() {
			out.Field(i).Set(deep(v.Field(i)))
		}
		return out
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := range v.Len() {
			out.Index(i).Set(deep(v.Index(i)))
		}
		return out
	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		out := reflect.New(v.Type()).Elem()
		out.Set(deep(v.Elem()))
		return out
	}
	return v
}
