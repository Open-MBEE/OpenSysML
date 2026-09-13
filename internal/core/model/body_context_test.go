package model

import (
	"strings"
	"testing"
)

// bodyContextFindings returns a document's diagnostics as "severity: message",
// keeping only those mentioning word.
func bodyContextFindings(t *testing.T, src, word string) []string {
	t.Helper()
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte(src), 1)
	defer ws.Close("a.sysml")
	var out []string
	for _, d := range ws.Diagnostics("a.sysml") {
		if strings.Contains(d.Message, word) {
			out = append(out, d.Severity.String()+": "+d.Message)
		}
	}
	return out
}

// A member legal in one body kind only is still reported beside an unrelated syntax
// error: the syntax tier is not gated and the variant rule is element scoped.
func TestBodyContextFindingsSurviveAnUnrelatedSyntaxError(t *testing.T) {
	const broken = " part def Broken { attribute x : ; } }"
	cases := []struct {
		name string
		src  string
		word string
		want string
	}{
		{"expose in a view def", "package P { view def V { expose P::*; }" + broken, "expose",
			"warning: `expose` in a view def body is an OpenSysML extension"},
		{"transition in a part def", "package P { part def D { state s1; state s2; transition first s1 then s2; }" + broken, "transition",
			"error: 'transition' declares a transition between states and is only allowed in a state body"},
		{"variant outside a variation", "package P { part def D { variant part v : D; }" + broken, "variant",
			"error: A variant must be an owned member of a variation."},
		{"require outside a requirement", "package P { part def D { require constraint { true } }" + broken, "require",
			"warning: `require` outside a requirement body is an OpenSysML extension"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := bodyContextFindings(t, tc.src, "expected a name"); len(got) != 1 {
				t.Fatalf("the unrelated syntax error must be reported once, got %v", got)
			}
			got := bodyContextFindings(t, tc.src, tc.word)
			if len(got) != 1 || !strings.HasPrefix(got[0], tc.want) {
				t.Fatalf("got %v, want one finding starting %q", got, tc.want)
			}
		})
	}
}
