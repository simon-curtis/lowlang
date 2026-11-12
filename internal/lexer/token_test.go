package lexer

import "testing"

func TestSpellingsRoundTrip(t *testing.T) {
	spelled := append(Keywords(), Operators()...)
	for _, text := range spelled {
		tokens, err := Lex([]byte(text), "spelling")
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
