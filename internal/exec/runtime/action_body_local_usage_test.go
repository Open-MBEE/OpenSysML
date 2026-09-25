package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestActionBodyLocalUsageBindsCurrentValues requires a calc usage declared in an
// action's body to bind its inputs from the values the body has reached, not from
// what the attribute it names declares.
func TestActionBodyLocalUsageBindsCurrentValues(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			calc def Twice { in k : Real; out d = k * 2.0; }
			action run {
				attribute v : Real = 1.0;
				attribute r : Real = 0.0;
				first start;
				action go {
					assign v := 2.0;
					calc t : Twice { in k = v; }
					assign r := t.d;
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

	out, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	// 2.0 is the declared default of v doubled, which the assignment before the
	// usage replaced.
	if got := out["r"].Const.Real; got != 4.0 {
		t.Errorf("r = %v, want 4", got)
	}
}

// TestActionBodyLocalUsageBindsPerIteration requires an action body's loop to bind
// its local usage's inputs anew each iteration, so every iteration computes from
// the values that iteration holds.
func TestActionBodyLocalUsageBindsPerIteration(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			calc def Twice { in k : Real; out d = k * 2.0; }
			action run {
				attribute i : Integer = 0;
				attribute acc : Real = 0.0;
				first start;
				action go {
					while i < 3 {
						assign i := i + 1;
						calc t : Twice { in k = i; }
						assign acc := acc + t.d;
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

	out, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	if got := out["acc"].Const.Real; got != 12.0 {
		t.Errorf("acc = %v, want 12 (2+4+6)", got)
	}
}
