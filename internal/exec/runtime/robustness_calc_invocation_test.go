package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessCalcInvocation covers calculation invocation: unbound and repeated
// parameters, arity, recursion, bodies without a result and rejected statements.
func TestRuntimeRobustnessCalcInvocation(t *testing.T) {
	t.Run("calc_unbound_parameter", testCalcUnboundParameter)
	t.Run("calc_this_default_required_parameter_unbound", testCalcThisDefaultRequiredParameterUnbound)
	t.Run("calc_calls_an_unimported_extension_function", testCalcCallsAnUnimportedExtensionFunction)
	t.Run("calc_calls_an_unimported_library_function", testCalcCallsAnUnimportedLibraryFunction)
	t.Run("calc_unbound_keyword_named_parameter", testCalcUnboundKeywordNamedParameter)
	t.Run("calc_too_many_arguments", testCalcTooManyArguments)
	t.Run("calc_unknown_named_argument", testCalcUnknownNamedArgument)
	t.Run("calc_parameter_named_twice", testCalcParameterNamedTwice)
	t.Run("calc_without_result", testCalcWithoutResult)
	t.Run("calc_states_second_result", testCalcStatesSecondResult)
	t.Run("calc_body_states_two_results", testCalcBodyStatesTwoResults)
	t.Run("calc_symbol_is_not_a_calc", testCalcSymbolIsNotACalc)
	t.Run("calc_direct_recursion", testCalcDirectRecursion)
	t.Run("calc_mutual_recursion", testCalcMutualRecursion)
	t.Run("calc_default_recursion", testCalcDefaultRecursion)
	t.Run("calc_library_default_recursion", testCalcLibraryDefaultRecursion)
	t.Run("calc_library_default_failure_names_one_frame", testCalcLibraryDefaultFailureNamesOneFrame)
	t.Run("calc_recursion_spends_step_budget", testCalcRecursionSpendsStepBudget)
	t.Run("calc_recursion_at_depth_ceiling", testCalcRecursionAtDepthCeiling)
	t.Run("calc_non_terminating_loop", testCalcNonTerminatingLoop)
	t.Run("calc_body_never_returns", testCalcBodyNeverReturns)
	t.Run("calc_send_posts_the_message", testCalcSendPostsTheMessage)
	t.Run("calc_send_to_no_object", testCalcSendToNoObject)
	t.Run("calc_terminate_ends_its_steps", testCalcTerminateEndsItsSteps)
	t.Run("calc_assignment_of_a_wrong_typed_value", testCalcAssignmentOfAWrongTypedValue)
	t.Run("calc_assignment_through_this_is_read_back", testCalcAssignmentThroughThisIsReadBack)
	t.Run("nested_calc_assigns_the_enclosing_calcs_value", testNestedCalcAssignsTheEnclosingCalcsValue)
	t.Run("nested_calc_assigns_a_wrong_typed_enclosing_value", testNestedCalcAssignsAWrongTypedEnclosingValue)
	t.Run("nested_calc_write_refreshes_the_enclosing_bindings", testNestedCalcWriteRefreshesTheEnclosingBindings)
	t.Run("calc_assignment_through_this_to_a_constant", testCalcAssignmentThroughThisToAConstant)
	t.Run("calc_assignment_through_this_past_a_shadowing_local", testCalcAssignmentThroughThisPastAShadowingLocal)
	t.Run("calc_output_write_of_a_wrong_typed_value", testCalcOutputWriteOfAWrongTypedValue)
}

// testCalcUnboundParameter: a parameter with neither an argument nor a default
// is a modeling error, not a null value.
func testCalcUnboundParameter(t *testing.T) {
	src := `
		package test {
			calc add {
				in x: Integer[1];
				in y: Integer[1];
				x + y
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "add", ast.DefCalc)
	if sym == nil {
		t.Fatal("add calc not found")
	}

	// Invoke with only 1 argument (missing y)
	xVal := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	result, err := ctx.InvokeCalc(sym, []Value{xVal}, rootScope)
	if err == nil {
		t.Fatalf("expected an unbound parameter error, calc returned %+v", result)
	}
	if !errors.Is(err, errUnboundParameter) {
		t.Errorf("expected errUnboundParameter, got: %v", err)
	}
}

// testCalcThisDefaultRequiredParameterUnbound: a parameter's default evaluating
// `this` materializes the occurrence before the later parameter binds, and the
// required [1] input an argument never bound still fails the invocation as
// unbound, whatever shape the occurrence already holds.
func testCalcThisDefaultRequiredParameterUnbound(t *testing.T) {
	src := `package test {
		calc def Pair {
			in self = this;
			in b : Integer[1];
			return : Integer = b->size();
		}
	}`
	err := invokeCalcExpecting(t, src, "test::Pair()")
	if !errors.Is(err, errUnboundParameter) {
		t.Fatalf("error = %v, want errUnboundParameter for the required b", err)
	}
	if !strings.Contains(err.Error(), `"b"`) {
		t.Errorf("error = %v, want it naming the parameter b", err)
	}
}

// testCalcCallsAnUnimportedExtensionFunction: `exp(x)` with no import of the
// OpenSysML extension library is reported unresolved by name resolution, so the
// call fails with a typed error naming the declaration whose package an import
// would make visible, rather than being answered.
func testCalcCallsAnUnimportedExtensionFunction(t *testing.T) {
	src := `
		package test {
			calc grow {
				in x: Real;
				exp(x)
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "grow", ast.DefCalc)
	if sym == nil {
		t.Fatal("grow calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 1}}
	result, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if err == nil {
		t.Fatalf("expected an unresolved-reference error, calc returned %+v", result)
	}
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("expected ErrUnresolvedReference, got: %v", err)
	}
	if want := ": unresolved reference: exp — did you mean OpenSysMLMathFunctions::exp? To use the bare name, import its package: private import OpenSysMLMathFunctions::*;"; !strings.HasSuffix(err.Error(), want) {
		t.Errorf("error %q does not end in %q", err, want)
	}
}

// testCalcCallsAnUnimportedLibraryFunction: `wheels->size()` in a model that
// imports no part of SequenceFunctions is reported unresolved by name
// resolution, so the call fails with a typed error carrying the validator's own
// hint — the library declarations of that name, whose package an import would
// make visible — rather than being answered by dispatch on the bare name. The
// same body evaluates once the import is written.
func testCalcCallsAnUnimportedLibraryFunction(t *testing.T) {
	model := func(imports string) string {
		return `
		package test {
			` + imports + `
			attribute wheels : Integer[*] = (1, 2, 3, 4);
			calc count {
				wheels->size()
			}
		}
	`
	}

	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, model("")))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "count", ast.DefCalc)
	if sym == nil {
		t.Fatal("count calc not found")
	}
	result, err := ctx.InvokeCalc(sym, nil, rootScope)
	if err == nil {
		t.Fatalf("expected an unresolved-reference error, calc returned %+v", result)
	}
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("expected ErrUnresolvedReference, got: %v", err)
	}
	if want := ": unresolved reference: size — did you mean SequenceFunctions::size or CollectionFunctions::size?"; !strings.HasSuffix(err.Error(), want) {
		t.Errorf("error %q does not end in %q", err, want)
	}

	idx, _, ctx = buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, model("private import SequenceFunctions::*;")))
	rootScope = idx.DocumentRoot("<test>")
	sym = findSymbolByName(rootScope, "count", ast.DefCalc)
	if sym == nil {
		t.Fatal("count calc not found")
	}
	result, err = ctx.InvokeCalc(sym, nil, rootScope)
	result = soleElement(result)
	if err != nil || result.Kind != ValConst || result.Const.Int != 4 {
		t.Fatalf("wheels->size() under import SequenceFunctions::* = %+v, %v; want 4", result, err)
	}
}

// testCalcUnboundKeywordNamedParameter: a parameter named with a keyword is a
// parameter like any other, so leaving it unbound reports, never panics.
func testCalcUnboundKeywordNamedParameter(t *testing.T) {
	src := `
		package test {
			calc classify {
				in 'type': Integer[1];
				in 'state': Integer[1];
				'type' + 'state'
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "classify", ast.DefCalc)
	if sym == nil {
		t.Fatal("classify calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	result, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if err == nil {
		t.Fatalf("expected an unbound parameter error, calc returned %+v", result)
	}
	if !errors.Is(err, errUnboundParameter) {
		t.Errorf("expected errUnboundParameter, got: %v", err)
	}
}

// testCalcTooManyArguments: more arguments than parameters has no binding, so it
// reports an arity error instead of dropping the extras.
func testCalcTooManyArguments(t *testing.T) {
	src := `
		package test {
			calc double {
				in x: Integer;
				x * 2
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "double", ast.DefCalc)
	if sym == nil {
		t.Fatal("double calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	_, err := ctx.InvokeCalc(sym, []Value{arg, arg}, rootScope)
	if err == nil {
		t.Fatal("expected an arity error, the calc accepted a surplus argument")
	}
	if !errors.Is(err, errCalcArity) {
		t.Errorf("expected errCalcArity, got: %v", err)
	}
}

// testCalcUnknownNamedArgument: a named argument that matches no parameter is
// reported instead of silently leaving the parameter on its default.
func testCalcUnknownNamedArgument(t *testing.T) {
	src := `
		package test {
			calc scale {
				in x: Integer = 1;
				x * 2
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "scale", ast.DefCalc)
	if sym == nil {
		t.Fatal("scale calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	_, err := ctx.invokeCalcNamed(sym, map[string]Value{"factor": arg}, rootScope)
	if err == nil {
		t.Fatal("expected an unknown parameter error, the invocation succeeded")
	}
	if !errors.Is(err, errUnknownParameter) {
		t.Errorf("expected errUnknownParameter, got: %v", err)
	}
}

// testCalcParameterNamedTwice: an invocation naming one parameter twice is
// reported rather than binding the later value.
func testCalcParameterNamedTwice(t *testing.T) {
	src := `
		package test {
			calc def Scale { in x : Integer; in factor : Integer = 2; return : Integer = x * factor; }
			calc twice { Scale(x = 1, x = 3, factor = 4) }
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "twice", ast.DefCalc)
	if sym == nil {
		t.Fatal("twice calc not found")
	}
	_, err := ctx.InvokeCalc(sym, nil, rootScope)
	if !errors.Is(err, errCalcArity) || !strings.Contains(err.Error(), `binds parameter "x" twice`) {
		t.Errorf("expected errCalcArity naming x, got: %v", err)
	}
}

// testCalcWithoutResult: a calc body with no return expression has no value to
// produce, own or inherited.
func testCalcWithoutResult(t *testing.T) {
	src := `
		package test {
			calc empty {
				in x: Integer;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "empty", ast.DefCalc)
	if sym == nil {
		t.Fatal("empty calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	_, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if err == nil {
		t.Fatal("expected a missing-result error, the calc returned a value")
	}
	if !errors.Is(err, ErrNoValue) {
		t.Errorf("expected ErrNoValue, got: %v", err)
	}
}

// testCalcStatesSecondResult: a calc stating a body over an inherited result
// expression is refused, not computed from a body of the runtime's choosing.
func testCalcStatesSecondResult(t *testing.T) {
	src := `
		package test {
			calc def Plus {
				in x: Integer;
				x + 1
			}
			calc def Twice :> Plus {
				x + 2
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Twice", ast.DefCalc)
	if sym == nil {
		t.Fatal("Twice calc not found")
	}

	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	_, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if !errors.Is(err, errConflictingResultExpressions) {
		t.Fatalf("expected errConflictingResultExpressions, got: %v", err)
	}
	for _, want := range []string{"test::Twice", "test::Plus"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %s", err, want)
		}
	}
}

// testCalcBodyStatesTwoResults: a calc body listing two bare expressions has
// two result expressions; neither it nor a calc inheriting them is computed
// from the first.
func testCalcBodyStatesTwoResults(t *testing.T) {
	src := `
		package test {
			calc def Twice {
				in x: Integer;
				x + 1
				x + 2
			}
			calc def Inherited :> Twice;
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 1}}
	for name, want := range map[string]string{
		"Twice":     "calc test::Twice states 2 result expressions",
		"Inherited": "from each of test::Twice, test::Twice",
	} {
		sym := findSymbolByName(rootScope, name, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", name)
		}
		_, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
		if !errors.Is(err, errConflictingResultExpressions) {
			t.Fatalf("%s: expected errConflictingResultExpressions, got: %v", name, err)
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error %q does not say %q", name, err, want)
		}
	}
}

// testCalcSymbolIsNotACalc: invoking a non-calc symbol is rejected by kind
// rather than by whatever its body happens to contain.
func testCalcSymbolIsNotACalc(t *testing.T) {
	src := `
		package test {
			part def Engine {
				attribute power : Integer;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Engine", ast.DefPart)
	if sym == nil {
		t.Fatal("Engine part def not found")
	}

	_, err := ctx.InvokeCalc(sym, nil, rootScope)
	if err == nil {
		t.Fatal("expected a not-a-calc error, the invocation succeeded")
	}
	if !errors.Is(err, ErrNotACalc) {
		t.Errorf("expected ErrNotACalc, got: %v", err)
	}
}

// testCalcDirectRecursion: a calc that invokes itself unconditionally never
// terminates, so the run's calc depth budget must report it instead of the
// process exhausting its stack.
func testCalcDirectRecursion(t *testing.T) {
	src := `
		package test {
			calc countdown {
				in n: Integer;
				countdown(n - 1)
			}
		}
	`
	assertCalcRecursionBounded(t, src, "countdown", ErrCalcRecursionLimit)
}

// testCalcMutualRecursion: the budget is spent by nesting, so a cycle through
// another calc is reported the same way direct self-invocation is.
func testCalcMutualRecursion(t *testing.T) {
	src := `
		package test {
			calc ping {
				in n: Integer;
				pong(n)
			}

			calc pong {
				in n: Integer;
				ping(n)
			}
		}
	`
	assertCalcRecursionBounded(t, src, "ping", ErrCalcRecursionLimit)
}

// testCalcDefaultRecursion: a default re-invoking its own calc nests through the
// binding rather than the body, and is bounded and collapsed the same way.
func testCalcDefaultRecursion(t *testing.T) {
	src := `
		package test {
			calc def f {
				in x : Integer;
				in y : Integer = f(x);
				return : Integer = x;
			}
		}
	`
	assertCalcRecursionBounded(t, src, "f", ErrCalcRecursionLimit)
}

// testCalcLibraryDefaultFailureNamesOneFrame: a default failing once on a library
// specialization is reported under one calc name, as a calc with a body reports it.
func testCalcLibraryDefaultFailureNamesOneFrame(t *testing.T) {
	src := `
		package test {
			import ScalarValues::*;
			import RealFunctions::*;
			calc def again :> max {
				in x :>> x;
				in y :>> y = missing;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "again", ast.DefCalc)
	if sym == nil {
		t.Fatal("calc again not found")
	}
	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 10}}
	_, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if err == nil {
		t.Fatal("expected the unresolved default to fail the invocation")
	}
	if got, prefix := err.Error(), `calc test::again: default for parameter "y": `; !strings.HasPrefix(got, prefix) || strings.Count(got, "calc test::again") != 1 {
		t.Errorf("err = %q; want one frame %q", got, prefix)
	}
}

// testCalcLibraryDefaultRecursion: a library specialization whose default invokes
// itself nests through its own binding, so the depth budget reports it like any calc.
func testCalcLibraryDefaultRecursion(t *testing.T) {
	src := `
		package test {
			import ScalarValues::*;
			import RealFunctions::*;
			calc def again :> max {
				in x :>> x;
				in y :>> y = again(x);
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	assertCalcInvocationBounded(t, idx, ctx, "again", ErrCalcRecursionLimit)
}

// testCalcRecursionSpendsStepBudget: the two bounds are independent, so a
// recursion whose evaluations run out first is reported by the step budget
// rather than running on until the depth bound.
func testCalcRecursionSpendsStepBudget(t *testing.T) {
	src := `
		package test {
			calc grow {
				in n: Integer;
				return : Integer = n + grow(n + 1);
			}
		}
	`
	assertCalcRecursionBounded(t, src, "grow", ErrStepLimitExceeded, func(ctx *Context) {
		// Room to recurse far deeper than the evaluations allow.
		ctx.maxCalcDepth = maxCalcDepthCeiling
		ctx.maxSteps = 500
	})
}

// testCalcRecursionAtDepthCeiling: the highest depth budget a run may be given
// must still be reported rather than reached by exhausting the stack, which
// would be fatal.
func testCalcRecursionAtDepthCeiling(t *testing.T) {
	src := `
		package test {
			calc deep {
				in n: Integer;
				attribute acc : Integer = (n + 1) * (n + 2) - n * n;
				return : Integer = acc + deep(n + 1);
			}
		}
	`
	assertCalcRecursionBounded(t, src, "deep", ErrCalcRecursionLimit, func(ctx *Context) {
		ctx.maxCalcDepth = maxCalcDepthCeiling
	})
}

// assertCalcRecursionBounded invokes calcName and requires the given budget
// error promptly: the invocation runs on its own goroutine so a hang fails the
// case instead of stalling the suite until the package timeout, and a panic in
// it fails the case rather than the package.
func assertCalcRecursionBounded(t *testing.T, src, calcName string, want error, budgets ...func(*Context)) {
	t.Helper()
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	assertCalcInvocationBounded(t, idx, ctx, calcName, want, budgets...)
}

func assertCalcInvocationBounded(t *testing.T, idx *symbols.Index, ctx *Context, calcName string, want error, budgets ...func(*Context)) {
	t.Helper()

	// The default step budget, so a recursion bounded by depth reaches that bound
	// rather than running out of evaluations first.
	ctx.maxSteps = DefaultMaxSteps
	for _, set := range budgets {
		set(ctx)
	}
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, calcName, ast.DefCalc)
	if sym == nil {
		t.Fatalf("calc %s not found", calcName)
	}

	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("calc %s panicked: %v", calcName, r)
			}
		}()
		arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 10}}
		_, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected recursive calc %s to be bounded, it returned a value", calcName)
		}
		if !errors.Is(err, want) {
			t.Errorf("expected %v, got: %v", want, err)
		}
		// One line per frame would make the message (and the memory building
		// it) grow with the square of the depth.
		if msg := err.Error(); len(msg) > 1024 {
			t.Errorf("error for recursive calc %s is %d bytes; want frames collapsed: %.200s…", calcName, len(msg), msg)
		}
	case <-watchdog(30 * time.Second):
		t.Fatalf("recursive calc %s did not terminate", calcName)
	}
}

// testCalcNonTerminatingLoop: a calc loop whose condition always holds spends
// the context's step budget, so it fails the invocation instead of hanging the
// REPL, LSP or gRPC caller that drove it.
func testCalcNonTerminatingLoop(t *testing.T) {
	src := `
		package test {
			calc spin {
				in n: Integer;
				attribute i : Integer = 0;
				while i >= 0 {
					assign i := i + 1;
				}
				return : Integer = i;
			}
		}
	`
	err := invokeCalcInSource(t, src, "spin", 1, 20)
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Errorf("expected ErrStepLimitExceeded, got: %v", err)
	}
}

// testCalcBodyNeverReturns: a body that computes but reaches no `return` leaves
// the result unbound, which is no value rather than a null one.
func testCalcBodyNeverReturns(t *testing.T) {
	src := `
		package test {
			calc maybe {
				in n: Integer;
				attribute total : Integer = 0;
				if n > 0 {
					assign total := n;
				}
				if n < 0 {
					return : Integer = total;
				}
			}
		}
	`
	err := invokeCalcInSource(t, src, "maybe", 5, 10000)
	if !errors.Is(err, ErrNoValue) {
		t.Errorf("expected ErrNoValue, got: %v", err)
	}
}

// calcOnHost invokes calc test::Host::<name> of src on a fresh Host and returns
// the runtime, the Host and the error.
func calcOnHost(t *testing.T, src, name string) (*Context, *Instance, error) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	root := idx.DocumentRoot("<test>")
	inst := mustInstantiate(t, ctx, findSymbolByName(root, "Host", ast.DefPart))
	_, err := ctx.invokeCalcWithSelf(oneSymbol(t, idx, "test::Host::"+name), calcArgs{}, root, inst)
	return ctx, inst, err
}

const calcSendingHost = `package test {
	private import ScalarValues::*;
	item def Ping;
	part def Worker;
	part def Host {
		part worker : Worker;
		calc def Notify {
			send new Ping() to worker;
			return : Integer = 1;
		}
		calc def NotifyNumber {
			in level : Integer = 4;
			send new Ping() to level;
			return : Integer = 1;
		}
	}
}`

// testCalcSendPostsTheMessage: a calculation may have side effects (KerML 7.4.8.1),
// so a send in its body posts its message to the object it addresses.
func testCalcSendPostsTheMessage(t *testing.T) {
	ctx, host, err := calcOnHost(t, calcSendingHost, "Notify")
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}
	worker, ok, err := ctx.fvObject(host, "worker")
	if err != nil || !ok {
		t.Fatalf("Host.worker: ok=%v err=%v", ok, err)
	}
	if len(ctx.messages) != 1 || ctx.messages[0].SignalType != "Ping" || ctx.messages[0].Object != worker.ID {
		t.Fatalf("messages = %+v, want one Ping addressed to the worker #%d", ctx.messages, worker.ID)
	}
}

// testCalcSendToNoObject: a send in a calc body answers to the rules of any send,
// so a target holding a number is refused and nothing is posted.
func testCalcSendToNoObject(t *testing.T) {
	ctx, _, err := calcOnHost(t, calcSendingHost, "NotifyNumber")
	if !errors.Is(err, errSendTargetNotObject) {
		t.Fatalf("err = %v, want errSendTargetNotObject", err)
	}
	if len(ctx.messages) != 0 {
		t.Errorf("messages = %+v, want none posted by the refused send", ctx.messages)
	}
}

// testCalcTerminateEndsItsSteps: a terminate ends the calculation's own performance (SysML v2
// §7.17.10), so later steps are not taken; the result, a binding, answers with the values held.
func testCalcTerminateEndsItsSteps(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		calc def Halting {
			in n : Integer;
			attribute x : Integer := n;
			if n > 2 { terminate; }
			assign x := x + 10;
			return : Integer = x;
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	for arg, want := range map[int64]string{1: "(11)", 5: "(5)"} {
		got, err := ctx.invokeCalcWithSelf(oneSymbol(t, idx, "test::Halting"),
			calcArgs{positional: []Value{integerValue(arg)}}, idx.DocumentRoot("<test>"), nil)
		if err != nil {
			t.Fatalf("Halting(%d): %v", arg, err)
		}
		if FormatTraceValue(got) != want {
			t.Errorf("Halting(%d) = %s, want %s", arg, FormatTraceValue(got), want)
		}
	}
}

// testCalcAssignmentOfAWrongTypedValue: a calc body writing a feature of the
// object performing it is held to that feature's declaration, as an action is.
func testCalcAssignmentOfAWrongTypedValue(t *testing.T) {
	ctx, host, err := calcOnHost(t, `package test {
		private import ScalarValues::*;
		part def Host {
			attribute count : Integer := 0;
			calc def Mistype {
				assign count := "three";
				return : Integer = 1;
			}
		}
	}`, "Mistype")
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("err = %v, want ErrTypeMismatch", err)
	}
	if fv, err := host.GetFeatureValue(ctx, "count"); err != nil {
		t.Fatalf("read Host.count: %v", err)
	} else if got := fv.HeldValue(); got.Const.Int != 0 {
		t.Errorf("Host.count = %v, want 0: the refused write changed nothing", got)
	}
}

// testCalcAssignmentThroughThisIsReadBack: `this.x` in a calculation names the
// value the body holds as x, so a read after the write answers what was written.
func testCalcAssignmentThroughThisIsReadBack(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		calc def Overwrite { in x : Integer; assign this.x := 5; return : Integer[1] = x; }
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	got, err := ctx.invokeCalcWithSelf(oneSymbol(t, idx, "test::Overwrite"),
		calcArgs{positional: []Value{integerValue(1)}}, idx.DocumentRoot("<test>"), nil)
	if err != nil {
		t.Fatalf("Overwrite(1): %v", err)
	}
	if got.Const.Int != 5 {
		t.Errorf("Overwrite(1) = %v, want 5: the value written through this", got)
	}
}

const nestedCalcWrites = `package test {
	private import ScalarValues::*;
	calc def Outer {
		attribute n : Integer := 0;
		calc inner { assign n := %s; return : Integer[1] = 1; }
		%s
	}
}`

// testNestedCalcAssignsTheEnclosingCalcsValue: a calculation declared within
// another writes the value the enclosing run holds, not a copy of its own.
func testNestedCalcAssignsTheEnclosingCalcsValue(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, fmt.Sprintf(nestedCalcWrites, "3", "return : Integer[1] = inner() + n;")))
	got, err := ctx.invokeCalcWithSelf(oneSymbol(t, idx, "test::Outer"), calcArgs{}, idx.DocumentRoot("<test>"), nil)
	if err != nil {
		t.Fatalf("Outer(): %v", err)
	}
	if got.Const.Int != 4 {
		t.Errorf("Outer() = %v, want 4: inner's 1 and the 3 it wrote to n", got)
	}
}

// testNestedCalcAssignsAWrongTypedEnclosingValue: the write to the enclosing
// run's value answers to that value's declaration.
func testNestedCalcAssignsAWrongTypedEnclosingValue(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, fmt.Sprintf(nestedCalcWrites, `"three"`, "return : Integer[1] = inner();")))
	_, err := ctx.invokeCalcWithSelf(oneSymbol(t, idx, "test::Outer"), calcArgs{}, idx.DocumentRoot("<test>"), nil)
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("err = %v, want ErrTypeMismatch", err)
	}
}

// testNestedCalcWriteRefreshesTheEnclosingBindings: a value of the enclosing run
// bound to the one a nested calculation writes is computed again after the write.
func testNestedCalcWriteRefreshesTheEnclosingBindings(t *testing.T) {
	src := fmt.Sprintf(nestedCalcWrites, "3", "attribute m : Integer = n + 1; return : Integer[1] = m + inner() + m;")
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	got, err := ctx.invokeCalcWithSelf(oneSymbol(t, idx, "test::Outer"), calcArgs{}, idx.DocumentRoot("<test>"), nil)
	if err != nil {
		t.Fatalf("Outer(): %v", err)
	}
	if got.Const.Int != 6 {
		t.Errorf("Outer() = %v, want 6: m is 1 before the write, inner answers 1, m is 4 after", got)
	}
}

// testCalcAssignmentThroughThisToAConstant: `this.k` names the calculation's own
// constant, which no assignment changes however it is reached.
func testCalcAssignmentThroughThisToAConstant(t *testing.T) {
	src := `package test {
		private import ScalarValues::*;
		calc def Fixed { constant attribute k : Integer = 1; assign this.k := 5; return : Integer[1] = k; }
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	_, err := ctx.invokeCalcWithSelf(oneSymbol(t, idx, "test::Fixed"), calcArgs{}, idx.DocumentRoot("<test>"), nil)
	if !errors.Is(err, errReadOnlyFeature) {
		t.Fatalf("err = %v, want errReadOnlyFeature", err)
	}
}

// testCalcAssignmentThroughThisPastAShadowingLocal: `this.x` names the calculation's own x,
// so a block's local of that name neither types nor freezes the write, and x's declaration does.
func testCalcAssignmentThroughThisPastAShadowingLocal(t *testing.T) {
	const model = `package test {
		private import ScalarValues::*;
		calc def C { %s return : Integer[1] = x; }
	}`
	for _, c := range []struct {
		name, body string
		want       int64
		err        error
	}{
		{"parameter under a local of another type", `in x : Integer; if true { attribute x : String := "s"; assign this.x := 5; }`, 5, nil},
		{"root local under a loop's local", `in n : Integer; attribute x : Integer := n; attribute i : Integer := 0;
			while i < 1 { attribute x : String := "s"; assign this.x := 7; assign i := i + 1; }`, 7, nil},
		{"parameter under a constant local", `in x : Integer; if true { constant attribute x : Integer = 2; assign this.x := 5; }`, 5, nil},
		{"value of the local's type, not the parameter's", `in x : Integer; if true { attribute x : String := "s"; assign this.x := "t"; }`, 0, ErrTypeMismatch},
		{"constant under a variable local", `in n : Integer; constant attribute x : Integer = 2; if true { attribute x : Integer := 1; assign this.x := 5; }`, 0, errReadOnlyFeature},
	} {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, fmt.Sprintf(model, c.body)))
		got, err := ctx.invokeCalcWithSelf(oneSymbol(t, idx, "test::C"),
			calcArgs{positional: []Value{integerValue(1)}}, idx.DocumentRoot("<test>"), nil)
		if !errors.Is(err, c.err) || (c.err == nil && err != nil) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.err)
		} else if c.err == nil && got.Const.Int != c.want {
			t.Errorf("%s: C(1) = %v, want %d", c.name, got, c.want)
		}
	}
}

// testCalcOutputWriteOfAWrongTypedValue: an output a calculation body binds is a
// feature too, so what it is bound to conforms to its declared type.
func testCalcOutputWriteOfAWrongTypedValue(t *testing.T) {
	src := `
	package test {
		private import ScalarValues::*;
		calc def Label {
			out tag : String;
			assign tag := 7;
		}
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	calc := oneSymbol(t, idx, "test::Label")
	_, err := ctx.InvokeCalc(calc, nil, idx.DocumentRoot("<test>"))
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("error = %v, want ErrTypeMismatch", err)
	}
}
