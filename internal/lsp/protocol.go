package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/textproto"
	"strconv"
)

type request struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Source   string `json:"source"`
	Message  string `json:"message"`
}

type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    Range         `json:"range"`
}

type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type TextDocumentItem struct {
	URI  string `json:"uri"`
	Text string `json:"text"`
}

type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type DidOpenParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type ContentChange struct {
	Text string `json:"text"`
}

type DidChangeParams struct {
	TextDocument   TextDocumentIdentifier `json:"textDocument"`
	ContentChanges []ContentChange        `json:"contentChanges"`
}

type DocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type SaveOptions struct {
	IncludeText bool `json:"includeText"`
}

type SyncOptions struct {
	OpenClose bool        `json:"openClose"`
	Change    int         `json:"change"`
	Save      SaveOptions `json:"save"`
}

type Capabilities struct {
	TextDocumentSync       SyncOptions `json:"textDocumentSync"`
	HoverProvider          bool        `json:"hoverProvider"`
	DefinitionProvider     bool        `json:"definitionProvider"`
	DocumentSymbolProvider bool        `json:"documentSymbolProvider"`
}

type ServerInfo struct {
	Name string `json:"name"`
}

type InitializeResult struct {
	Capabilities Capabilities `json:"capabilities"`
	ServerInfo   ServerInfo   `json:"serverInfo"`
}

const (
	symbolNamespace  = 3
	symbolField      = 8
	symbolEnum       = 10
	symbolFunction   = 12
	symbolEnumMember = 22
	symbolStruct     = 23
	severityError    = 1
	syncFull         = 1
	methodNotFound   = -32601
	invalidParams    = -32602
)

type stream struct {
	in  *textproto.Reader
	out io.Writer
}

func newStream(in io.Reader, out io.Writer) *stream {
	reader := bufio.NewReader(in)
	return &stream{in: textproto.NewReader(reader), out: out}
}

func (s *stream) body() ([]byte, error) {
	header, err := s.in.ReadMIMEHeader()
	if err != nil {
		return nil, err
	}
	length, err := strconv.Atoi(header.Get("Content-Length"))
	if err != nil {
		return nil, fmt.Errorf("invalid Content-Length: %w", err)
	}
	body := make([]byte, length)
	if _, err := io.ReadFull(s.in.R, body); err != nil {
		return nil, err
	}
	return body, nil
}

func (s *stream) read() (request, error) {
	body, err := s.body()
	if err != nil {
		return request{}, err
	}
	var req request
	if err := json.Unmarshal(body, &req); err != nil {
		return request{}, fmt.Errorf("invalid message: %w", err)
	}
	return req, nil
}

func (s *stream) write(message any) error {
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := io.WriteString(s.out, header); err != nil {
		return err
	}
	_, err = s.out.Write(body)
	return err
}
