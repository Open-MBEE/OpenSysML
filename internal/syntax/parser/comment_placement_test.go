package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// commentWarnings returns the nonstandard-notation warnings and parse errors
// of a parse, so a case can assert on placement without hiding an error.
func commentWarnings(t *testing.T, input string) (warnings []Diagnostic, errs []string) {
	t.Helper()
	p := New(source.New("a.sysml", []byte(input)))
	p.ParseFile()
	for _, w := range p.Warnings {
		if w.Code == codeNonstandardNotation {
			warnings = append(warnings, w)
		}
	}
	for _, d := range p.Diagnostics {
		errs = append(errs, d.Message)
	}
	return warnings, errs
}

// A regular comment is a Comment element, which the grammar admits only where
// a member may start, at the close of a body, or as a comment notation's body;
// anywhere else the parser reads it as trivia but warns.
func TestCommentPlacement(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  int
	}{
		{"between_trigger_and_then", `state def S { state Idle; state Off; transition first Idle accept Stop /* c */ then Off; }`, 1},
		{"member_position", `package P { /* c */ part a; part b; /* d */ }`, 0},
		{"comment_notation_body", `package P { comment /* c */ part a; doc /* d */ }`, 0},
		{"before_member_then", `action def A { first start; /* c */ then done; }`, 0},
		{"inside_expression", `part def P { attribute x = 1 /* c */ + 2; }`, 1},
		{"file_ending_comment", `part a; /* c */`, 0},
		{"empty_body_comment", `part def P { /* c */ }`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			warnings, errs := commentWarnings(t, tc.input)
			if len(errs) != 0 {
				t.Fatalf("parse errors %v, want none", errs)
			}
			if len(warnings) != tc.want {
				t.Fatalf("got %d nonstandard-notation warnings %+v, want %d", len(warnings), warnings, tc.want)
			}
			for _, w := range warnings {
				text := source.New("a.sysml", []byte(tc.input)).Text(w.Span)
				if !strings.HasPrefix(text, "/*") {
					t.Errorf("warning span %v covers %q, want the comment", w.Span, text)
				}
			}
		})
	}
}
