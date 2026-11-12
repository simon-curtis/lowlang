package check

import (
	"fmt"

	"lowlang/internal/lexer"
)

func fixedBuiltin(name string) *Builtin {
	b := builtinIndex[name]
	if b == nil || b.Shadowable || b.Kind == TypeBuiltin {
		return nil
	}
	return b
}

func DeclarationNameError(name string) error {
	switch {
	case lexer.IsKeyword(name):
		return fmt.Errorf("%q is a keyword", name)
	case fixedBuiltin(name) != nil:
		return fmt.Errorf("%q is a builtin", name)
	case reservedType(name):
		return fmt.Errorf("%q is a builtin type", name)
	}
	return nil
}
