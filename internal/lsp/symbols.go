package lsp

import (
	"fmt"

	"lowlang/internal/ast"
	"lowlang/internal/lexer"
)

func outline(file *ast.File, t text) []DocumentSymbol {
	symbols := []DocumentSymbol{}
	for _, e := range file.Externs {
		symbols = append(symbols, externSymbol(e, t))
	}
	for _, d := range file.Structs {
		symbols = append(symbols, structSymbol(d, t))
	}
	for _, d := range file.Enums {
		symbols = append(symbols, enumSymbol(d, t))
	}
	for _, f := range file.Funcs {
		if f.Template == nil && f.Closure == nil {
			symbols = append(symbols, funcSymbol(f, t))
		}
	}
	return symbols
}

func symbol(t text, pos lexer.Pos, name, detail string, kind int) DocumentSymbol {
	span := t.span(pos, len(name))
	return DocumentSymbol{Name: name, Detail: detail, Kind: kind, Range: span, SelectionRange: span}
}

func funcSymbol(f *ast.Func, t text) DocumentSymbol {
	pos := t.nameAfter(f.Pos, f.Name)
	return symbol(t, pos, f.Name, signature(f), symbolFunction)
}

func externSymbol(e *ast.Extern, t text) DocumentSymbol {
	name := fmt.Sprintf("extern %q", e.Library)
	s := symbol(t, e.Pos, "extern", "", symbolNamespace)
	s.Name = name
	for _, f := range e.Funcs {
		s.Children = append(s.Children, funcSymbol(f, t))
	}
	return s
}

func structSymbol(d *ast.StructDecl, t text) DocumentSymbol {
	pos := t.nameAfter(d.Pos, d.Name)
	s := symbol(t, pos, d.Name, "struct", symbolStruct)
	for _, f := range d.Fields {
		detail := typeNameString(f.TypeName)
		s.Children = append(s.Children, symbol(t, f.Pos, f.Name, detail, symbolField))
	}
	return s
}

func enumSymbol(d *ast.EnumDecl, t text) DocumentSymbol {
	pos := t.nameAfter(d.Pos, d.Name)
	s := symbol(t, pos, d.Name, "enum", symbolEnum)
	for _, v := range d.Variants {
		detail := variantDeclText(v)
		s.Children = append(s.Children, symbol(t, v.Pos, v.Name, detail, symbolEnumMember))
	}
	return s
}
