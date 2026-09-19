package lexer

import "testing"

func TestKindString(t *testing.T) {
	cases := map[Kind]string{
		EOF:        "EOF",
		Identifier: "Identifier",
		Decimal:    "Decimal",
		ColonColon: "::",
		LBrace:     "{",
		Error:      "Error",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", k, got, want)
		}
	}
}

func TestTokenIsTrivia(t *testing.T) {
	if !(Token{Kind: Whitespace}).IsTrivia() {
		t.Error("Whitespace should be trivia")
	}
	if !(Token{Kind: SLNote}).IsTrivia() {
		t.Error("SLNote should be trivia")
	}
	if (Token{Kind: Identifier}).IsTrivia() {
		t.Error("Identifier should not be trivia")
	}
	// REGULAR_COMMENT is NOT hidden trivia.
	if (Token{Kind: RegularComment}).IsTrivia() {
		t.Error("RegularComment is not hidden trivia")
	}
}
