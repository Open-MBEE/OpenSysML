package repl

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestMessageQuality pins the exact text of the diagnostics the prompt reports,
// since the wording, the position and the remedy are the interface.
func TestMessageQuality(t *testing.T) {
	cases := []struct {
		name  string
		decls string
		line  string
		want  []string
	}{{
		// A real failure of an expression of literals is the answer, so an empty
		// session reports it rather than "no declarations loaded".
		name: "division_by_zero_without_declarations",
		line: "%eval 1/0",
		want: []string{"error: evaluation failed: division by zero"},
	}, {
		name:  "division_by_zero_with_declarations",
		decls: "part def Wheel;",
		line:  "%eval 1/0",
		want:  []string{"error: evaluation failed: division by zero"},
	}, {
		name: "unresolved_name_without_declarations",
		line: "%eval missing",
		want: []string{"error: no declarations loaded (literals work, but feature references need declarations)"},
	}, {
		// A unit or a qualified name declarations would answer keeps the
		// no-declarations message too.
		name: "unresolved_unit_without_declarations",
		line: "%eval 1.0 [m]",
		want: []string{"error: no declarations loaded (literals work, but feature references need declarations)"},
	}, {
		name: "unresolved_qualified_name_without_declarations",
		line: "%eval Demo::Vehicle::mass",
		want: []string{"error: no declarations loaded (literals work, but feature references need declarations)"},
	}, {
		// One parser diagnostic, about the expression, with the caret treatment
		// declarations get — not the recovery fallout of a namespace member.
		name: "incomplete_expression",
		line: "%eval 1 +",
		want: []string{"error: 1:4: expected an expression", "1 +", "   ^"},
	}, {
		name:  "incomplete_expression_with_declarations",
		decls: "part def Wheel;",
		line:  "%eval 1 +",
		want:  []string{"error: 1:4: expected an expression", "1 +", "   ^"},
	}, {
		name: "operand_type_mismatch",
		line: `%eval 1 + "a"`,
		want: []string{
			`error: 1:1: type mismatch: operator '+' is not defined for an Integer and a string`,
			`1 + "a"`,
			"^~~~~~~",
		},
	}, {
		name:  "operand_type_mismatch_with_declarations",
		decls: "attribute d = 1;",
		line:  `%eval d + "a"`,
		want: []string{
			`error: 1:1: type mismatch: operator '+' is not defined for an Integer and a string`,
			`d + "a"`,
			"^~~~~~~",
		},
	}, {
		// A mismatch written in a calc keeps the wrapped message that names the
		// calc, rather than a position in the line the user typed.
		name:  "operand_type_mismatch_inside_a_calc",
		decls: "calc def f { in n; n + \"a\" }",
		line:  "%eval f(1)",
		want:  []string{`error: evaluation failed: calc f: `, `operator '+' is not defined for`},
	}, {
		// A name of another kind names its kind in SysML terms, never a Go type.
		name:  "calc_argument_of_wrong_kind",
		decls: "part def Wheel;",
		line:  "%calc Wheel 1",
		want:  []string{"error: not a calc: Wheel is a part def, not a calc definition or usage"},
	}, {
		// A usage error, not a verdict: the model is not what is wrong.
		name:  "constraint_argument_of_wrong_kind",
		decls: "part def Wheel;",
		line:  "%constraint Wheel",
		want:  []string{"error: not a constraint: Wheel is a part def, not a constraint definition or usage"},
	}, {
		name:  "requirement_argument_of_wrong_kind",
		decls: "part def Wheel;",
		line:  "%requirement Wheel",
		want:  []string{"error: not a requirement: Wheel is a part def, not a requirement definition or usage"},
	}, {
		// One wording for a name nothing declares, the parser's and the runtime's.
		name:  "unknown_name",
		decls: "part def Wheel;",
		line:  "%eval nope",
		want:  []string{"error: unresolved reference: nope"},
	}, {
		name:  "unknown_qualified_name",
		decls: "package P { part def Wheel; }",
		line:  "%instantiate P::Nope",
		want:  []string{"error: unresolved reference: P::Nope"},
	}, {
		// The remedy of the other surface is named too, so the two agree.
		name:  "save_to_an_unknown_format",
		decls: "part def Wheel;",
		line:  "%save /tmp/opensysml-message-test.txt",
		want: []string{
			`error: cannot tell the format of "/tmp/opensysml-message-test.txt": expected .sysml, .kerml, .ttl or .json, ` +
				"so name the file with a .sysml, .kerml, .ttl or .json extension, or pass -convert on the command line",
		},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewSession()
			if tc.decls != "" {
				if res := s.Submit(tc.decls); len(res.Diagnostics) > 0 {
					t.Fatalf("declarations have diagnostics: %v", res.Diagnostics)
				}
			}
			got := run(t, s, tc.line)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("expected %q in output:\n%s", want, got)
				}
			}
			if strings.Contains(got, "*ast.") {
				t.Errorf("output names a Go type:\n%s", got)
			}
		})
	}
}

// TestLoadMissingFileReportsPathOnce pins the unwrapped %load failure: the
// operation and the path are each named once.
func TestLoadMissingFileReportsPathOnce(t *testing.T) {
	s := NewSession()
	_, _, err := s.RunMeta("%load /nonexistent/model.sysml")
	if err == nil {
		t.Fatal("expected an error loading a missing file")
	}
	want := "cannot read /nonexistent/model.sysml: no such file or directory"
	if err.Error() != want {
		t.Errorf("err = %q; want %q", err.Error(), want)
	}
	if strings.Count(err.Error(), "/nonexistent/model.sysml") != 1 {
		t.Errorf("path named more than once: %q", err.Error())
	}
}

// TestMismatchCaretOnlyForTypedOperator pins that a caret is drawn only for a
// mismatch written at the prompt: a span landing on prompt text by coincidence,
// from a declaration in another source, keeps the wrapped message.
func TestMismatchCaretOnlyForTypedOperator(t *testing.T) {
	const expr = `1 + "a"`
	cases := []struct {
		name string
		op   string
		span source.Span
		want string
	}{
		{"typed operator", "+", source.Span{Offset: 10, Len: len(expr)}, "1:1: type mismatch"},
		{"offset before the expression", "+", source.Span{Offset: 4, Len: 3}, "evaluation failed:"},
		{"offset past the expression", "+", source.Span{Offset: 40, Len: 3}, "evaluation failed:"},
		{"in range but not the operator written there", "<", source.Span{Offset: 10, Len: len(expr)}, "evaluation failed:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := evalError(expr, &runtime.OperandTypeError{
				Op: tc.op, Left: "an Integer", Right: "a string", Span: tc.span,
			}, 10)
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %q; want it to contain %q", err.Error(), tc.want)
			}
		})
	}
}

// TestDeclarationCaretCountsPrintedCells pins the same cell counting for a
// declaration diagnostic, whose caret and column share the rendering.
func TestDeclarationCaretCountsPrintedCells(t *testing.T) {
	lines := renderResult(NewSession().Submit("attribute αβ = 1 +"), VerbosityNormal)
	got := strings.Join(lines, "\n")
	for _, want := range []string{"1:11: error:", "attribute αβ = 1 +", "          ^~"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output:\n%s", want, got)
		}
	}
}

// TestCaretCountsPrintedCells pins that a caret and the column it is reported
// under are measured in terminal cells, so multi-byte and East Asian wide runes
// before the finding do not push the caret past what it points at.
func TestCaretCountsPrintedCells(t *testing.T) {
	cases := []struct {
		name string
		line string
		want []string
	}{{
		name: "multi-byte runes before the caret",
		line: `%eval "αβγ" +`,
		want: []string{"error: 1:8: expected an expression", `"αβγ" +`, "       ^"},
	}, {
		name: "wide runes count two cells",
		line: `%eval "日本" +`,
		want: []string{"error: 1:9: expected an expression", `"日本" +`, "        ^"},
	}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := run(t, NewSession(), tc.line)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("expected %q in output:\n%s", want, got)
				}
			}
		})
	}
}
