package lexer

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// TestStringValue reads the escapes KerML §8.2.2 defines, and leaves text
// carrying no escape as it was written.
func TestStringValue(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{`"abc"`, "abc"},
		{`'abc'`, "abc"},
		{`""`, ""},
		{`"a\nb"`, "a\nb"},
		{`"a\tb"`, "a\tb"},
		{`"a\rb"`, "a\rb"},
		{`"a\bb"`, "a\bb"},
		{`"a\fb"`, "a\fb"},
		{`"say \"hi\""`, `say "hi"`},
		{`"a\\b"`, `a\b`},
		{`"it\'s"`, "it's"},
		{`"héllo 🚗"`, "héllo 🚗"},
		{`"trailing\"`, `trailing\`},
		{"unquoted", "unquoted"},
	}
	for _, tt := range tests {
		if got := source.StringValue(tt.raw); got != tt.want {
			t.Errorf("source.StringValue(%s) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

// TestStringTextIsReadBackByStringValue escapes every character the notation
// cannot carry bare, and no other, so a value survives being written and read.
func TestStringTextIsReadBackByStringValue(t *testing.T) {
	for _, tt := range []struct {
		value string
		want  string
	}{
		{"", `""`},
		{"abc", `"abc"`},
		{"say \"hi\"", `"say \"hi\""`},
		{`a\b`, `"a\\b"`},
		{"a\nb\rc\td\be\ff", `"a\nb\rc\td\be\ff"`},
		{"it's", `"it's"`},
		{"héllo 🚗", `"héllo 🚗"`},
	} {
		got := source.StringText(tt.value)
		if got != tt.want {
			t.Errorf("source.StringText(%q) = %s, want %s", tt.value, got, tt.want)
		}
		if back := source.StringValue(got); back != tt.value {
			t.Errorf("source.StringValue(StringText(%q)) = %q", tt.value, back)
		}
	}
}
