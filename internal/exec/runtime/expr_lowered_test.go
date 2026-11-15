package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast/astcodec"
)

const loweredExprSrc = `
	package P {
		part def A { attribute x = 1; attribute y = x + 1; }
		part def B { attribute x = 10; }
		part def L {
			attribute small = 0.5;
			attribute wide = 1.0e-40;
		}
	}
`

func loweredValue(t *testing.T, idx *symbols.Index, fqn string) ast.Node {
	t.Helper()
	usage, ok := lookupOne(t, idx, fqn).Decl.(*ast.Usage)
	if !ok || usage.Value == nil {
		t.Fatalf("%s binds no value", fqn)
	}
	return usage.Value
}

// TestLoweredExprIsPerScope pins that one tree evaluated in two scopes reads
// what its names denote in each, from one record per scope lowered once.
func TestLoweredExprIsPerScope(t *testing.T) {
	ctx, idx := contextForSource(t, loweredExprSrc)
	node := loweredValue(t, idx, "P::A::y")
	for _, tc := range []struct {
		owner string
		want  int64
	}{{"P::A", 2}, {"P::B", 11}, {"P::A", 2}} {
		scope := lookupOne(t, idx, tc.owner).Scope
		val, err := ctx.EvalWithScope(node, scope)
		if err != nil || val.Kind != ValConst || val.Const.Int != tc.want {
			t.Fatalf("x + 1 in %s = %v, %v; want %d", tc.owner, val, err, tc.want)
		}
		x := ctx.model.loweredExpr(scope, node)
		if again := ctx.model.loweredExpr(scope, node); again != x || x.expr().Kind != program.ExprOp {
			t.Fatalf("x + 1 in %s lowered to %v then %v", tc.owner, x, again)
		}
		read, _ := scope.LookupLocal("x")
		if ref := x.arg(0).expr(); ref.Kind != program.ExprRef || ref.Ref.Sym != read {
			t.Fatalf("x in %s resolved to %v, want %v", tc.owner, ref.Ref.Sym, read)
		}
		// An operand read by its own tree is a record of its own, lowered once.
		operand := node.(*ast.OperatorExpr).Operands[0]
		found := ctx.model.loweredExpr(scope, operand)
		if again := ctx.model.loweredExpr(scope, operand); again != found || found.expr().Ref.Sym != read {
			t.Fatalf("operand x in %s lowered to %v then %v, resolved to %v", tc.owner, found, again, found.expr().Ref.Sym)
		}
	}
}

// TestLoweredLiteralAnswersToTheRunBudget pins that a decimal literal is refused
// by the Integer size budget of the run reading it, not the one lowering read it under.
func TestLoweredLiteralAnswersToTheRunBudget(t *testing.T) {
	ctx, idx := contextForSource(t, loweredExprSrc)
	scope := lookupOne(t, idx, "P::L").Scope
	small, wide := loweredValue(t, idx, "P::L::small"), loweredValue(t, idx, "P::L::wide")

	half, _ := semantics.ParseRational("0.5", semantics.DefaultMaxIntegerBits)
	for _, node := range []ast.Node{small, wide} {
		if val, err := ctx.EvalWithScope(node, scope); err != nil || val.Kind != ValConst || val.Const.Kind != semantics.ValRational {
			t.Fatalf("literal under the default budget = %v, %v", val, err)
		}
	}
	budgets := ctx.Budgets()
	budgets.MaxIntegerBits = semantics.MinMaxIntegerBits
	if err := ctx.SetBudgets(budgets); err != nil {
		t.Fatal(err)
	}
	if val, err := ctx.EvalWithScope(small, scope); err != nil || !val.Const.Equal(half) {
		t.Fatalf("0.5 under the least budget = %v, %v", val, err)
	}
	_, err := ctx.EvalWithScope(wide, scope)
	if !errors.Is(err, semantics.ErrRationalSizeLimit) || !strings.HasPrefix(err.Error(), "literal 1.0e-40: ") {
		t.Fatalf("1.0e-40 under the least budget: %v", err)
	}
	budgets.MaxIntegerBits = semantics.DefaultMaxIntegerBits
	if err := ctx.SetBudgets(budgets); err != nil {
		t.Fatal(err)
	}
	if _, err := ctx.EvalWithScope(wide, scope); err != nil {
		t.Fatalf("1.0e-40 under the default budget again: %v", err)
	}
}

// TestLoweredExprForgetsARequestExpression pins that an expression the model does
// not own leaves no record once the Scratch its request evaluated it in ends.
func TestLoweredExprForgetsARequestExpression(t *testing.T) {
	ctx, idx := contextForSource(t, loweredExprSrc)
	scope := lookupOne(t, idx, "P::A").Scope
	owned := loweredValue(t, idx, "P::A::y")
	if _, err := ctx.EvalWithScope(owned, scope); err != nil {
		t.Fatal(err)
	}
	kept := len(ctx.model.lowered)
	request := parseExpr(t, "x + 1 + y")
	var val Value
	var err error
	ctx.Resolver().Scratch(astcodec.Reachable(request), func() {
		val, err = ctx.EvalWithScope(request, scope)
		if len(ctx.model.lowered) <= kept {
			t.Errorf("the request's expression was not lowered through the model's table")
		}
	})
	if err != nil || val.Kind != ValConst || val.Const.Int != 4 {
		t.Fatalf("x + 1 + y in P::A = %v, %v; want 4", val, err)
	}
	if got := len(ctx.model.lowered); got != kept {
		t.Fatalf("%d expressions held after the request, %d before it", got, kept)
	}
	if _, held := ctx.model.lowered[loweredKey{scope: scope, expr: owned}]; !held {
		t.Fatalf("the model's own expression was forgotten with the request's")
	}
}
