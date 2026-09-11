package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// parseExpr parses src as one expression, so a test can evaluate the same node twice.
func parseExpr(t *testing.T, src string) ast.Node {
	t.Helper()
	expr := parser.New(source.New("<e>", []byte(src))).ParseExpression()
	if expr == nil {
		t.Fatalf("failed to parse %q", src)
	}
	return expr
}

// TestRepeatedLiteralEvaluationAnswersAlike: a literal evaluated again in one
// context, memoized or not, answers the same value, and one outside its range
// reports the same error each time rather than a cached success.
func TestRepeatedLiteralEvaluationAnswersAlike(t *testing.T) {
	model, resolver, _ := parseAndBuildModel(t, sumModel)
	ctx := NewContext(NewModel(model, resolver), 1000)

	for _, tc := range []struct {
		src  string
		want semantics.Value
	}{
		{"42", semantics.Value{Kind: semantics.ValInt, Int: 42}},
		{"2.5", semantics.Value{Kind: semantics.ValReal, Real: 2.5}},
	} {
		expr := parseExpr(t, tc.src)
		for i := 0; i < 3; i++ {
			got, err := ctx.Eval(expr)
			if err != nil || got.Kind != ValConst || got.Const != tc.want {
				t.Fatalf("eval %d of %s = %+v, %v; want %+v", i, tc.src, got, err, tc.want)
			}
		}
	}

	for _, src := range []string{"9223372036854775808", "1e400"} {
		expr := parseExpr(t, src)
		var first string
		for i := 0; i < 3; i++ {
			got, err := ctx.Eval(expr)
			if !errors.Is(err, semantics.ErrArithmeticOverflow) {
				t.Fatalf("eval %d of %s = %+v, %v; want ErrArithmeticOverflow", i, src, got, err)
			}
			if i == 0 {
				first = err.Error()
			} else if err.Error() != first {
				t.Fatalf("eval %d of %s reported %q; first reported %q", i, src, err.Error(), first)
			}
		}
	}
}

// TestNestedInvocationArgumentsStayDistinct: arguments that are themselves
// invocations, or that recurse while others are already evaluated, bind to their
// own parameters, and an argument failing part-way leaves nothing behind for
// the next invocation to read.
func TestNestedInvocationArgumentsStayDistinct(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `
		calc def sub { in a : Integer; in b : Integer; return : Integer = a - b; }
		calc def fib { in k : Integer; return : Integer = if k <= 1 ? k else fib(k - 1) + fib(k - 2); }
		calc def three { in a : Integer; in b : Integer; in c : Integer; return : Integer = a * 100 + b * 10 + c; }
	`)
	ctx := NewContext(NewModel(model, resolver), 100000)
	ec := NewEvalContext(ctx, root)

	for _, tc := range []struct {
		src  string
		want int64
	}{
		{"sub(fib(10), fib(5))", 50},
		{"sub(sub(9, fib(3)), sub(fib(4), 1))", 5},
		{"three(fib(6), sub(fib(7), fib(6)), fib(3))", 852},
		{"fib(sub(fib(6), 2))", 8},
	} {
		expr := parseExpr(t, tc.src)
		for i := 0; i < 2; i++ {
			got, err := ec.Eval(expr)
			if err != nil || got.Kind != ValConst || got.Const.Int != tc.want {
				t.Fatalf("eval %d of %s = %+v, %v; want %d", i, tc.src, got, err, tc.want)
			}
			if len(ctx.argStack) != 0 {
				t.Fatalf("eval %d of %s left %d arguments on the stack", i, tc.src, len(ctx.argStack))
			}
		}
	}

	failing := parseExpr(t, "sub(fib(5), 9223372036854775808)")
	if _, err := ec.Eval(failing); !errors.Is(err, semantics.ErrArithmeticOverflow) {
		t.Fatalf("eval of a failing second argument: err = %v; want ErrArithmeticOverflow", err)
	}
	if len(ctx.argStack) != 0 {
		t.Fatalf("a failing argument left %d arguments on the stack", len(ctx.argStack))
	}
	got, err := ec.Eval(parseExpr(t, "sub(7, 2)"))
	if err != nil || got.Const.Int != 5 {
		t.Fatalf("sub(7, 2) after a failed invocation = %+v, %v; want 5", got, err)
	}
}

// TestRepeatedInvocationEvaluationAnswersAlike: an invocation expression
// evaluated again in one context reaches the same calc, and one naming no
// declaration is refused the same way each time, after its arguments.
func TestRepeatedInvocationEvaluationAnswersAlike(t *testing.T) {
	model, resolver, root := parseAndBuildModel(t, `
		calc def twice { in x : Integer; return : Integer = x + x; }
		calc def fib { in k : Integer; return : Integer = if k <= 1 ? k else fib(k - 1) + fib(k - 2); }
	`)
	ctx := NewContext(NewModel(model, resolver), 100000)
	ec := NewEvalContext(ctx, root)

	for _, tc := range []struct {
		src  string
		want int64
	}{
		{"twice(21)", 42},
		{"fib(10)", 55},
	} {
		expr := parseExpr(t, tc.src)
		for i := 0; i < 3; i++ {
			got, err := ec.Eval(expr)
			if err != nil || got.Kind != ValConst || got.Const.Int != tc.want {
				t.Fatalf("eval %d of %s = %+v, %v; want %d", i, tc.src, got, err, tc.want)
			}
		}
	}

	missing := parseExpr(t, "nowhere(1)")
	var first string
	for i := 0; i < 3; i++ {
		got, err := ec.Eval(missing)
		if !errors.Is(err, ErrUnresolvedReference) {
			t.Fatalf("eval %d of nowhere(1) = %+v, %v; want ErrUnresolvedReference", i, got, err)
		}
		if i == 0 {
			first = err.Error()
		} else if err.Error() != first {
			t.Fatalf("eval %d reported %q; first reported %q", i, err.Error(), first)
		}
	}

	// An argument that fails is reported before the target is judged, so a
	// resolved and an unresolved call alike answer with the argument's error.
	for _, src := range []string{"twice(9223372036854775808)", "nowhere(9223372036854775808)"} {
		expr := parseExpr(t, src)
		for i := 0; i < 2; i++ {
			if _, err := ec.Eval(expr); !errors.Is(err, semantics.ErrArithmeticOverflow) {
				t.Fatalf("eval %d of %s: err = %v; want the argument's ErrArithmeticOverflow", i, src, err)
			}
		}
	}
}
