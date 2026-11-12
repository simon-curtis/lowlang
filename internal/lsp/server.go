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
	stream      *stream
	docs        compiler.Overlay
	entries     map[string]*entryState
	compiles    *scheduler
	indexed     indexed
	hintRefresh bool
	requests    int
	shutdown    bool
}

func NewServer(in io.Reader, out io.Writer) *Server {
	return &Server{
		stream:   newStream(in, out),
		docs:     compiler.Overlay{},
		entries:  map[string]*entryState{},
		compiles: newScheduler(),
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
		return false, serve(s, req, s.initialize)
	case "textDocument/inlayHint":
		return false, serve(s, req, s.inlayHints)
	case "workspace/symbol":
		return false, serve(s, req, s.workspaceSymbols)
	case "shutdown":
		s.shutdown = true
		return false, s.reply(req.ID, nil)
	case "exit":
		return true, nil
	case "textDocument/didOpen":
		return false, observe(s, req, s.open)
	case "textDocument/didChange":
		return false, observe(s, req, s.change)
	case "textDocument/didSave":
		return false, observe(s, req, s.save)
	case "textDocument/didClose":
		return false, observe(s, req, s.close)
	case "textDocument/hover":
		return false, serve(s, req, s.hover)
	case "textDocument/definition":
		return false, serve(s, req, s.definition)
	case "textDocument/completion":
		return false, serve(s, req, s.completion)
	case "textDocument/signatureHelp":
		return false, serve(s, req, s.signatureHelp)
	case "textDocument/references":
		return false, serve(s, req, s.references)
	case "textDocument/rename":
		return false, serveChecked(s, req, s.rename)
	case "textDocument/documentSymbol":
		return false, serve(s, req, s.documentSymbols)
	}
	if req.ID == nil {
		return false, nil
	}
	message := fmt.Sprintf("method %s not supported", req.Method)
	return false, s.fail(req.ID, methodNotFound, message)
}

func decode[P any](req request) (P, error) {
	var p P
	if len(req.Params) == 0 {
		return p, nil
	}
	err := json.Unmarshal(req.Params, &p)
	return p, err
}

func serve[P, R any](s *Server, req request, answer func(P) R) error {
	return serveChecked(s, req, func(p P) (R, error) { return answer(p), nil })
}

func serveChecked[P, R any](s *Server, req request, answer func(P) (R, error)) error {
	p, err := decode[P](req)
	if err != nil {
		return s.fail(req.ID, invalidParams, err.Error())
	}
	result, err := answer(p)
	if err != nil {
		return s.fail(req.ID, requestFailed, err.Error())
	}
	return s.reply(req.ID, result)
}

func observe[P any](s *Server, req request, apply func(P)) error {
	p, err := decode[P](req)
	if err != nil {
		message := fmt.Sprintf("%s: %v", req.Method, err)
		return s.notify("window/logMessage", LogMessageParams{Type: messageError, Message: message})
	}
	apply(p)
	return nil
}

func (s *Server) initialize(p InitializeParams) InitializeResult {
	s.hintRefresh = p.Capabilities.Workspace.InlayHint.RefreshSupport
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

func (s *Server) open(p DidOpenParams) {
	path := pathOf(p.TextDocument.URI)
	s.docs[path] = []byte(p.TextDocument.Text)
	s.schedule(path, 0)
}

func (s *Server) change(p DidChangeParams) {
	if len(p.ContentChanges) == 0 {
		return
	}
	path := pathOf(p.TextDocument.URI)
	latest := p.ContentChanges[len(p.ContentChanges)-1]
	s.docs[path] = []byte(latest.Text)
	s.schedule(path, debounce)
}

func (s *Server) schedule(path string, delay time.Duration) {
	entry := s.entry(path)
	s.entryState(entry).fresh = false
	s.compiles.mark(entry, path, delay)
}

func (s *Server) save(p DocumentParams) {
	s.schedule(pathOf(p.TextDocument.URI), 0)
}

func (s *Server) close(p DocumentParams) {
	path := pathOf(p.TextDocument.URI)
	delete(s.docs, path)
	s.schedule(path, 0)
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
	if root, ok := compiler.Root(s.docs, filepath.Dir(path)); ok {
		return root
	}
	return path
}

func (s *Server) entryState(entry string) *entryState {
	state, ok := s.entries[entry]
	if !ok {
		state = &entryState{}
		s.entries[entry] = state
	}
	return state
}

func (s *Server) stateOf(path string) *entryState {
	return s.entryState(s.entry(path))
}

func (s *Server) finish(r compiled) error {
	current := s.compiles.finish(r)
	entry := r.job.entry
	state := s.entryState(entry)
	if r.err == nil {
		state.unit = r.unit
		state.docs = r.job.docs
	}
	if !current {
		return nil
	}
	state.fresh = r.err == nil
	diagnostics := map[string][]Diagnostic{}
	for path := range s.docs {
		if s.entry(path) == entry {
			diagnostics[uriOf(path)] = []Diagnostic{}
		}
	}
	if r.err != nil {
		uri, d := s.diagnose(r.err, r.job.path)
		diagnostics[uri] = append(diagnostics[uri], d)
		return s.publish(state, diagnostics)
	}
	if err := s.refreshHints(); err != nil {
		return err
	}
	for _, w := range r.unit.Warnings {
		uri, d := s.warn(w, r.job.path)
		diagnostics[uri] = append(diagnostics[uri], d)
	}
	return s.publish(state, diagnostics)
}

func located(pos diag.Pos, document string) diag.Pos {
	if !filepath.IsAbs(pos.File) {
		pos.File = document
	}
	return pos
}

func (s *Server) warn(w lint.Warning, document string) (string, Diagnostic) {
	pos := located(w.Pos, document)
	d := Diagnostic{Severity: severityWarning, Source: "low", Message: w.Msg, Range: s.text(pos.File).word(pos)}
	return uriOf(pos.File), d
}

func (s *Server) publish(state *entryState, diagnostics map[string][]Diagnostic) error {
	var uris []string
	for uri := range diagnostics {
		uris = append(uris, uri)
	}
	for _, uri := range state.published {
		if _, ok := diagnostics[uri]; !ok {
			diagnostics[uri] = []Diagnostic{}
		}
	}
	state.published = uris
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
	var failure *diag.Error
	if !errors.As(err, &failure) {
		return uriOf(document), d
	}
	pos := located(failure.Pos, document)
	d.Message = failure.Msg
	d.Range = s.text(pos.File).word(pos)
	return uriOf(pos.File), d
}

func (s *Server) query(p TextDocumentPositionParams) *query {
	path := pathOf(p.TextDocument.URI)
	prog := s.stateOf(path).program()
	file := prog.fileAt(path)
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
	return &query{program: prog, index: x, occ: occ}
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
	r := q.resolve()
	content := r.hover()
	if content == "" {
		return nil
	}
	value := fmt.Sprintf("```low\n%s\n```", content)
	if doc := r.doc(); doc != "" {
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
	tg, ok := q.resolve().target()
	if !ok {
		return nil
	}
	return s.locate(tg)
}

func (s *Server) cursorAt(p TextDocumentPositionParams) *cursor {
	path := pathOf(p.TextDocument.URI)
	state := s.stateOf(path)
	t := s.text(path)
	prog := state.program()
	return &cursor{
		program:  prog,
		file:     prog.fileAt(path),
		text:     t,
		compiled: state.compiledText(path),
		line:     p.Position.Line,
		col:      t.byteCol(p.Position.Line, p.Position.Character),
		seen:     map[string]bool{},
	}
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

func (s *Server) documentSymbols(p DocumentParams) []DocumentSymbol {
	return s.symbols(pathOf(p.TextDocument.URI))
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
	return s.stateOf(path).program().fileAt(path)
}
