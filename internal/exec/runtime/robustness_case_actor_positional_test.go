package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessCaseActorPositional covers the arguments of a case or
// requirement that declares an actor before its inputs: an actor binds by name
// only, so positional arguments map onto the other parameters in declaration
// order, and the count checked against them leaves the actor out.
func TestRuntimeRobustnessCaseActorPositional(t *testing.T) {
	t.Run("case_actor_surplus_positional_argument", testCaseActorSurplusPositionalArgument)
	t.Run("case_actor_exact_positional_arguments", testCaseActorExactPositionalArguments)
	t.Run("case_actor_positional_beside_named", testCaseActorPositionalBesideNamed)
	t.Run("case_actor_function_value_arity", testCaseActorFunctionValueArity)
	t.Run("case_actor_invocation_memo_key", testCaseActorInvocationMemoKey)
}

const actorBeforeInputSource = `
	package test {
		private import ScalarValues::*;
		part def Operator;
		requirement def WithinLimit {
			subject v : Real;
			actor op : Operator;
			in max : Real;
			in slack : Real default 0.0;
			require constraint { v < max + slack }
		}
	}
`

// testCaseActorSurplusPositionalArgument: the actor takes no positional slot, so
// a fourth argument to a subject and two inputs is surplus, a typed refusal.
func testCaseActorSurplusPositionalArgument(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, actorBeforeInputSource))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "WithinLimit", ast.DefRequirement)
	if sym == nil {
		t.Fatal("WithinLimit requirement not found")
	}

	holds, err := ctx.InvokePredicate(sym, []Value{realArg(3), realArg(10), realArg(1), realArg(7)}, rootScope)
	if err == nil {
		t.Fatalf("expected an arity error, the requirement accepted a surplus argument and answered %v", holds)
	}
	if !errors.Is(err, errCalcArity) {
		t.Errorf("expected errCalcArity, got: %v", err)
	}
}

// testCaseActorExactPositionalArguments: the subject and the input after the
// actor are the positional parameters, so two arguments bind them both and the
// defaulted input keeps its default.
func testCaseActorExactPositionalArguments(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, actorBeforeInputSource))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "WithinLimit", ast.DefRequirement)
	if sym == nil {
		t.Fatal("WithinLimit requirement not found")
	}

	holds, err := ctx.InvokePredicate(sym, []Value{realArg(3), realArg(10)}, rootScope)
	if err != nil {
		t.Fatalf("InvokePredicate(3, 10) failed: %v", err)
	}
	if !holds {
		t.Error("WithinLimit(3, 10) = false, want true: 3 < 10 + 0")
	}
	holds, err = ctx.InvokePredicate(sym, []Value{realArg(12), realArg(10), realArg(5)}, rootScope)
	if err != nil {
		t.Fatalf("InvokePredicate(12, 10, 5) failed: %v", err)
	}
	if !holds {
		t.Error("WithinLimit(12, 10, 5) = false, want true: 12 < 10 + 5")
	}
}

// testCaseActorPositionalBesideNamed: a positional argument binds the first
// positional parameter, a named one the input it names, and a positional
// argument beyond the parameters left is surplus whatever the names supply.
func testCaseActorPositionalBesideNamed(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, actorBeforeInputSource))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "WithinLimit", ast.DefRequirement)
	if sym == nil {
		t.Fatal("WithinLimit requirement not found")
	}

	ec := newEvalContext(ctx, rootScope)
	result, err := ec.invokePredicate(sym, calcArgs{
		positional: []Value{realArg(12)},
		named:      map[string]Value{"max": realArg(10), "slack": realArg(5)},
	})
	if err != nil {
		t.Fatalf("WithinLimit(12, max = 10, slack = 5) failed: %v", err)
	}
	if !result.isBool() || !result.Const.Bool {
		t.Errorf("WithinLimit(12, max = 10, slack = 5) = %s, want true", DescribeValue(result))
	}

	_, err = ec.invokePredicate(sym, calcArgs{
		positional: []Value{realArg(12), realArg(10), realArg(5), realArg(1)},
		named:      map[string]Value{"slack": realArg(5)},
	})
	if !errors.Is(err, errCalcArity) {
		t.Errorf("expected errCalcArity for a surplus positional argument, got: %v", err)
	}
}

// testCaseActorFunctionValueArity: read as a function value, the shape's arity
// is that of its positional parameters, so a body check with that many
// arguments passes and one counting the actor is refused with ErrBodyArity.
func testCaseActorFunctionValueArity(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, actorBeforeInputSource))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "WithinLimit", ast.DefRequirement)
	if sym == nil {
		t.Fatal("WithinLimit requirement not found")
	}
	shape := ctx.predicateShapeOf(sym)
	fn := &functionValue{shape: shape, scope: rootScope}
	if low, high := fn.arity(); low != 2 || high != 3 {
		t.Errorf("arity() = (%d, %d), want (2, 3): the subject and two inputs, not the actor", low, high)
	}
	value := Value{Kind: ValFunction, ref: fn}
	if _, err := checkBody("select", value, 2); err != nil {
		t.Errorf("checkBody with the two positional parameters refused: %v", err)
	}
	_, err := checkBody("select", value, 4)
	if !errors.Is(err, ErrBodyArity) {
		t.Errorf("expected ErrBodyArity for four arguments, got: %v", err)
	}
}

// testCaseActorInvocationMemoKey: two calls the binder tells apart must not share
// a statement-order memo: a positional argument binds the input after the actor,
// a named one of the same value binds the actor.
func testCaseActorInvocationMemoKey(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, actorBeforeInputSource))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "WithinLimit", ast.DefRequirement)
	if sym == nil {
		t.Fatal("WithinLimit requirement not found")
	}
	shape := ctx.predicateShapeOf(sym)
	positional := canonicalInvocationArguments(shape, calcArgs{
		positional: []Value{realArg(3), realArg(10)},
		named:      map[string]Value{"slack": realArg(1)},
	})
	named := canonicalInvocationArguments(shape, calcArgs{
		named: map[string]Value{"v": realArg(3), "op": realArg(10), "slack": realArg(1)},
	})
	if positional == named {
		t.Errorf("memo key %q does not tell the positional binding of max from the named binding of op", positional)
	}
	same := canonicalInvocationArguments(shape, calcArgs{
		named: map[string]Value{"v": realArg(3), "max": realArg(10), "slack": realArg(1)},
	})
	if positional != same {
		t.Errorf("memo keys differ for one binding:\n%q\n%q", positional, same)
	}
	surplus := canonicalInvocationArguments(shape, calcArgs{
		positional: []Value{realArg(3), realArg(10), realArg(1), realArg(7)},
	})
	if !strings.Contains(surplus, "extra-pos:") {
		t.Errorf("memo key %q does not record the surplus positional argument", surplus)
	}
}
