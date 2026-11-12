package lsp

import (
	"lowlang/internal/compiler"
)

type entryState struct {
	unit      *compiler.Unit
	docs      compiler.Overlay
	fresh     bool
	published []string
}

func (e *entryState) program() program {
	return program{unit: e.unit}
}

func (e *entryState) compiledText(path string) text {
	if e.docs == nil {
		return text{}
	}
	src, err := e.docs.ReadFile(path)
	if err != nil {
		return text{}
	}
	return newText(src)
}
