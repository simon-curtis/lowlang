package lsp

import (
	"fmt"

	"lowlang/internal/ast"
)

func (s *Server) inlayHints(p InlayHintParams) []InlayHint {
	hints := []InlayHint{}
	path := pathOf(p.TextDocument.URI)
	file := s.checked(path)
	if file == nil || !s.fresh[s.entry(path)] {
		return hints
	}
	x := newIndex(file, s.text(path))
	for _, o := range x.occurrences {
		if !inferred(o) || o.local.Type == nil {
			continue
		}
		line := o.pos.Line - 1
		if line < p.Range.Start.Line || line > p.Range.End.Line {
			continue
		}
		end := o.pos
		end.Col += len(o.name)
		label := fmt.Sprintf(": %s", o.local.Type)
		hints = append(hints, InlayHint{Position: x.text.position(end), Label: label, Kind: hintType})
	}
	return hints
}

func inferred(o occurrence) bool {
	if o.local == nil {
		return false
	}
	switch n := o.node.(type) {
	case *ast.Let:
		return n.TypeName == nil
	case *ast.ForIn:
		return true
	}
	return false
}

func (s *Server) refreshHints() error {
	if !s.hintRefresh {
		return nil
	}
	s.requests++
	return s.stream.write(serverRequest{JSONRPC: "2.0", ID: s.requests, Method: "workspace/inlayHint/refresh"})
}
