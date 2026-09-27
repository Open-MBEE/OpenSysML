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
		{"before_attached_then", `action def A { first start; /* c */ then done; }`, 1},
		{"before_member_then", `action def A { action a; /* c */ then action b; }`, 0},
		{"before_state_attached_then", `state def S { state A; state B; /* c */ then B; }`, 1},
		{"before_state_attached_accept", `state def S { state A; state B; /* c */ accept Go then B; }`, 1},
		{"before_transition_member", `state def S { state A; state B; /* c */ transition first A accept Go then B; }`, 0},
		{"after_result_expression", `calc def F { 1 /* c */ }`, 1},
		{"before_result_expression", `calc def F { /* c */ 1 }`, 0},
		{"body_close_comment", `calc def F { attribute x = 1; /* c */ }`, 0},
		{"inside_expression", `part def P { attribute x = 1 /* c */ + 2; }`, 1},
		{"file_ending_comment", `part a; /* c */`, 0},
		{"empty_body_comment", `part def P { /* c */ }`, 0},
		{"after_doc_body", `package P { doc /* body */ /* note */ }`, 0},
		{"after_comment_body", `package P { comment /* a */ /* b */ }`, 0},
		{"body_expr_param_start", `part def P { attribute y = xs.?{ /* c */ in x; x }; }`, 0},
		{"body_expr_result_start", `part def P { attribute y = xs.?{ in x; /* c */ x }; }`, 0},
		{"body_expr_result_end", `part def P { attribute y = xs.?{ in x; x /* c */ }; }`, 1},
		{"after_nested_body_result", `calc def F { { 1 } /* c */ }`, 1},
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
