package prelude

import (
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
		s, ok := structs[l.Struct]
		if !ok {
			s = &ast.StructDecl{Name: l.Struct}
			structs[l.Struct] = s
			file.Structs = append(file.Structs, s)
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

func TestInstancesListEachRole(t *testing.T) {
	for _, role := range []Role{MapOp, ChanOp} {
		if len(Instances(role)) == 0 {
			t.Fatalf("role %d has no entries", role)
		}
	}
}
