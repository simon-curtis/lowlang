package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"lowlang/internal/ast"
	"lowlang/internal/compiler"
	"lowlang/internal/diag"
	"lowlang/internal/lint"
)

type Server struct {
	stream       *stream
	docs         compiler.Overlay
	units        map[string]*compiler.Unit
	published    map[string][]string
	fresh        map[string]bool
	compiledDocs map[string]compiler.Overlay
	compiles     *scheduler
	indexed      indexed
	hintRefresh  bool
	requests     int
	shutdown     bool
}

func NewServer(in io.Reader, out io.Writer) *Server {
	return &Server{
		stream:       newStream(in, out),
		docs:         compiler.Overlay{},
		units:        map[string]*compiler.Unit{},
		published:    map[string][]string{},
		fresh:        map[string]bool{},
		compiledDocs: map[string]compiler.Overlay{},
		compiles:     newScheduler(),
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
	case "":
		return false, nil
	case "initialize":
		var p InitializeParams
		if err := json.Unmarshal(req.Params, &p); err == nil {
			s.hintRefresh = p.Capabilities.Workspace.InlayHint.RefreshSupport
		}
		return false, s.reply(req.ID, s.initialize())
	case "textDocument/inlayHint":
		var p InlayHintParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return false, s.fail(req.ID, invalidParams, err.Error())
		}
		hints := s.inlayHints(p)
		return false, s.reply(req.ID, hints)
	case "workspace/symbol":
		var p WorkspaceSymbolParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return false, s.fail(req.ID, invalidParams, err.Error())
		}
		symbols := s.workspaceSymbols(p)
		return false, s.reply(req.ID, symbols)
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
	case "textDocument/signatureHelp":
		return false, s.position(req, s.signatureHelp)
	case "textDocument/references":
		var p ReferenceParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return false, s.fail(req.ID, invalidParams, err.Error())
		}
		locations := s.references(p)
		return false, s.reply(req.ID, locations)
	case "textDocument/rename":
		var p RenameParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return false, s.fail(req.ID, invalidParams, err.Error())
		}
		edit, err := s.rename(p)
		if err != nil {
			return false, s.fail(req.ID, requestFailed, err.Error())
		}
		return false, s.reply(req.ID, edit)
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
			TextDocumentSync:        SyncOptions{OpenClose: true, Change: syncFull, Save: SaveOptions{}},
			HoverProvider:           true,
			DefinitionProvider:      true,
			ReferencesProvider:      true,
			RenameProvider:          true,
			DocumentSymbolProvider:  true,
			CompletionProvider:      CompletionOptions{TriggerCharacters: []string{"."}},
			SignatureHelpProvider:   SignatureHelpOptions{TriggerCharacters: []string{"(", ","}, RetriggerCharacters: []string{","}},
			InlayHintProvider:       true,
			WorkspaceSymbolProvider: true,
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
	s.fresh[entry] = false
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
	src, err := s.docs.ReadFile(path)
	if err != nil {
		return text{}
	}
	return newText(src)
}

func (s *Server) entry(path string) string {
	if root, found := compiler.Root(s.docs, filepath.Dir(path)); found {
		return root
	}
	return path
}

func (s *Server) finish(r compiled) error {
	current := s.compiles.finish(r)
	entry := r.job.entry
	if r.err == nil {
		s.units[entry] = r.unit
		s.compiledDocs[entry] = r.job.docs
	}
	if !current {
		return nil
	}
	s.fresh[entry] = r.err == nil
	if r.err == nil {
		if err := s.refreshHints(); err != nil {
			return err
		}
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
	if r.err == nil {
		for _, w := range r.unit.Warnings {
			uri, d := s.warn(w, r.job.path)
			diagnostics[uri] = append(diagnostics[uri], d)
		}
	}
	return s.publish(entry, diagnostics)
}

func (s *Server) warn(w lint.Warning, document string) (string, Diagnostic) {
	file := w.Pos.File
	if file == "" || !filepath.IsAbs(file) {
		file = document
	}
	pos := diag.Pos{File: file, Line: w.Pos.Line, Col: w.Pos.Col}
	d := Diagnostic{Severity: severityWarning, Source: "low", Message: w.Msg, Range: s.text(file).word(pos)}
	return uriOf(file), d
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
	var located *diag.Error
	if !errors.As(err, &located) {
		return uriOf(document), d
	}
	pos := located.Pos
	if !filepath.IsAbs(pos.File) {
		pos.File = document
	}
	d.Message = located.Msg
	d.Range = s.text(pos.File).word(pos)
	return uriOf(pos.File), d
}

func (s *Server) query(p TextDocumentPositionParams) *query {
	path := pathOf(p.TextDocument.URI)
	unit := s.units[s.entry(path)]
	file := checkedFile(unit, path)
	if file == nil {
		return nil
	}
	x := s.index(path, file)
	line := p.Position.Line
	col := x.text.byteCol(line, p.Position.Character)
	occ := x.at(line, col)
	if occ == nil {
		return nil
	}
	return &query{unit: unit, index: x, occ: occ}
}

func (s *Server) index(path string, file *ast.File) *index {
	src, err := s.docs.ReadFile(path)
	if err != nil {
		return newIndex(file, text{})
	}
	if s.indexed.matches(path, file, src) {
		return s.indexed.index
	}
	x := newIndex(file, newText(src))
	s.indexed = indexed{path: path, file: file, src: string(src), index: x}
	return x
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
	if doc := q.doc(); doc != "" {
		value = fmt.Sprintf("%s\n\n%s", value, doc)
	}
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

func (s *Server) cursorAt(p TextDocumentPositionParams) *cursor {
	path := pathOf(p.TextDocument.URI)
	entry := s.entry(path)
	t := s.text(path)
	unit := s.units[entry]
	return &cursor{
		unit:     unit,
		file:     checkedFile(unit, path),
		text:     t,
		compiled: s.compiledText(entry, path),
		line:     p.Position.Line,
		col:      t.byteCol(p.Position.Line, p.Position.Character),
		seen:     map[string]bool{},
	}
}

func (s *Server) compiledText(entry, path string) text {
	docs, ok := s.compiledDocs[entry]
	if !ok {
		return text{}
	}
	src, err := docs.ReadFile(path)
	if err != nil {
		return text{}
	}
	return newText(src)
}

func (s *Server) completion(p TextDocumentPositionParams) any {
	items := s.cursorAt(p).complete()
	return CompletionList{Items: items}
}

func (s *Server) signatureHelp(p TextDocumentPositionParams) any {
	return s.cursorAt(p).signatureHelp()
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
	file, err := compiler.ParseFile(s.docs, path)
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
	return checkedFile(s.units[s.entry(path)], path)
}

func checkedFile(unit *compiler.Unit, path string) *ast.File {
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
