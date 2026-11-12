package lexer

import (
	"testing"

	"lowlang/internal/diag"
)

func TestSpellingsRoundTrip(t *testing.T) {
	spelled := append(Keywords(), Operators()...)
	for _, text := range spelled {
		tokens, err := Lex([]byte(text), diag.Start("spelling"), false)
		if err != nil {
			t.Errorf("%q does not lex: %v", text, err)
			continue
		}
		if len(tokens) == 0 || tokens[0].Kind.String() != text {
			t.Errorf("%q lexes to %v", text, tokens)
		}
	}
}

func TestKeywordsAreKeywords(t *testing.T) {
	for _, word := range Keywords() {
		if !IsKeyword(word) {
			t.Errorf("%s is listed but not a keyword", word)
		}
	}
	if IsKeyword("print") {
		t.Error("print is not a keyword")
	}
}

func TestQuoteRoundTrips(t *testing.T) {
	var every []byte
	for c := range 256 {
		every = append(every, byte(c))
	}
	for _, text := range []string{"", "plain", "\"\\{}\n\r\x00\t'", string(every)} {
		tokens, err := Lex([]byte(Quote(text)), diag.Start("quote"), false)
		if err != nil {
			t.Errorf("Quote(%q) does not lex: %v", text, err)
			continue
		}
		if tokens[0].Kind != String || tokens[0].Text != text {
			t.Errorf("Quote(%q) lexes to %q", text, tokens[0].Text)
		}
	}
}

func TestCharCarriesValue(t *testing.T) {
	tokens, err := Lex([]byte("'a' '\\n' 'é'"), diag.Start("char"), false)
	if err != nil {
		t.Fatal(err)
	}
	var got []rune
	for _, token := range tokens {
		if token.Kind == Char {
			got = append(got, token.Char)
		}
	}
	want := []rune{'a', '\n', 'é'}
	if string(got) != string(want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestInterpolationIsLexedOnce(t *testing.T) {
	tokens, err := Lex([]byte(`"a{x + 1,-8:#,##0}b"`), diag.Start("interp"), false)
	if err != nil {
		t.Fatal(err)
	}
	parts := tokens[0].Parts
	if len(parts) != 3 || parts[0].Text != "a" || parts[2].Text != "b" {
		t.Fatalf("parts %+v", parts)
	}
	expr := parts[1]
	if expr.Tokens[0].Kind != Ident || expr.Tokens[0].Pos.Col != 4 {
		t.Errorf("expression tokens %+v", expr.Tokens)
	}
	if expr.Spec != ",-8:#,##0" || expr.SpecPos.Col != 9 {
		t.Errorf("spec %q at %v", expr.Spec, expr.SpecPos)
	}
}

func TestInterpolationLexErrors(t *testing.T) {
	_, err := Lex([]byte(`"{__x}"`), diag.Start("reserved"), false)
	if err == nil {
		t.Fatal("reserved name in an interpolation lexed")
	}
	if _, err := Lex([]byte(`"{__x}"`), diag.Start("privileged"), true); err != nil {
		t.Errorf("privileged source rejected: %v", err)
	}
}
