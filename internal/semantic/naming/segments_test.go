package naming

import "testing"

// TestEscapedSpellsAnIdentityAsQualifiedNameDoes: a basic name stays bare, a reserved word
// included; a name no basic name spells is quoted once, however the identity wrote it.
func TestEscapedSpellsAnIdentityAsQualifiedNameDoes(t *testing.T) {
	cases := map[string]string{
		"P::FunctionToPart::function": "P::FunctionToPart::function",
		"P::Holder::two words":        "P::Holder::'two words'",
		`P::Holder::it's`:             `P::Holder::'it\'s'`,
		`P::Holder::'a::b'`:           `P::Holder::'a::b'`,
		`P::Holder::'@3'`:             `P::Holder::'@3'`,
		`P::Holder::'q\'s'`:           `P::Holder::'q\'s'`,
		"P::Holder::@2::x":            "P::Holder::@2::x",
		"$":                           "$",
	}
	for identity, want := range cases {
		if got := Escaped(identity); got != want {
			t.Errorf("Escaped(%q) = %q, want %q", identity, got, want)
		}
	}
}

// TestUnquoteInvertsSegment: every declared name survives Segment then Unquote.
func TestUnquoteInvertsSegment(t *testing.T) {
	for _, name := range []string{"x", "two words", "a::b", "@3", `it's`, `'q'`, `back\slash`, "function"} {
		if got := Unquote(Segment(name)); got != name {
			t.Errorf("Unquote(Segment(%q)) = %q", name, got)
		}
	}
}

// A declared name reading as an `@N` address is quoted, apart from the unnamed member at
// that position, and the quoting comes off to the spelling the index records.
func TestDeclaredQuotesANameThatReadsAsAPositionalAddress(t *testing.T) {
	cases := map[string]string{
		"Col::W::a":      "Col::W::a",
		"Col::W::@2":     "Col::W::'@2'",
		"Col::W::@2::z":  "Col::W::'@2'::z",
		"Col::W::@x":     "Col::W::@x",
		"Col::W::@2tail": "Col::W::@2tail",
	}
	for recorded, declared := range cases {
		if got := Declared(recorded); got != declared {
			t.Errorf("Declared(%q) = %q, want %q", recorded, got, declared)
		}
		if got := Recorded(declared); got != recorded {
			t.Errorf("Recorded(%q) = %q, want %q", declared, got, recorded)
		}
	}
}
