package runtime

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessBodyAnnotations covers metadata annotations written among
// the members of an executable body: they annotate the body and are no step of
// it, whatever they are typed by or bind, so the body still runs to its result.
func TestRuntimeRobustnessBodyAnnotations(t *testing.T) {
	t.Run("annotation_typed_by_nothing_resolvable", testAnnotationTypedByNothingResolvable)
	t.Run("annotation_binding_an_unbound_name", testAnnotationBindingAnUnboundName)
	t.Run("annotation_beside_a_then_chain", testAnnotationBesideAThenChain)
	t.Run("annotation_among_constraint_steps", testAnnotationAmongConstraintSteps)
}

// testAnnotationTypedByNothingResolvable: an annotation whose metadata
// definition resolves to nothing is the name resolution tier's finding; the
// calculation still answers its result rather than refusing the body.
func testAnnotationTypedByNothingResolvable(t *testing.T) {
	src := `
		package test {
			calc twice {
				in n : Integer;
				@Nowhere { }
				n * 2
			}
		}
	`
	calcAnswersInSource(t, src, "twice", 3, 6)
}

// testAnnotationBindingAnUnboundName: the body of an annotation is no
// statement of the calculation, so a name it binds that nothing declares is
// never evaluated as a step.
func testAnnotationBindingAnUnboundName(t *testing.T) {
	src := `
		package test {
			metadata def Note { attribute tag : String; }
			calc twice {
				in n : Integer;
				@Note { tag = nothingDeclared; }
				n * 2
			}
		}
	`
	calcAnswersInSource(t, src, "twice", 3, 6)
}

// testAnnotationBesideAThenChain: an annotation between the steps of a
// calculation body with positional `then` successions is read past, so the
// chain still sequences from the step before it and the result is answered.
func testAnnotationBesideAThenChain(t *testing.T) {
	src := `
		package test {
			metadata def Note { attribute tag : String; }
			calc counting {
				in n : Integer;
				attribute total : Integer = 0;
				assign total := total + n;
				@Note { tag = "between"; }
				then assign total := total + 1;
				total
			}
		}
	`
	calcAnswersInSource(t, src, "counting", 3, 4)
}

// calcAnswersInSource invokes calcName with one Integer argument and asserts it
// answers want, on its own goroutine so a body that never terminates fails the
// case instead of stalling the suite.
func calcAnswersInSource(t *testing.T, src, calcName string, arg, want int64) {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, calcName, ast.DefCalc)
	if sym == nil {
		t.Fatalf("calc %s not found", calcName)
	}

	type outcome struct {
		result Value
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		value := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: arg}}
		result, err := ctx.InvokeCalc(sym, []Value{value}, rootScope)
		done <- outcome{result, err}
	}()

	select {
	case out := <-done:
		if out.err != nil {
			t.Fatalf("calc %s: the annotation must not stop the body, got: %v", calcName, out.err)
		}
		if got := FormatTraceValue(out.result); got != fmt.Sprintf("(%d)", want) {
			t.Fatalf("calc %s answered %s, want (%d)", calcName, got, want)
		}
	case <-watchdog(10 * time.Second):
		t.Fatalf("calc %s did not terminate", calcName)
	}
}

// testAnnotationAmongConstraintSteps: an annotation among a constraint body's
// steps is no step, so the assignments around it run in order and the
// condition reads the state they left.
func testAnnotationAmongConstraintSteps(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		metadata def Note { attribute tag : String; }
		constraint def Bounded {
			attribute y : Real = 0.0;
			@Note { tag = "before"; }
			assign y := 5.0;
			@Note { tag = "after"; }
			y > 3.0
		}
	}`
	ctx, idx := contextForSource(t, src)
	bounded := lookupOne(t, idx, "test::Bounded")
	satisfied, err := ctx.EvaluateConstraint(bounded, bounded.OwnerScope)
	if err != nil {
		if strings.Contains(err.Error(), "not executable") {
			t.Fatalf("the annotation was lowered as a step: %v", err)
		}
		t.Fatalf("err = %v", err)
	}
	if !satisfied {
		t.Error("the condition must read the assignment the annotation sits beside")
	}
}
