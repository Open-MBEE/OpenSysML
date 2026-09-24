package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// guardModel loops through a decision whose guards read a calc usage over an
// attribute the loop's action assigns, so the loop only ends if each decision
// reads the usage anew.
const guardModel = `
	package test {
		private import ScalarValues::*;
		calc def Twice { in k : Real; out d = k * 2.0; }
		action run {
			attribute i : Integer = 0;
			attribute log : Real = 0.0;
			calc t : Twice { in k = i; }
			first start;
			action bump {
				assign i := i + 1;
				assign log := log + t.d;
			}
			decide check;
			done;
			succession first start then bump;
			succession first bump then check;
			succession first check if t.d < 6.0 then bump;
			succession first check if t.d >= 6.0 then done;
		}
	}
`

// TestDecisionGuardReadsCalcUsagePerStep requires a guard's read of a calc usage
// to belong to the step that made it: the next decision reads the usage again and
// sees what the action assigned in between.
func TestDecisionGuardReadsCalcUsagePerStep(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, guardModel))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "run", ast.DefAction)
	if sym == nil {
		t.Fatal("action run not found")
	}

	out, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if got := out["i"].Const.Int; got != 3 {
		t.Errorf("i = %v, want 3", got)
	}
	if got := out["log"].Const.Real; got != 12.0 {
		t.Errorf("log = %v, want 12 (2 + 4 + 6)", got)
	}
}

// TestDecisionGuardsShareOneCalcUsageEvaluation requires the guards of one
// decision to read one evaluation of the usage they both name, so the branch they
// choose cannot rest on two bindings.
func TestDecisionGuardsShareOneCalcUsageEvaluation(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, guardModel))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "run", ast.DefAction)
	if sym == nil {
		t.Fatal("action run not found")
	}
	tr := NewTraceRecorder()
	ctx.SetTrace(tr)

	if _, err := ctx.ExecuteAction(sym); err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	ctx.SetTrace(nil)

	// Three iterations, each running the usage's body once for the assignment and
	// once for the decision, whose second guard reads that same evaluation.
	trace := tr.String()
	if got := strings.Count(trace, "enter calc test::run::t"); got != 6 {
		t.Errorf("usage body ran %d times, want 6 (one per assignment and one per decision)", got)
	}
	if !strings.Contains(trace, "reuse calc test::run::t") {
		t.Error("the guards of one decision must read one evaluation of the usage")
	}
}

// TestPartChainReadBelongsToTheReadingActivation requires a read through a part's
// feature chain to belong to the evaluation making it, so a later iteration reads
// the usage again rather than answering from what an earlier one computed.
func TestPartChainReadBelongsToTheReadingActivation(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			calc def Twice { in k : Real; out d = k * 2.0; }
			part lander {
				attribute base : Real = 3.0;
				calc mass : Twice { in k = base; }
			}
			action run {
				attribute i : Integer = 0;
				attribute acc : Real = 0.0;
				first start;
				action go {
					while i < 3 {
						assign i := i + 1;
						assign acc := acc + lander.mass.d;
					}
				}
				done;
				succession first start then go;
				succession first go then done;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "run", ast.DefAction)
	if sym == nil {
		t.Fatal("action run not found")
	}
	tr := NewTraceRecorder()
	ctx.SetTrace(tr)

	out, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	ctx.SetTrace(nil)

	if got := out["acc"].Const.Real; got != 18.0 {
		t.Errorf("acc = %v, want 18 (three reads of 6)", got)
	}
	if got := strings.Count(tr.String(), "enter calc test::lander::mass"); got != 3 {
		t.Errorf("usage body ran %d times, want 3 (once per iteration reading it)", got)
	}
}
