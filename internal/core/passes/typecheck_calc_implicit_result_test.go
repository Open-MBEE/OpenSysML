package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// libraryTypeDiags analyzes src against the standard library, so a check keyed
// off library types — the dimensional warning — is exercised as written.
func libraryTypeDiags(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()

	idx := newTestIndex()
	root := parser.New(source.New("<t>", []byte(src))).ParseFile()
	idx.AddDocument("<t>", root)
	idx.ExpandWildcardImports()

	var out []diag.Diagnostic
	for _, d := range Analyze("<t>", root, nil, idx) {
		if d.Source == "type" {
			out = append(out, d)
		}
	}
	return out
}

// TestCalcBodyImplicitResultIsChecked: a body whose result is the value of its
// last expression states that expression as a member, and the type tier must
// reach it as it reaches an explicit `return`.
func TestCalcBodyImplicitResultIsChecked(t *testing.T) {
	wantOneDiag(t, `package P {
		calc def C {
			in n : ScalarValues::Integer;
			n + "one"
		}
	}`, "operator '+' is not defined for Integer and String")
}

// TestCalcBodyImplicitResultCleanIsClean: reaching the expression must not
// invent a diagnostic for a well-typed implicit result.
func TestCalcBodyImplicitResultCleanIsClean(t *testing.T) {
	wantNoDiags(t, `package P {
		calc def C {
			in n : ScalarValues::Integer;
			n * 2
		}
	}`)
}

// TestCalcBodyImplicitResultAfterStatementsIsChecked: the implicit result is
// typed in the body's scope, so it reads the locals the statements before it
// declare.
func TestCalcBodyImplicitResultAfterStatementsIsChecked(t *testing.T) {
	wantOneDiag(t, `package P {
		calc def C {
			in n : ScalarValues::Integer;
			attribute flag : ScalarValues::Boolean = true;
			n + flag
		}
	}`, "operator '+' is not defined for Integer and Boolean")
}

// TestCalcUsageImplicitResultIsChecked: a calc usage states its body the same
// way a definition does, so its implicit result is typed too.
func TestCalcUsageImplicitResultIsChecked(t *testing.T) {
	wantOneDiag(t, `package P {
		calc c {
			in n : ScalarValues::Integer;
			n + "one"
		}
	}`, "operator '+' is not defined for Integer and String")
}

// TestCalcBodyImplicitResultSkippedAfterNameError: the tier contract holds for
// the newly reached expression — a name the lower tier already reported is not
// typed on top of that error.
func TestCalcBodyImplicitResultSkippedAfterNameError(t *testing.T) {
	if diags := exprDiags(t, `package P {
		calc def C {
			in n : ScalarValues::Integer;
			n + missingName
		}
	}`); len(diags) != 0 {
		t.Fatalf("expected no type diagnostics over an unresolved name, got %v", diags)
	}
}

// TestCalcBodyImplicitResultWarnsOnDimensions: the reported A9 case — the
// dimensional warning inside a body whose result is its last expression.
func TestCalcBodyImplicitResultWarnsOnDimensions(t *testing.T) {
	diags := libraryTypeDiags(t, `package P {
		private import ISQ::*;
		private import SI::*;
		calc def Compare {
			in mass : ISQ::MassValue;
			mass < 1000.0 [m]
		}
	}`)
	var warnings []diag.Diagnostic
	for _, d := range diags {
		if strings.Contains(d.Message, "incommensurable quantities") {
			if d.Severity != diag.SeverityWarning {
				t.Errorf("dimensional diagnostic is %v, want a warning: %s", d.Severity, d.Message)
			}
			warnings = append(warnings, d)
		} else if d.Severity == diag.SeverityError {
			t.Errorf("unexpected type error: %s", d.Message)
		}
	}
	if len(warnings) != 1 {
		t.Fatalf("want one dimensional warning inside the implicit-result body, got %v", diags)
	}
	if !strings.Contains(warnings[0].Message, "dimension L") {
		t.Errorf("warning %q does not name the clashing dimension", warnings[0].Message)
	}
}
