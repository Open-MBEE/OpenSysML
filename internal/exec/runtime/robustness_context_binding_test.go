package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessRedefiningRefBinding exercises a redefinition binding a
// reference parameter to an instance no type it declares could hold: the
// declared value is checked against the type the redefined parameter states.
func TestRuntimeRobustnessRedefiningRefBinding(t *testing.T) {
	t.Run("instance_of_an_unrelated_type", testRedefiningRefBindsUnrelatedInstance)
}

const redefiningRefFixture = `
	package test {
		private import ScalarValues::*;
		part def Tank {}
		part def Env {
			part t : Tank;
			action def Warm { in ref context : Heater; first step; action step { assign context.level := 1; } }
			action def Go {
				first warm;
				action warm : Warm { in ref :>> context = Env::t; }
				first warm then done;
			}
			action go : Go;
		}
		part def Heater { attribute level : Integer = 0; }
	}
`

// testRedefiningRefBindsUnrelatedInstance: `in ref :>> context = Env::t` binds a
// Tank where the redefined parameter declares Heater — the binding states an
// identity, but an identity the declared type does not admit.
func testRedefiningRefBindsUnrelatedInstance(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, redefiningRefFixture))
	env := findSymbolByName(idx.DocumentRoot("<test>"), "Env", ast.DefPart)
	inst, err := ctx.Instantiate(env)
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	goAction := oneSymbol(t, idx, "test::Env::Go")
	_, err = ctx.ExecuteActionPerformedBy(goAction, inst, nil)
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("binding a Tank to context : Heater: %v, want ErrTypeMismatch", err)
	}
}
