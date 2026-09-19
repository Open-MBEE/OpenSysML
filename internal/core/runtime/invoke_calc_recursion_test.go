package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// recursiveSumModel sums the integers up to its argument by recursion, so the
// depth one invocation reaches is the argument it is given.
const recursiveSumModel = `
package test {
	calc sumTo {
		in n: Integer;
		return : Integer = if n <= 0 ? 0 else n + sumTo(n - 1);
	}
}
`

// invokeSumTo invokes sumTo(n) under the given calc depth budget.
func invokeSumTo(t *testing.T, n int64, maxCalcDepth int64) (Value, error) {
	t.Helper()

	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, recursiveSumModel))
	ctx.maxSteps = DefaultMaxSteps
	ctx.maxCalcDepth = maxCalcDepth
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "sumTo", ast.DefCalc)
	if sym == nil {
		t.Fatal("calc sumTo not found")
	}
	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
	return ctx.InvokeCalc(sym, []Value{arg}, rootScope)
}

// TestRecursiveCalcSpendsTheDepthBudget requires the calc depth budget to be
// what bounds a recursion: a recursion terminating inside it evaluates, and the
// same recursion beyond it reports the budget and how to raise it.
func TestRecursiveCalcSpendsTheDepthBudget(t *testing.T) {
	result, err := invokeSumTo(t, 200, 1000)
	if err != nil {
		t.Fatalf("sumTo(200) under a depth budget of 1000: %v", err)
	}
	if got := result.Const.Int; got != 20100 {
		t.Errorf("sumTo(200) = %d, want 20100", got)
	}

	_, err = invokeSumTo(t, 200, 50)
	if !errors.Is(err, ErrCalcRecursionLimit) {
		t.Fatalf("sumTo(200) under a depth budget of 50: got %v, want ErrCalcRecursionLimit", err)
	}
	if !strings.Contains(err.Error(), MaxCalcDepthEnvVar) {
		t.Errorf("err = %q; want the budget's variable %s named", err, MaxCalcDepthEnvVar)
	}
}

// TestRecursiveCalcDepthIsNotAFixedBound requires a recursion far deeper than
// the fixed nesting bound that used to refuse one to evaluate under the default
// budget, since only an unbounded recursion is meant to be stopped.
func TestRecursiveCalcDepthIsNotAFixedBound(t *testing.T) {
	result, err := invokeSumTo(t, 5000, DefaultMaxCalcDepth)
	if err != nil {
		t.Fatalf("sumTo(5000) under the default depth budget: %v", err)
	}
	if got := result.Const.Int; got != 12502500 {
		t.Errorf("sumTo(5000) = %d, want 12502500", got)
	}
}

// TestMutuallyRecursiveCalcsEvaluate requires a cycle between two calculations
// to evaluate as one recursion: the depth they share is spent from the same
// budget, and each invocation binds its own parameter.
func TestMutuallyRecursiveCalcsEvaluate(t *testing.T) {
	src := `
		package test {
			calc isEven {
				in n: Integer;
				return : Boolean = if n == 0 ? true else isOdd(n - 1);
			}
			calc isOdd {
				in n: Integer;
				return : Boolean = if n == 0 ? false else isEven(n - 1);
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	ctx.maxSteps = DefaultMaxSteps
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "isEven", ast.DefCalc)
	if sym == nil {
		t.Fatal("calc isEven not found")
	}

	for _, tc := range []struct {
		n    int64
		want bool
	}{{n: 0, want: true}, {n: 7, want: false}, {n: 400, want: true}} {
		arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: tc.n}}
		result, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
		if err != nil {
			t.Fatalf("isEven(%d): %v", tc.n, err)
		}
		if got := result.Const.Bool; got != tc.want {
			t.Errorf("isEven(%d) = %t, want %t", tc.n, got, tc.want)
		}
	}
}
