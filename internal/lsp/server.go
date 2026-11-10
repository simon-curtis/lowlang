package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"lowlang/internal/ast"
	"lowlang/internal/compiler"
	"lowlang/internal/lexer"
	"lowlang/internal/parser"
)

var errorPos = regexp.MustCompile(`(?s)^(?:(.*?):)?(\d+):(\d+): (.*)$`)

type Server struct {
	stream    *stream
	docs      sources
	units     map[string]*compiler.Unit
	published map[string][]string
	compiles  *scheduler
	shutdown  bool
}

func NewServer(in io.Reader, out io.Writer) *Server {
	return &Server{
		stream:    newStream(in, out),
		docs:      sources{},
		units:     map[string]*compiler.Unit{},
		published: map[string][]string{},
		compiles:  newScheduler(),
	}
}

func (s *Server) Run(ctx context.Context) error {
	requests := make(chan request)
	failures := make(chan error, 1)
	go s.receive(ctx, requests, failures)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-failures:
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		case <-s.compiles.timer.C:
			s.compiles.next(ctx, s.docs)
		case result := <-s.compiles.results:
			if err := s.finish(result); err != nil {
				return err
			}
			s.compiles.next(ctx, s.docs)
		case req := <-requests:
			exit, err := s.handle(req)
			if err != nil {
				return err
			}
			if exit && !s.shutdown {
				return errors.New("exit before shutdown")
			}
			if exit {
				return nil
			}
		}
	}
}

func (s *Server) receive(ctx context.Context, requests chan<- request, failures chan<- error) {
	for {
		req, err := s.stream.read()
		if err != nil {
			failures <- err
			return
		}
		select {
		case requests <- req:
		case <-ctx.Done():
			return
		}
	}
}

func (s *Server) handle(req request) (bool, error) {
	switch req.Method {
	case "initialize":
		return false, s.reply(req.ID, s.initialize())
	case "shutdown":
		s.shutdown = true
		return false, s.reply(req.ID, nil)
	case "exit":
		return true, nil
	case "textDocument/didOpen":
		var p DidOpenParams
		if json.Unmarshal(req.Params, &p) != nil {
			return false, nil
		}
		path := pathOf(p.TextDocument.URI)
		s.docs[path] = []byte(p.TextDocument.Text)
		s.schedule(path, 0)
		return false, nil
	case "textDocument/didChange":
		var p DidChangeParams
		if json.Unmarshal(req.Params, &p) != nil || len(p.ContentChanges) == 0 {
			return false, nil
		}
		path := pathOf(p.TextDocument.URI)
		latest := p.ContentChanges[len(p.ContentChanges)-1]
		s.docs[path] = []byte(latest.Text)
		s.schedule(path, debounce)
		return false, nil
	case "textDocument/didSave":
		s.document(req, s.save)
		return false, nil
	case "textDocument/didClose":
		s.document(req, s.close)
		return false, nil
	case "textDocument/hover":
		return false, s.position(req, s.hover)
	case "textDocument/definition":
		return false, s.position(req, s.definition)
	case "textDocument/completion":
		return false, s.position(req, s.completion)
	case "textDocument/documentSymbol":
		var p DocumentParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return false, s.fail(req.ID, invalidParams, err.Error())
		}
		path := pathOf(p.TextDocument.URI)
		symbols := s.symbols(path)
		return false, s.reply(req.ID, symbols)
	}
	if req.ID == nil {
		return false, nil
	}
	message := fmt.Sprintf("method %s not supported", req.Method)
	return false, s.fail(req.ID, methodNotFound, message)
}

func (s *Server) initialize() InitializeResult {
	return InitializeResult{
		Capabilities: Capabilities{
			TextDocumentSync:       SyncOptions{OpenClose: true, Change: syncFull, Save: SaveOptions{}},
			HoverProvider:          true,
			DefinitionProvider:     true,
			DocumentSymbolProvider: true,
			CompletionProvider:     CompletionOptions{TriggerCharacters: []string{"."}},
		},
		ServerInfo: ServerInfo{Name: "low"},
	}
}

func (s *Server) document(req request, apply func(path string)) {
	var p DocumentParams
	if json.Unmarshal(req.Params, &p) != nil {
		return
	}
	path := pathOf(p.TextDocument.URI)
	apply(path)
}

func (s *Server) schedule(path string, delay time.Duration) {
	entry := s.entry(path)
	s.compiles.mark(entry, path, delay)
}

func (s *Server) save(path string) {
	s.schedule(path, 0)
}

func (s *Server) close(path string) {
	delete(s.docs, path)
	s.schedule(path, 0)
}

func (s *Server) position(req request, answer func(TextDocumentPositionParams) any) error {
	var p TextDocumentPositionParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return s.fail(req.ID, invalidParams, err.Error())
	}
	result := answer(p)
	return s.reply(req.ID, result)
}

func (s *Server) reply(id json.RawMessage, result any) error {
	body, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return s.stream.write(response{JSONRPC: "2.0", ID: id, Result: body})
}

func (s *Server) fail(id json.RawMessage, code int, message string) error {
	return s.stream.write(response{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message}})
}

func (s *Server) notify(method string, params any) error {
	return s.stream.write(notification{JSONRPC: "2.0", Method: method, Params: params})
}

func (s *Server) text(path string) text {
	src, err := s.docs.read(path)
	if err != nil {
		return text{}
	}
	return newText(src)
}

func (s *Server) entry(path string) string {
	for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
		marker := filepath.Join(dir, "low.mod")
		if _, err := os.Stat(marker); err == nil {
			return dir
		}
		if filepath.Dir(dir) == dir {
			return path
		}
	}
}

func (s *Server) finish(r compiled) error {
	current := s.compiles.finish(r)
	entry := r.job.entry
	if r.err == nil {
		s.units[entry] = r.unit
	}
	if !current {
		return nil
	}
	diagnostics := map[string][]Diagnostic{}
	for path := range s.docs {
		if s.entry(path) == entry {
			diagnostics[uriOf(path)] = []Diagnostic{}
		}
	}
	if r.err != nil {
		uri, d := s.diagnose(r.err, r.job.path)
		diagnostics[uri] = append(diagnostics[uri], d)
	}
	return s.publish(entry, diagnostics)
}

func (s *Server) publish(entry string, diagnostics map[string][]Diagnostic) error {
	var uris []string
	for uri := range diagnostics {
		uris = append(uris, uri)
	}
	for _, uri := range s.published[entry] {
		if _, ok := diagnostics[uri]; !ok {
			diagnostics[uri] = []Diagnostic{}
		}
	}
	s.published[entry] = uris
	for uri, list := range diagnostics {
		params := PublishDiagnosticsParams{URI: uri, Diagnostics: list}
		if err := s.notify("textDocument/publishDiagnostics", params); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) diagnose(err error, document string) (string, Diagnostic) {
	d := Diagnostic{Severity: severityError, Source: "low", Message: err.Error()}
	m := errorPos.FindStringSubmatch(err.Error())
	if m == nil {
		return uriOf(document), d
	}
	file := m[1]
	if file == "" || !filepath.IsAbs(file) {
		file = document
	}
	line, _ := strconv.Atoi(m[2])
	col, _ := strconv.Atoi(m[3])
	d.Message = m[4]
	pos := lexer.Pos{File: file, Line: line, Col: col}
	t := s.text(file)
	d.Range = t.word(pos)
	return uriOf(file), d
}

func (s *Server) query(p TextDocumentPositionParams) *query {
	path := pathOf(p.TextDocument.URI)
	file := s.checked(path)
	if file == nil {
		return nil
	}
	t := s.text(path)
	x := newIndex(file, t)
	line := p.Position.Line
	col := t.byteCol(line, p.Position.Character)
	occ := x.at(line, col)
	if occ == nil {
		return nil
	}
	unit := s.units[s.entry(path)]
	return &query{unit: unit, index: x, occ: occ}
}

func (s *Server) hover(p TextDocumentPositionParams) any {
	q := s.query(p)
	if q == nil {
		return nil
	}
	content := q.hover()
	if content == "" {
		return nil
	}
	value := fmt.Sprintf("```low\n%s\n```", content)
	span := q.index.text.span(q.occ.pos, len(q.occ.name))
	return Hover{Contents: MarkupContent{Kind: "markdown", Value: value}, Range: span}
}

func (s *Server) definition(p TextDocumentPositionParams) any {
	q := s.query(p)
	if q == nil {
		return nil
	}
	tg, ok := q.definition()
	if !ok {
		return nil
	}
	return s.locate(tg)
}

func (s *Server) completion(p TextDocumentPositionParams) any {
	path := pathOf(p.TextDocument.URI)
	t := s.text(path)
	c := &completer{
		unit: s.units[s.entry(path)],
		file: s.checked(path),
		text: t,
		line: p.Position.Line,
		col:  t.byteCol(p.Position.Line, p.Position.Character),
		seen: map[string]bool{},
	}
	items := c.complete()
	return CompletionList{Items: items}
}

func (s *Server) locate(tg target) Location {
	t := s.text(tg.path)
	pos := tg.pos
	if tg.name != "" {
		pos = t.nameAfter(pos, tg.name)
	}
	return Location{URI: uriOf(tg.path), Range: t.word(pos)}
}

func (s *Server) parsed(path string) *ast.File {
	src, err := s.docs.read(path)
	if err != nil {
		return nil
	}
	tokens, err := lexer.Lex(src, path)
	if err != nil {
		return nil
	}
	file, err := parser.Parse(tokens)
	if err != nil {
		return nil
	}
	return file
}

func (s *Server) symbols(path string) []DocumentSymbol {
	file := s.parsed(path)
	if file == nil {
		file = s.checked(path)
	}
	if file == nil {
		return []DocumentSymbol{}
	}
	return outline(file, s.text(path))
}

func (s *Server) checked(path string) *ast.File {
	unit := s.units[s.entry(path)]
	if unit == nil {
		return nil
	}
	for _, f := range unit.Files {
		if f.Path != "" && filepath.Clean(f.Path) == path {
			return f
		}
	}
	return nil
}
