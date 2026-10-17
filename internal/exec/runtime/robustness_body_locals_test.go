package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessBodyLocals covers declarations local to a body: usages of non-calcs,
// non-executable members and locals read outside their declaration.
func TestRuntimeRobustnessBodyLocals(t *testing.T) {
	t.Run("body_local_usage_of_a_non_calc", testBodyLocalUsageOfANonCalc)
	t.Run("body_local_declaration_not_executable", testBodyLocalDeclarationNotExecutable)
	t.Run("f99_body_member_without_value", testF99BodyMemberWithoutValue)
	t.Run("f99_unsupported_body_member", testF99UnsupportedBodyMember)
	t.Run("f99_cyclic_body_declaration", testF99CyclicBodyDeclaration)
	t.Run("loop_body_declaration_does_not_leak", testLoopBodyDeclarationDoesNotLeak)
	t.Run("loop_body_of_unexecutable_statement", testLoopBodyOfUnexecutableStatement)
	t.Run("block_flow_of_unexecutable_member", testBlockFlowOfUnexecutableMember)
	t.Run("body_local_outside_its_declaration", testBodyLocalOutsideItsDeclaration)
}

// testBodyLocalOutsideItsDeclaration: a body-local whose run-time value falls
// outside its declared type, multiplicity or uniqueness is refused with a typed
// error where it is declared - in the evaluator and, where eligible, in the
// compiled tier alike - rather than computed with, hung on or panicked over.
func testBodyLocalOutsideItsDeclaration(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want error
	}{
		{"enumeration extent", "attribute l : Level = n; return : Integer = l + 0;", ErrTypeMismatch},
		{"scalar subtype", "attribute p : Positive = n - 3; return : Integer = p;", ErrTypeMismatch},
		{"stated multiplicity", "attribute xs : Integer[2] = (n, n + 1, n + 2); return : Integer = SequenceFunctions::size(xs);", ErrMultiplicityViolation},
		{"uniqueness", "attribute xs : Integer[*] = (n, n + 1, n); return : Integer = SequenceFunctions::size(xs);", ErrUniquenessViolation},
		{"nested block", "attribute r : Integer = 0; if n > 0 { attribute l : Level = n; assign r := l; } return : Integer = r;", ErrTypeMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := `package test {
				private import ScalarValues::*;
				enum def Level :> Integer { low = 1; high = 3; }
				calc def Body { in n : Integer; ` + tc.body + ` }
			}`
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
			pkg := resolveSymbol(t, idx.DocumentRoot("<test>"), "test")
			sym := resolveSymbol(t, pkg.Scope, "Body")

			done := make(chan error, 1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						done <- fmt.Errorf("panic: %v", r)
					}
				}()
				_, err := ctx.InvokeCalc(sym, []Value{constInt(2)}, pkg.Scope)
				done <- err
			}()
			select {
			case err := <-done:
				if !errors.Is(err, tc.want) {
					t.Errorf("InvokeCalc err = %v, want %v", err, tc.want)
				}
			case <-watchdog(5 * time.Second):
				t.Fatal("declaring the body-local did not terminate")
			}
		})
	}
}

// testLoopBodyDeclarationDoesNotLeak: a loop body and an `if` branch body are
// namespaces of their own, so a name one of them declares is not a member of the
// action and does not appear among its results.
func testLoopBodyDeclarationDoesNotLeak(t *testing.T) {
	src := `
		package test {
			action counter {
				attribute total : Integer = 0;
				first start;
				action accumulate {
					while total < 3 {
						attribute bump : Integer = 1;
						assign total := total + bump;
						if total == 2 {
							attribute marker : Integer = 9;
							assign total := total + marker;
						}
					}
				}
				done;
				succession first start then accumulate;
				succession first accumulate then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "counter", ast.DefAction)
	if sym == nil {
		t.Fatal("action counter not found")
	}

	outputs, err := ctx.ExecuteAction(sym)
	if err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	// 1, then 2 which the conditional lifts to 11, which ends the loop.
	total, ok := outputs["total"]
	if !ok {
		t.Fatal("total missing from the action's results")
	}
	if total.Const.Int != 11 {
		t.Errorf("total = %v, want 11", FormatTraceValue(total))
	}
	for _, local := range []string{"bump", "marker"} {
		if _, ok := outputs[local]; ok {
			t.Errorf("body-local %s leaked into the action's results: %v", local, outputs)
		}
	}
}

// testLoopBodyOfUnexecutableStatement: a body member the lowering layer cannot
// turn into a statement is reported when it is reached, rather than skipped —
// silently dropping it would give a wrong answer with no diagnostic.
func testLoopBodyOfUnexecutableStatement(t *testing.T) {
	src := `
		package test {
			action counter {
				attribute total : Integer = 0;
				first start;
				action accumulate {
					while total < 3 {
						part inner;
						assign total := total + 1;
					}
				}
				done;
				succession first start then accumulate;
				succession first accumulate then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "counter", ast.DefAction)
	if sym == nil {
		t.Fatal("action counter not found")
	}

	_, err := ctx.ExecuteAction(sym)
	if err == nil {
		t.Fatal("expected an unexecutable loop body member to be reported")
	}
	if !strings.Contains(err.Error(), "not executable") {
		t.Errorf("error does not name the unexecutable member: %v", err)
	}
}

// testBlockFlowOfUnexecutableMember: a member outside the semantics a block's own
// flow gives its members is still reported when reached, even in a block that
// does state a flow because a nested action is declared beside it.
func testBlockFlowOfUnexecutableMember(t *testing.T) {
	src := `
		package test {
			action counter {
				attribute total : Integer = 0;
				first start;
				action accumulate {
					while total < 3 {
						action bump {
							assign total := total + 1;
						}
						part inner;
					}
				}
				done;
				succession first start then accumulate;
				succession first accumulate then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "counter", ast.DefAction)
	if sym == nil {
		t.Fatal("action counter not found")
	}

	_, err := ctx.ExecuteAction(sym)
	if err == nil {
		t.Fatal("expected the unexecutable member of the block's flow to be reported")
	}
	if !strings.Contains(err.Error(), "not executable") {
		t.Errorf("error does not name the unexecutable member: %v", err)
	}
}

// testBodyLocalUsageOfANonCalc: a body-local usage typed by something that is no
// calc is reported when the declaration is reached, not skipped.
func testBodyLocalUsageOfANonCalc(t *testing.T) {
	src := `
		package test {
			part def Thing;
			calc def Holder {
				in n : Integer;
				attribute i : Integer = 0;
				while i < n {
					calc r : Thing;
					assign i := i + 1;
				}
				i
			}
		}
	`
	err := calcErrorWithLibraries(t, src, "Holder", []Value{constInt(1)}, 10000)
	if !errors.Is(err, ErrNotACalc) {
		t.Errorf("expected ErrNotACalc, got: %v", err)
	}
}

// testBodyLocalDeclarationNotExecutable: a declaration in a body the runtime has
// no execution for names itself rather than passing silently.
func testBodyLocalDeclarationNotExecutable(t *testing.T) {
	src := `
		package test {
			calc def Holder {
				in n : Integer;
				if n > 0 {
					part broken;
				}
				n
			}
		}
	`
	err := calcErrorWithLibraries(t, src, "Holder", []Value{constInt(1)}, 10000)
	if err == nil || !strings.Contains(err.Error(), "broken") {
		t.Errorf("error should name the declaration it cannot execute, got: %v", err)
	}
}

func testF99BodyMemberWithoutValue(t *testing.T) {
	_, err := evalCollectionExprBounded(t,
		"xs->collect { in i; private missing : Integer; missing }", 10000)
	if !errors.Is(err, ErrNoValue) {
		t.Fatalf("error = %v, want ErrNoValue", err)
	}
}

func testF99UnsupportedBodyMember(t *testing.T) {
	_, err := evalCollectionExprBounded(t,
		"xs->select { in i; private import ScalarValues::*; i > 0 }", 10000)
	if !errors.Is(err, errUnsupportedBodyDeclaration) {
		t.Fatalf("error = %v, want errUnsupportedBodyDeclaration", err)
	}
	if !strings.Contains(err.Error(), "Import") {
		t.Errorf("unsupported-member error = %q, want it to name the form", err)
	}
}

func testF99CyclicBodyDeclaration(t *testing.T) {
	_, err := evalCollectionExprBounded(t,
		"xs->collect { in i; private attribute cycleValue : Integer = cycleValue; cycleValue }", 10000)
	if !errors.Is(err, ErrCyclicFeatureValue) {
		t.Fatalf("error = %v, want ErrCyclicFeatureValue", err)
	}
}
