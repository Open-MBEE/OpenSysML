package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessUnresolvedNames covers action and state bodies that name a unit or
// feature that does not resolve.
func TestRuntimeRobustnessUnresolvedNames(t *testing.T) {
	t.Run("action_body_unresolved_unit", testActionBodyUnresolvedUnit)
	t.Run("action_body_unresolved_feature", testActionBodyUnresolvedFeature)
	t.Run("state_body_unresolved_unit", testStateBodyUnresolvedUnit)
}

// testActionBodyUnresolvedUnit: an action body is evaluated in the scope it was
// written in, and a unit that scope does not bring in resolves to nothing — the
// quantity is reported as such rather than evaluated as its bare magnitude.
func testActionBodyUnresolvedUnit(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import ScalarValues::*;
			action descend {
				attribute h : Real = 500.0 [furlong];
				first start;
				done;
				succession first start then done;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "descend", ast.DefAction)
	if sym == nil {
		t.Fatal("action descend not found")
	}

	out, err := ctx.ExecuteAction(sym)
	if !errors.Is(err, errNotAQuantity) {
		t.Fatalf("outputs = %v, err = %v; want errNotAQuantity", out, err)
	}
	if !strings.Contains(err.Error(), semantics.ErrNotAUnit.Error()) {
		t.Errorf("err = %v; want it to report that the index names no measurement unit", err)
	}
}

// testActionBodyUnresolvedFeature: a name no frame, object or enclosing scope
// supplies is reported as unresolved, so giving a body its declaring scope does
// not turn a typo into a silent zero.
func testActionBodyUnresolvedFeature(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			action counter {
				attribute total : Integer = 0;
				first start;
				action bump {
					assign total := missingName + 1;
				}
				done;
				succession first start then bump;
				succession first bump then done;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "counter", ast.DefAction)
	if sym == nil {
		t.Fatal("action counter not found")
	}

	out, err := ctx.ExecuteAction(sym)
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("outputs = %v, err = %v; want ErrUnresolvedReference", out, err)
	}
	if !strings.Contains(err.Error(), "missingName") {
		t.Errorf("err = %v; want it to name the unresolved feature", err)
	}
}

// testStateBodyUnresolvedUnit: the same for a state machine's attribute default,
// which is evaluated when the machine initializes.
func testStateBodyUnresolvedUnit(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import ScalarValues::*;
			state monitor {
				attribute speed : Real = 1.5 [knot];
				entry; then start;
				state start;
				state running;
				succession first start then running;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "monitor", ast.DefState)
	if sym == nil {
		t.Fatal("state monitor not found")
	}

	_, err := ctx.ExecuteState(sym)
	if !errors.Is(err, errNotAQuantity) {
		t.Fatalf("err = %v; want errNotAQuantity", err)
	}
}
