package source

import "testing"

// Unescape decodes every escape KerML §8.2.2 names, stands a backslash before
// anything else for that character, and leaves text with no backslash alone.
func TestUnescape(t *testing.T) {
	cases := []struct{ raw, want string }{
		{`plain text`, "plain text"},
		{`First\nSecond`, "First\nSecond"},
		{`a\tb`, "a\tb"},
		{`It\'s`, "It's"},
		{`back\\slash`, `back\slash`},
		{`say \"hi\"`, `say "hi"`},
		{`a\bb`, "a\bb"},
		{`a\fb`, "a\fb"},
		{`a\rb`, "a\rb"},
		{`a\xb`, "axb"},
		{`end\`, "end\\"},
	}
	for _, tc := range cases {
		if got := Unescape(tc.raw); got != tc.want {
			t.Errorf("Unescape(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}
