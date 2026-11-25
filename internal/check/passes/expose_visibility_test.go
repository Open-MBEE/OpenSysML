package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// exposeVisibilityDiags returns the expose-visibility findings of src.
func exposeVisibilityDiags(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	var out []diag.Diagnostic
	for _, d := range analyzeSrc(t, src) {
		if d.Code == CodeExposeVisibility {
			out = append(out, d)
		}
	}
	return out
}

// An Expose has no visibility prefix (SysML v2 validateExposeVisibility: it is
// always protected); a prefixed one is reported, a bare one and an import are not.
func TestExposeVisibilityIndicatorRejected(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		count int
	}{
		{"bare", "package P { part def D; } view def VD; view v : VD { expose P::*; }", 0},
		{"public", "package P { part def D; } view def VD; view v : VD { public expose P::*; }", 1},
		{"private", "package P { part def D; } view def VD; view v : VD { private expose P::*; }", 1},
		{"protected", "package P { part def D; } view def VD; view v : VD { protected expose P::*; }", 1},
		{"recursive", "package P { part def D; } view def VD; view v : VD { public expose P::**; }", 1},
		{"two exposes", "package P { part def D; } view def VD; view v : VD { public expose P::*; private expose P::D; }", 2},
		{"import beside", "package P { part def D; } view def VD; view v : VD { private import P::*; expose P::*; }", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diags := exposeVisibilityDiags(t, tc.src)
			if len(diags) != tc.count {
				t.Fatalf("got %d expose-visibility diagnostics, want %d: %v", len(diags), tc.count, diags)
			}
			for _, d := range diags {
				if d.Severity != diag.SeverityError || !d.Notation {
					t.Fatalf("expose-visibility must be a notation error after analysis, got %+v", d)
				}
			}
		})
	}
}

// The parser carries the finding as a warning on the visibility keyword, with the
// expose still read as protected, so a direct parser consumer sees it before any pass.
func TestExposeVisibilityIsAParserWarning(t *testing.T) {
	src := "package P { part def D; } view def VD; view v : VD { public expose P::*; }"
	p := parser.New(source.New("<t>", []byte(src)))
	p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse errors = %v, want none: the prefixed form still reads", p.Diagnostics)
	}
	var found *parser.Diagnostic
	for i := range p.Warnings {
		if p.Warnings[i].Code == CodeExposeVisibility {
			found = &p.Warnings[i]
		}
	}
	if found == nil {
		t.Fatalf("parser reported no expose-visibility warning: %v", p.Warnings)
	}
	if got := src[found.Span.Offset:found.Span.End()]; got != "public" {
		t.Fatalf("warning spans %q, want the visibility keyword", got)
	}
	if !strings.Contains(found.Message, "always protected") {
		t.Fatalf("message %q does not explain the fixed visibility", found.Message)
	}
}

// The escalation changes severity only: the exposed members still resolve, so a
// reference through the view is not additionally unresolved.
func TestExposeVisibilityDoesNotGateHigherTiers(t *testing.T) {
	src := "package P { part def D; } view def VD; view v : VD { public expose P::*; } part x : P::D;"
	for _, d := range analyzeSrc(t, src) {
		if d.Code != CodeExposeVisibility {
			t.Fatalf("unexpected diagnostic beside expose-visibility: %+v", d)
		}
	}
}
