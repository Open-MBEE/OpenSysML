package passes

import (
	"strings"
	"testing"
)

// Expose is a ViewBodyItem alone (SysML.xtext): silent in a view usage, a notation
// warning in a view def body, a parser error elsewhere so no syntax error hides it.
func TestExposeOwningBody(t *testing.T) {
	cases := []struct {
		name      string
		src       string
		parseErr  bool
		notation  int
		wantsBody string
	}{
		{"view usage", "package P { part p; view v { expose P::**; } }", false, 0, ""},
		{"view def", "package P { part p; view def V { expose P::**; } }", false, 1, "view def body"},
		{"part def", "package P { part p; part def D { expose P::**; } }", true, 0, "view usage body"},
		{"part usage", "package P { part p; part q { expose P::**; } }", true, 0, "view usage body"},
		{"nested in a view usage", "package P { part p; view v { view w { expose P::**; } } }", false, 0, ""},
		{"plain import is untouched", "package P { part p; part def D { import P::*; } }", false, 0, ""},
		{"view def beside a syntax error", "package P { part p; view def V { expose P::**; } part def E { attribute x = ; } }", true, 1, "view def body"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, pd, idx := analyzeInputs(t, "a.sysml", tc.src)
			if hasParseError(pd) != tc.parseErr {
				t.Fatalf("parse errors = %+v, want error: %v", pd, tc.parseErr)
			}
			if tc.parseErr && tc.notation == 0 {
				if !strings.Contains(pd[0].Message, "'expose' declares what a view usage exposes and is only allowed in a "+tc.wantsBody) {
					t.Errorf("parser message = %q, want it to name the body that admits expose", pd[0].Message)
				}
			}
			got := NonstandardNotationPass{}.Run(NewContext("a.sysml", idx, pd), "a.sysml", root)
			if len(got) != tc.notation {
				t.Fatalf("got %d notation diagnostics, want %d: %v", len(got), tc.notation, got)
			}
			for _, d := range got {
				if d.Code != CodeNonstandardNotation || d.Severity != SeverityWarning {
					t.Errorf("got %+v, want a nonstandard-notation warning", d)
				}
				if !strings.Contains(d.Message, tc.wantsBody) {
					t.Errorf("message = %q, want it to name the %s", d.Message, tc.wantsBody)
				}
			}
		})
	}
}
