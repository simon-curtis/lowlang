package lsp

import (
	"fmt"
	"slices"
	"strings"

	"lowlang/internal/types"
)

type opening struct {
	char   byte
	line   int
	col    int
	commas int
}

type callShape struct {
	head   string
	params []string
	tail   string
}

func (c *cursor) openings() []opening {
	from, _ := c.headerLine()
	var stack []opening
	for line := from; line <= c.line; line++ {
		src := c.text.line(line)
		limit := len(src)
		if line == c.line {
			limit = min(c.col, len(src))
		}
		stack = scanLine(src[:limit], line, stack)
	}
	return stack
}

func scanLine(src string, line int, stack []opening) []opening {
	for i := 0; i < len(src); i++ {
		switch ch := src[i]; {
		case ch == '/' && i+1 < len(src) && src[i+1] == '/':
			return stack
		case ch == '"' || ch == '\'':
			i = skipQuoted(src, i)
		case ch == '(' || ch == '[' || ch == '{':
			stack = append(stack, opening{char: ch, line: line, col: i})
		case ch == ')' || ch == ']' || ch == '}':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case ch == ',' && len(stack) > 0:
			stack[len(stack)-1].commas++
		}
	}
	return stack
}

func skipQuoted(src string, at int) int {
	quote := src[at]
	for i := at + 1; i < len(src); i++ {
		switch src[i] {
		case '\\':
			i++
		case quote:
			return i
		}
	}
	return len(src)
}

func (c *cursor) openCall() (opening, bool) {
	stack := c.openings()
	for i := len(stack) - 1; i >= 0; i-- {
		if stack[i].char == '(' {
			return stack[i], true
		}
	}
	return opening{}, false
}

func calleeChain(src string, paren int) []string {
	end := paren
	if end > 0 && src[end-1] == ']' {
		depth := 0
		for end > 0 {
			end--
			switch src[end] {
			case ']':
				depth++
			case '[':
				depth--
			}
			if depth == 0 {
				break
			}
		}
	}
	return chainBefore(src, end)
}

func (c *cursor) funcShape(module, name string, public bool) (callShape, bool) {
	for _, f := range c.files(module) {
		candidates := slices.Clone(f.Funcs)
		for _, e := range f.Externs {
			candidates = append(candidates, e.Funcs...)
		}
		for _, fn := range candidates {
			if fn.Name == name && fn.Template == nil && fn.Closure == nil && (fn.Pub || !public) {
				head, params, tail := signatureParts(fn)
				return callShape{head: head, params: params, tail: tail}, true
			}
		}
	}
	return callShape{}, false
}

func (c *cursor) variantShape(module, enum, name string) (callShape, bool) {
	for _, f := range c.files(module) {
		for _, d := range f.Enums {
			if enum != "" && d.Name != enum {
				continue
			}
			for _, v := range d.Variants {
				if v.Name != name || len(v.Fields) == 0 {
					continue
				}
				params := make([]string, len(v.Fields))
				for i, field := range v.Fields {
					params[i] = typeNameString(field)
				}
				head := fmt.Sprintf("%s.%s(", d.Name, v.Name)
				return callShape{head: head, params: params, tail: ")"}, true
			}
		}
	}
	return callShape{}, false
}

func valueShape(name string, t *types.Type) (callShape, bool) {
	if t == nil || t.Kind != types.FuncKind {
		return callShape{}, false
	}
	params := make([]string, len(t.Params))
	for i, p := range t.Params {
		params[i] = p.String()
		if i < len(t.Muts) && t.Muts[i] {
			params[i] = fmt.Sprintf("mut %s", params[i])
		}
	}
	tail := ")"
	if t.Ret != nil && t.Ret != types.Void {
		tail = fmt.Sprintf(") -> %s", t.Ret)
	}
	return callShape{head: fmt.Sprintf("%s(", name), params: params, tail: tail}, true
}

func (c *cursor) shape(chain []string) (callShape, bool) {
	module := c.file.Module
	switch len(chain) {
	case 1:
		if local := c.local(chain[0]); local != nil {
			return valueShape(chain[0], local.Type)
		}
		if shape, ok := c.funcShape(module, chain[0], false); ok {
			return shape, true
		}
		return c.variantShape(module, "", chain[0])
	case 2:
		if path, ok := c.file.Imported(chain[0]); ok {
			return c.funcShape(path, chain[1], true)
		}
		return c.variantShape(module, chain[0], chain[1])
	case 3:
		if path, ok := c.file.Imported(chain[0]); ok {
			return c.variantShape(path, chain[1], chain[2])
		}
	}
	return callShape{}, false
}

func (c *cursor) signatureHelp() *SignatureHelp {
	if c.file == nil {
		return nil
	}
	call, ok := c.openCall()
	if !ok {
		return nil
	}
	src := c.text.line(call.line)
	chain := calleeChain(src, call.col)
	if len(chain) == 0 {
		return nil
	}
	shape, ok := c.shape(chain)
	if !ok {
		return nil
	}
	return shape.help(call.commas)
}

func (s callShape) help(commas int) *SignatureHelp {
	var label strings.Builder
	label.WriteString(s.head)
	params := make([]ParameterInformation, len(s.params))
	for i, p := range s.params {
		if i > 0 {
			label.WriteString(", ")
		}
		start := label.Len()
		label.WriteString(p)
		params[i] = ParameterInformation{Label: [2]int{start, label.Len()}}
	}
	label.WriteString(s.tail)
	active := min(commas, max(len(params)-1, 0))
	info := SignatureInformation{Label: label.String(), Parameters: params}
	return &SignatureHelp{Signatures: []SignatureInformation{info}, ActiveParameter: active}
}
