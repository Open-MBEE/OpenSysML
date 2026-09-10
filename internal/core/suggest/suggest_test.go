package suggest_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/suggest"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// TestEditDistance locks the distances the suggestions rank on, including the
// transposition that a swapped pair of letters is one edit away.
func TestEditDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{a: "eval", b: "eval", want: 0},
		{a: "evl", b: "eval", want: 1},
		{a: "eavl", b: "eval", want: 1},
		{a: "isntances", b: "instances", want: 1},
		{a: "", b: "eval", want: 4},
		{a: "abcd", b: "", want: 4},
		{a: "wheel", b: "whel", want: 1},
		{a: "cat", b: "dog", want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.a+"/"+tt.b, func(t *testing.T) {
			if got := suggest.EditDistance(tt.a, tt.b); got != tt.want {
				t.Errorf("suggest.EditDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// TestNearest covers the tolerance a typo of each length gets, and the order
// the candidates come back in.
func TestNearest(t *testing.T) {
	names := []string{"Integer", "Real", "String", "instances", "eval"}
	tests := []struct {
		word string
		want []string
	}{
		{word: "Intger", want: []string{"Integer"}},
		{word: "isntances", want: []string{"instances"}},
		{word: "Rael", want: []string{"Real"}},
		{word: "eval", want: nil}, // a name that is itself is not a suggestion
		{word: "Zzzzqqqqwwww", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.word, func(t *testing.T) {
			got := suggest.Nearest(tt.word, names)
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("suggest.Nearest(%q) = %v, want %v", tt.word, got, tt.want)
			}
		})
	}
}

// TestQualifiedOverLibrary covers the qualified spellings the bundled library
// offers for a bare name, and that a re-export is not offered as a declaration.
func TestQualifiedOverLibrary(t *testing.T) {
	idx := libraryIndex(t)
	tests := []struct {
		name    string
		want    string
		rejects []string
	}{
		{name: "Integer", want: "ScalarValues::Integer", rejects: []string{"SysML::Integer", "KerML::Integer"}},
		{name: "String", want: "ScalarValues::String"},
		{name: "Zzzznotatype", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := suggest.Qualified(idx, tt.name)
			if tt.want == "" {
				if len(got) > 0 {
					t.Fatalf("suggest.Qualified(%q) = %v, want none", tt.name, got)
				}
				return
			}
			if len(got) == 0 || got[0] != tt.want {
				t.Fatalf("suggest.Qualified(%q) = %v, want %q first", tt.name, got, tt.want)
			}
			for _, bad := range tt.rejects {
				for _, c := range got {
					if c == bad {
						t.Errorf("suggest.Qualified(%q) offered the re-export %q", tt.name, bad)
					}
				}
			}
		})
	}
}

// TestWithAndOrList covers how a suggestion reads, and that a candidate equal
// to the word it explains is not offered back.
func TestWithAndOrList(t *testing.T) {
	tests := []struct {
		name       string
		word       string
		candidates []string
		want       string
	}{
		{name: "none", word: "x", want: "unresolved reference: x"},
		{name: "one", word: "x", candidates: []string{"A::x"}, want: "unresolved reference: x — did you mean A::x?"},
		{
			name:       "several",
			word:       "x",
			candidates: []string{"A::x", "B::x", "C::x"},
			want:       "unresolved reference: x — did you mean A::x, B::x or C::x?",
		},
		{name: "itself", word: "x", candidates: []string{"x"}, want: "unresolved reference: x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := suggest.With("unresolved reference: "+tt.word, tt.word, tt.candidates)
			if got != tt.want {
				t.Errorf("suggest.With(...) = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestUnquoted covers which registered names an identifier is the unquoted
// start of: those that go on with a character no basic name holds, and only
// those actually given — never a spelling made up from the word.
func TestUnquoted(t *testing.T) {
	names := []string{"SA", "SA-506", "SA-507", "SA_506", "SAT", "SA 506", "HLR-R001"}
	tests := []struct {
		word string
		want []string
	}{
		{word: "SA", want: []string{"SA-506", "SA-507", "SA 506"}},
		{word: "SA-506", want: nil}, // not an identifier: it is already what was typed
		{word: "HLR", want: []string{"HLR-R001"}},
		{word: "SAT", want: nil},
		{word: "S", want: nil},
		{word: "", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.word, func(t *testing.T) {
			got := suggest.Unquoted(tt.word, names)
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("suggest.Unquoted(%q) = %v, want %v", tt.word, got, tt.want)
			}
		})
	}
}

// TestQuotingRule covers the characters the rule names: each one a segment
// holds that a basic name cannot, once, in the order met, and never the `::`
// between segments.
func TestQuotingRule(t *testing.T) {
	tests := []struct {
		name  string
		names []string
		want  string
	}{
		{name: "none", names: nil, want: ""},
		{name: "identifiers", names: []string{"T::SA"}, want: ""},
		{name: "hyphen", names: []string{"T::SA-506"}, want: "Names containing '-' must be quoted."},
		{name: "several", names: []string{"SA-506", "SA 506", "A.B-1"}, want: "Names containing '-', ' ' or '.' must be quoted."},
		{name: "leading digit", names: []string{"1st"}, want: "Names containing '1' must be quoted."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := suggest.QuotingRule(tt.names); got != tt.want {
				t.Errorf("suggest.QuotingRule(%v) = %q, want %q", tt.names, got, tt.want)
			}
		})
	}
}

// TestHint covers how the names a word is the unquoted start of read: quoted as
// the notation needs, before the ordinary candidates, with the rule after the
// question, and nothing added when there are none.
func TestHint(t *testing.T) {
	tests := []struct {
		name       string
		word       string
		candidates []string
		unquoted   []string
		want       string
	}{
		{name: "none", word: "SA", want: "unresolved reference: SA"},
		{
			name:     "qualified",
			word:     "T::SA",
			unquoted: []string{"T::SA-506"},
			want:     "unresolved reference: T::SA — did you mean T::'SA-506'? Names containing '-' must be quoted.",
		},
		{
			name:       "beside candidates",
			word:       "SA",
			candidates: []string{"SAT"},
			unquoted:   []string{"SA-506", "T::SA 506"},
			want:       "unresolved reference: SA — did you mean 'SA-506', T::'SA 506' or SAT? Names containing '-' or ' ' must be quoted.",
		},
		{
			name:       "candidates only",
			word:       "SAT",
			candidates: []string{"SA", "SAX"},
			want:       "unresolved reference: SAT — did you mean SA or SAX?",
		},
		{
			name:     "limit",
			word:     "SA",
			unquoted: []string{"SA-1", "SA-2", "SA-3", "SA-4"},
			want:     "unresolved reference: SA — did you mean 'SA-1', 'SA-2' or 'SA-3'? Names containing '-' must be quoted.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := suggest.Hint("unresolved reference: "+tt.word, tt.word, tt.candidates, tt.unquoted)
			if got != tt.want {
				t.Errorf("suggest.Hint(...) = %q, want %q", got, tt.want)
			}
		})
	}
}

// Notation quotes the segments no basic name spells and leaves a keyword the
// library writes bare as it is, so `TriggerKind::when` is offered as written.
func TestNotation(t *testing.T) {
	tests := map[string]string{
		"":                                  "",
		"SA-506":                            "'SA-506'",
		"T::SA-506":                         "T::'SA-506'",
		"My Pkg::Car":                       "'My Pkg'::Car",
		"SysML::Systems::TriggerKind::when": "SysML::Systems::TriggerKind::when",
		"BaseFunctions::#::index":           "BaseFunctions::'#'::index",
	}
	for fqn, want := range tests {
		if got := suggest.Notation(fqn); got != want {
			t.Errorf("suggest.Notation(%q) = %q, want %q", fqn, got, want)
		}
	}
}

// libraryIndex indexes the bundled standard library.
func libraryIndex(t *testing.T) *symbols.Index {
	t.Helper()
	idx := symbols.NewIndex()
	libs.LoadInto(idx)
	return idx
}
