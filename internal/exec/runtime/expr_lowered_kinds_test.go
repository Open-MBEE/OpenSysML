package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/program"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast/astcodec"
)

const loweredKindsSrc = `
package K {
	private import ScalarValues::*;
	calc def sub { in a : Integer; in b : Integer; return : Integer = a - b; }
	part def Pair { attribute left : Integer; attribute right : Integer; }
	metadata def Mark;
	part marked { @Mark; }
}
`

// TestLoweredKindsEvaluateFromTheirRecords pins each kind the evaluator reads from its
// record: the kind lowered and its value, an annotated part's metadata being two objects.
func TestLoweredKindsEvaluateFromTheirRecords(t *testing.T) {
	for _, tc := range []struct {
		expr string
		kind program.ExprKind
		want func(Value) bool
	}{
		{"K::sub(7, 2)", program.ExprInvoke, isInt(5)},
		{"K::sub(b = 2, a = 7)", program.ExprInvoke, isInt(5)},
		{"(1, 2, 3)->SequenceFunctions::size()", program.ExprInvoke, isInt(3)},
		{"(1, 2, 3).{in x : ScalarValues::Integer; x + 1}", program.ExprCollect, isInts(2, 3, 4)},
		{"(1, 2, 3).?{in x : ScalarValues::Integer; x > 1}", program.ExprSelect, isInts(2, 3)},
		{"new K::Pair(1, 2)", program.ExprConstruct, func(v Value) bool { return soleElement(v).Kind == ValInstance }},
		{"new K::Pair(right = 2, left = 1)", program.ExprConstruct, func(v Value) bool { return soleElement(v).Kind == ValInstance }},
		{"K::marked.metadata", program.ExprMetadata, func(v Value) bool { return len(elementsOf(v)) == 2 }},
	} {
		ctx, scope, decl := declaredExpr(t, loweredKindsSrc, tc.expr)
		ec := newEvalContext(ctx, scope)
		if kind := ec.lowered(decl).expr().Kind; kind != tc.kind {
			t.Fatalf("%s lowers to kind %d, want %d", tc.expr, kind, tc.kind)
		}
		for i := 0; i < 2; i++ {
			got, err := ec.Eval(decl)
			if err != nil || !tc.want(got) {
				t.Fatalf("eval %d of %s = %+v, %v", i, tc.expr, got, err)
			}
		}
	}
}

// TestLoweredBodyIsDeferred pins that a body expression's record evaluates to the
// body itself, applied by what it is passed to rather than where it is written.
func TestLoweredBodyIsDeferred(t *testing.T) {
	ctx, scope, decl := declaredExpr(t, loweredKindsSrc, "(4, 0).{in x : ScalarValues::Integer; 8 / x}")
	ec := newEvalContext(ctx, scope)
	body := ec.lowered(decl).arg(1)
	if body.expr().Kind != program.ExprBody {
		t.Fatalf("the collected body lowers to kind %d", body.expr().Kind)
	}
	if got, err := ec.evalExpr(body); err != nil || got.Kind != valExpr {
		t.Fatalf("the body alone = %+v, %v; want it deferred", got, err)
	}
	if _, err := ec.Eval(decl); !errors.Is(err, ErrDivisionByZero) {
		t.Fatalf("the body applied to 0: %v; want ErrDivisionByZero", err)
	}
}

// TestLoweredInvocationArgumentsFailBeforeTheTarget pins the order a call reports
// in: a failing argument first, by position or by name, then a name denoting nothing.
func TestLoweredInvocationArgumentsFailBeforeTheTarget(t *testing.T) {
	for expr, want := range map[string]error{
		"K::sub(1 / 0, 2)":                   ErrDivisionByZero,
		"K::sub(b = 1 / 0)":                  ErrDivisionByZero,
		"nowhere(1 / 0)":                     ErrDivisionByZero,
		"nowhere(1)":                         ErrUnresolvedReference,
		"(1 / 0)->SequenceFunctions::size()": ErrDivisionByZero,
		"new K::Pair(1 / 0, 2)":              ErrDivisionByZero,
		"new K::Pair(left = 1 / 0)":          ErrDivisionByZero,
	} {
		ctx, scope, decl := declaredExpr(t, loweredKindsSrc, expr)
		if _, err := newEvalContext(ctx, scope).Eval(decl); !errors.Is(err, want) {
			t.Errorf("%s: %v; want %v", expr, err, want)
		}
	}
}

// TestLoweredInvocationTargetsLeaveWithTheRequest pins that what a request's
// invocations were resolved to is dropped with their records, and the model's own kept.
func TestLoweredInvocationTargetsLeaveWithTheRequest(t *testing.T) {
	ctx, scope, owned := declaredExpr(t, loweredKindsSrc, "K::sub(7, 2)")
	if got, err := ctx.EvalWithScope(owned, scope); err != nil || !isInt(5)(got) {
		t.Fatalf("K::sub(7, 2) = %+v, %v", got, err)
	}
	kept := len(ctx.model.invocationTargets)
	if kept == 0 {
		t.Fatal("the model's invocation was not memoized")
	}
	request := parseExpr(t, "K::sub(K::sub(9, 1), 2)")
	for i := 0; i < 3; i++ {
		ctx.Resolver().Scratch(astcodec.Reachable(request), func() {
			got, err := ctx.EvalWithScope(request, scope)
			if err != nil || !isInt(6)(got) {
				t.Errorf("request %d = %+v, %v; want 6", i, got, err)
			}
			if len(ctx.model.invocationTargets) != kept+2 {
				t.Errorf("request %d memoized %d targets beside the model's %d", i, len(ctx.model.invocationTargets)-kept, kept)
			}
		})
		if got := len(ctx.model.invocationTargets); got != kept {
			t.Fatalf("%d invocation targets held after request %d, %d before it", got, i, kept)
		}
	}
}

func isInt(want int64) func(Value) bool {
	return func(v Value) bool {
		v = soleElement(v)
		return v.Kind == ValConst && v.Const.Int == want
	}
}

func isInts(want ...int64) func(Value) bool {
	return func(v Value) bool {
		elems := elementsOf(v)
		if len(elems) != len(want) {
			return false
		}
		for i, elem := range elems {
			if !isInt(want[i])(elem) {
				return false
			}
		}
		return true
	}
}
