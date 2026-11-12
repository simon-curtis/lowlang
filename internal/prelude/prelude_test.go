package prelude

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"lowlang/internal/ast"
)

func declare() *ast.File {
	file := &ast.File{}
	for _, e := range Entries {
		fn := &ast.Func{Name: e.Name, Params: make([]*ast.Param, e.Params)}
		for range typeParams(e.Role) {
			fn.TypeParams = append(fn.TypeParams, &ast.TypeParam{})
		}
		file.Funcs = append(file.Funcs, fn)
	}
	structs := map[string]*ast.StructDecl{}
	for _, l := range Layouts {
		s, ok := structs[l.Func]
		if !ok {
			s = &ast.StructDecl{Name: fmt.Sprintf("%sResult", l.Func)}
			structs[l.Func] = s
			file.Structs = append(file.Structs, s)
			at := slices.IndexFunc(file.Funcs, func(fn *ast.Func) bool { return fn.Name == l.Func })
			if at < 0 {
				file.Funcs = append(file.Funcs, &ast.Func{Name: l.Func})
				at = len(file.Funcs) - 1
			}
			fn := file.Funcs[at]
			fn.Result = &ast.TypeName{Pointer: true, Elem: &ast.TypeName{Name: s.Name}}
		}
		s.Fields = append(s.Fields, &ast.FieldDecl{Name: l.Field})
	}
	return file
}

func TestValidateAcceptsADeclaredPrelude(t *testing.T) {
	if err := Validate(declare()); err != nil {
		t.Fatal(err)
	}
}

func TestValidateNamesAMissingFunction(t *testing.T) {
	file := declare()
	file.Funcs = file.Funcs[1:]
	err := Validate(file)
	if err == nil || !strings.Contains(err.Error(), Entries[0].Name) {
		t.Fatalf("want an error naming %s, got %v", Entries[0].Name, err)
	}
}

func TestValidateNamesAWrongParameterCount(t *testing.T) {
	file := declare()
	file.Funcs[0].Params = nil
	err := Validate(file)
	if err == nil || !strings.Contains(err.Error(), "parameters") {
		t.Fatalf("want a parameter count error, got %v", err)
	}
}

func TestValidateNamesAMissingField(t *testing.T) {
	file := declare()
	file.Structs[0].Fields = nil
	err := Validate(file)
	if err == nil || !strings.Contains(err.Error(), Layouts[0].Field) {
		t.Fatalf("want an error naming %s, got %v", Layouts[0].Field, err)
	}
}

func TestValidateNamesAFieldThatIsNotFirst(t *testing.T) {
	file := declare()
	for _, s := range file.Structs {
		if s.Name == fmt.Sprintf("%sResult", Box) {
			s.Fields = append([]*ast.FieldDecl{{Name: "other"}}, s.Fields...)
		}
	}
	err := Validate(file)
	if err == nil || !strings.Contains(err.Error(), BoxValue) {
		t.Fatalf("want an error naming %s, got %v", BoxValue, err)
	}
}

func TestValidateAcceptsAStructResultByValue(t *testing.T) {
	file := declare()
	index := slices.IndexFunc(file.Funcs, func(fn *ast.Func) bool { return fn.Name == SelRecv })
	file.Funcs[index].Result = &ast.TypeName{Name: file.Funcs[index].Result.Elem.Name}
	if err := Validate(file); err != nil {
		t.Fatal(err)
	}
}

func TestValidateNamesAResultThatIsNotAStruct(t *testing.T) {
	file := declare()
	index := slices.IndexFunc(file.Funcs, func(fn *ast.Func) bool { return fn.Name == Layouts[0].Func })
	file.Funcs[index].Result = &ast.TypeName{Name: "i64"}
	err := Validate(file)
	if err == nil || !strings.Contains(err.Error(), Layouts[0].Func) {
		t.Fatalf("want an error naming %s, got %v", Layouts[0].Func, err)
	}
}
