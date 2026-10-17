package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// TestRuntimeRobustnessRunBudget covers the step and element budgets: ranges, collections,
// non-terminating loops and evaluation on an instance.
func TestRuntimeRobustnessRunBudget(t *testing.T) {
	t.Run("range_bound_is_not_an_integer", testRangeBoundIsNotAnInteger)
	t.Run("range_spends_the_step_budget", testRangeSpendsTheStepBudget)
	t.Run("collection_spends_the_element_budget", testCollectionSpendsTheElementBudget)
	t.Run("step_budget_exceeded", testStepBudgetExceeded)
	t.Run("eval_on_an_instance_spends_the_step_budget", testEvalOnAnInstanceSpendsTheStepBudget)
	t.Run("non_terminating_loop_exhausts_step_budget", testNonTerminatingLoopExhaustsStepBudget)
	t.Run("non_terminating_loop_performing_an_action", testNonTerminatingLoopPerformingAnAction)
	t.Run("for_over_a_value_no_expression_makes_iterable", testForOverAValueNoExpressionMakesIterable)
	t.Run("for_over_a_scalar", testForOverAScalar)
}

// testStepBudgetExceeded: evaluation exceeds maxSteps. Each Eval call spends one
// step, so an expression with more subexpressions than the budget must report
// ErrStepLimitExceeded rather than run to the end. The operands are a parameter
// rather than literals because a constant expression is folded in one step.
func testStepBudgetExceeded(t *testing.T) {
	src := `
		package test {
			calc deep {
				in x : Integer;
				x + x + x + x + x + x + x + x
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	ctx.maxSteps = 3
	ctx.run.steps = 0

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "deep", ast.DefCalc)
	if sym == nil {
		t.Fatal("calc deep not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	_, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if err == nil {
		t.Fatal("expected the step budget to be exceeded, the calc completed")
	}
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// testEvalOnAnInstanceSpendsTheStepBudget: an expression evaluated against an
// instance is one run, so reading a feature value inside it does not start a run of its
// own and reset the counter; an expression longer than the budget is refused.
func testEvalOnAnInstanceSpendsTheStepBudget(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
		package test {
			part def Car { attribute m = 5.0; }
		}
	`))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Car", ast.DefPart)
	if sym == nil {
		t.Fatal("part def Car not found")
	}
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		t.Fatalf("instantiating Car: %v", err)
	}

	// Nested to the right, so the feature value read - which brackets a run of its own when
	// the evaluation is not already one - is reached on the second step.
	expr := "m"
	for i := 0; i < 60; i++ {
		expr = "m + (" + expr + ")"
	}
	node := parser.New(source.New("<e>", []byte(expr))).ParseExpression()
	if node == nil {
		t.Fatal("the expression did not parse")
	}

	ctx.maxSteps = 6
	got, err := ctx.EvalWithScopeOn(node, sym.Scope, inst)
	if err == nil {
		t.Fatalf("expected the step budget to bound the evaluation, got %v", got)
	}
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}

	// The budget bounds one run, not the session: a short expression is answered
	// however many ran before it.
	ctx.maxSteps = 20
	short := parser.New(source.New("<e>", []byte("m + m"))).ParseExpression()
	for i := 0; i < 5; i++ {
		if _, err := ctx.EvalWithScopeOn(short, sym.Scope, inst); err != nil {
			t.Fatalf("evaluation %d of m + m under a fresh run: %v", i+1, err)
		}
	}
}

// testNonTerminatingLoopExhaustsStepBudget: a loop whose condition never fails
// spends a step per iteration, so it ends the execution with
// ErrStepLimitExceeded instead of hanging whoever drove it (a REPL or the LSP).
func testNonTerminatingLoopExhaustsStepBudget(t *testing.T) {
	src := `
		package test {
			action spinner {
				attribute total : Integer = 0;
				first start;
				action spin {
					while total >= 0 {
						assign total := total + 1;
					}
				}
				done;
				succession first start then spin;
				succession first spin then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	ctx.maxSteps = 20
	ctx.run.steps = 0

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "spinner", ast.DefAction)
	if sym == nil {
		t.Fatal("action spinner not found")
	}

	_, err := ctx.ExecuteAction(sym)
	if err == nil {
		t.Fatal("expected the step budget to be exceeded, the action completed")
	}
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// testNonTerminatingLoopPerformingAnAction: a loop whose body performs an action
// as a node of the block's own flow still spends a step per iteration, so it
// ends with ErrStepLimitExceeded rather than performing forever.
func testNonTerminatingLoopPerformingAnAction(t *testing.T) {
	src := `
		package test {
			action spinner {
				attribute total : Integer = 0;
				first start;
				action spin {
					while total >= 0 {
						perform bump;
						assign total := total + 1;
					}
				}
				done;
				succession first start then spin;
				succession first spin then done;
			}

			action bump {
				out spun : Integer;
				first begin;
				action run {
					assign spun := 1;
				}
				done;
				succession first begin then run;
				succession first run then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	ctx.maxSteps = 40
	ctx.run.steps = 0

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "spinner", ast.DefAction)
	if sym == nil {
		t.Fatal("action spinner not found")
	}

	_, err := ctx.ExecuteAction(sym)
	if err == nil {
		t.Fatal("expected the step budget to be exceeded, the action completed")
	}
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// testForOverAValueNoExpressionMakesIterable: a value that states a computation
// rather than a collection is no collection in any order, so a `for` over it
// fails with a typed error naming it.
func testForOverAValueNoExpressionMakesIterable(t *testing.T) {
	for _, value := range []Value{{Kind: valExpr}, {Kind: ValInvalid}} {
		elements, err := forElements(value)
		if err == nil {
			t.Errorf("forElements(%s) = %v, want a typed error", DescribeValue(value), elements)
			continue
		}
		if !errors.Is(err, ErrTypeMismatch) {
			t.Errorf("forElements(%s) failed with %v, want ErrTypeMismatch", DescribeValue(value), err)
		}
		if !strings.Contains(err.Error(), DescribeValue(value)) {
			t.Errorf("error does not name the value: %v", err)
		}
	}
}

// testForOverAScalar: a `for` whose input is a scalar fails with a typed error
// rather than iterating once over the coercion elementsOf would make of it.
func testForOverAScalar(t *testing.T) {
	src := `
		package test {
			action counter {
				attribute single : Integer = 7;
				attribute visited : Integer = 0;
				first start;
				action iterate {
					for s in single {
						assign visited := visited + 1;
					}
				}
				done;
				succession first start then iterate;
				succession first iterate then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))

	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "counter", ast.DefAction)
	if sym == nil {
		t.Fatal("action counter not found")
	}

	result, err := ctx.ExecuteAction(sym)
	if err == nil {
		t.Fatalf("the action completed with %v, want a typed error", result)
	}
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("execution failed with %v, want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "an Integer is not one") {
		t.Errorf("error does not name the value it was given: %v", err)
	}
}

// testRangeBoundIsNotAnInteger: `..` declares Integer bounds, so a Real bound is
// the type mismatch it is rather than a truncated range.
func testRangeBoundIsNotAnInteger(t *testing.T) {
	src := `
		package test {
			calc def Span {
				in n : Integer;
				attribute r = 1.5..n;
				n
			}
		}
	`
	err := calcErrorWithLibraries(t, src, "Span", []Value{constInt(3)}, 10000)
	if !errors.Is(err, ErrTypeMismatch) {
		t.Errorf("expected ErrTypeMismatch, got: %v", err)
	}
}

// testRangeSpendsTheStepBudget: each element a range generates costs a step, so
// a range too large to hold fails the run rather than exhausting memory.
func testRangeSpendsTheStepBudget(t *testing.T) {
	src := `
		package test {
			calc def Span {
				in n : Integer;
				attribute r = 1..1000000;
				n
			}
		}
	`
	err := calcErrorWithLibraries(t, src, "Span", []Value{constInt(3)}, 100)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// testCollectionSpendsTheElementBudget: a materialized element is memory the
// collection keeps, so it has its own ceiling and its own error rather than
// reading as the step budget's.
func testCollectionSpendsTheElementBudget(t *testing.T) {
	src := `
		package test {
			calc def Span {
				in n : Integer;
				attribute r = (1..1000)->collect{in i; i * i};
				n
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	ctx.maxElements = 100
	sym, scope := calcByName(t, idx.DocumentRoot("<test>"), "test", "Span")
	_, err := ctx.InvokeCalc(sym, []Value{constInt(3)}, scope)
	if err == nil {
		t.Fatal("want the element budget's error, got a value")
	}
	if !errors.Is(err, ErrElementLimitExceeded) {
		t.Errorf("expected ErrElementLimitExceeded, got: %v", err)
	}
	if errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("error %q also reads as the step budget's", err)
	}
	if !strings.Contains(err.Error(), MaxElementsEnvVar) {
		t.Errorf("error %q does not name %s", err, MaxElementsEnvVar)
	}
}
