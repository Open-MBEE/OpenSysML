package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
)

// TestActionBodyActivationEndsWithTheBody requires what a calc usage read in an
// action node's body computed to be discarded when that execution of the body
// ends: a run stepping a body many times must not hold every execution's values.
func TestActionBodyActivationEndsWithTheBody(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			calc def Twice { in k : Real; out d = k * 2.0; }
			action run {
				attribute v : Real = 0.0;
				calc t : Twice { in k = 3.0; }
				first start;
				action go { assign v := t.d; }
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

	out, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if got := out["v"].Const.Real; got != 6.0 {
		t.Errorf("v = %v, want 6", got)
	}
	if len(ctx.run.calcUsageRuns) != 0 {
		t.Errorf("%d activation(s) still held after the run: %v", len(ctx.run.calcUsageRuns), ctx.run.calcUsageRuns)
	}
}
