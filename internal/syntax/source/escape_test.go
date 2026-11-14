package source

import "testing"

// table4 is the escape sequences of KerML 1.1 Beta 2 §8.2.2.3 Table 4, each
// with the character it names, transcribed from the specification.
var table4 = map[byte]byte{'\'': '\'', '"': '"', 'b': '\b', 'f': '\f', 't': '\t', 'n': '\n', '\\': '\\'}

// The standard escapes are Table 4's seven exactly, and the lexer admits one
// more, the pilot grammar's `\r` (KerMLExpressions.xtext:562-566).
func TestEscapeTableSeparatesTable4FromThePilotExtension(t *testing.T) {
	for c := 0; c < 256; c++ {
		b := byte(c)
		_, standard := table4[b]
		if IsStandardEscapeChar(b) != standard {
			t.Errorf("IsStandardEscapeChar(%q) = %v, want %v", b, !standard, standard)
		}
		if want := standard || b == 'r'; IsEscapeChar(b) != want {
			t.Errorf("IsEscapeChar(%q) = %v, want %v", b, !want, want)
		}
	}
	if got, want := StandardEscapes(), `\b \t \n \f \" \' \\`; got != want {
		t.Errorf("StandardEscapes() = %s, want %s", got, want)
	}
	if got, want := AdmittedEscapes(), `\b \t \n \f \r \" \' \\`; got != want {
		t.Errorf("AdmittedEscapes() = %s, want %s", got, want)
	}
}

// Every admitted escape decodes to the character it names and is written back
// as the same escape, so acceptance, decoding and writing read one table.
func TestEveryAdmittedEscapeDecodesAndIsWrittenBack(t *testing.T) {
	values := map[byte]byte{'r': '\r'}
	for letter, value := range table4 {
		values[letter] = value
	}
	for c := 0; c < 256; c++ {
		letter := byte(c)
		if !IsEscapeChar(letter) {
			continue
		}
		value, ok := values[letter]
		if !ok {
			t.Fatalf("the lexer admits \\%c, which this test does not know", letter)
		}
		raw := string([]byte{'\\', letter})
		if got := Unescape(raw); got != string(value) {
			t.Errorf("Unescape(%q) = %q, want %q", raw, got, string(value))
		}
		if letter == '"' {
			continue // an unrestricted name writes the other quote bare
		}
		if got, want := UnrestrictedNameText(string(value)), "'"+raw+"'"; got != want {
			t.Errorf("UnrestrictedNameText(%q) = %s, want %s", string(value), got, want)
		}
	}
	if got, want := StringText(`"`), `"\""`; got != want {
		t.Errorf("StringText of a double quote = %s, want %s", got, want)
	}
}
