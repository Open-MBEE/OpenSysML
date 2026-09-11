package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Failure modes of overload selection: each returns a typed error naming the
// declarations at play, never a silent pick.
func TestInvocationSelectionRobustness(t *testing.T) {
	t.Run("calc_call_ambiguous_between_two_imports", testCalcCallAmbiguousBetweenTwoImports)
	t.Run("calc_call_ambiguity_names_only_the_tied_best", testCalcCallAmbiguityNamesOnlyTheTiedBest)
	t.Run("calc_call_fits_no_visible_candidate", testCalcCallFitsNoVisibleCandidate)
	t.Run("calc_call_fits_no_visible_candidate_behind_a_non_callable", testCalcCallFitsNoVisibleCandidateBehindANonCallable)
	t.Run("calc_call_selects_by_argument_type", testCalcCallSelectsByArgumentType)
	t.Run("calc_call_omitting_an_input_runs_the_fitting_candidate", testCalcCallOmittingAnInputRunsTheFittingCandidate)
	t.Run("calc_call_omitting_every_input_is_ambiguous", testCalcCallOmittingEveryInputIsAmbiguous)
	t.Run("calc_call_selects_a_feature_typed_by_a_calc", testCalcCallSelectsAFeatureTypedByACalc)
	t.Run("calc_call_applies_the_library_function_a_feature_is_typed_by", testCalcCallAppliesTheLibraryFunctionAFeatureIsTypedBy)
	t.Run("calc_call_binds_a_library_function_through_redeclared_inputs", testCalcCallBindsALibraryFunctionThroughRedeclaredInputs)
	t.Run("calc_call_selects_a_calc_over_a_more_specific_action", testCalcCallSelectsACalcOverAMoreSpecificAction)
	t.Run("global_root_call_selects_by_argument_type", testGlobalRootCallSelectsByArgumentType)
	t.Run("bare_call_selects_among_other_documents_root_declarations", testBareCallSelectsAmongOtherDocumentsRootDeclarations)
	t.Run("bare_call_reaches_private_root_declarations_of_other_documents", testBareCallReachesPrivateRootDeclarationsOfOtherDocuments)
	t.Run("calc_call_selects_by_named_argument", testCalcCallSelectsByNamedArgument)
	t.Run("calc_call_selects_by_sibling_scalar_type", testCalcCallSelectsBySiblingScalarType)
	t.Run("calc_call_typed_parameter_beats_untyped", testCalcCallTypedParameterBeatsUntyped)
	t.Run("calc_call_explicit_anything_ties_with_untyped", testCalcCallExplicitAnythingTiesWithUntyped)
	t.Run("calc_call_crossed_specificity_is_ambiguous", testCalcCallCrossedSpecificityIsAmbiguous)
	t.Run("calc_call_undetermined_statically_is_settled_by_the_values", testCalcCallUndeterminedStaticallyIsSettledByTheValues)
	t.Run("calc_call_repeated_named_argument_is_refused", testCalcCallRepeatedNamedArgumentIsRefused)
	t.Run("calc_call_names_a_parameter_as_the_checker_does", testCalcCallNamesAParameterAsTheCheckerDoes)
	t.Run("calc_call_selects_among_owned_inherited_and_recursive_import", testCalcCallSelectsAmongOwnedInheritedAndRecursiveImport)
	t.Run("calc_call_selects_among_inherited_imports", testCalcCallSelectsAmongInheritedImports)
	t.Run("calc_call_selects_by_collection_literal_element_type", testCalcCallSelectsByCollectionLiteralElementType)
	t.Run("action_call_selects_by_argument_type", testActionCallSelectsByArgumentType)
	t.Run("action_call_ambiguous_between_two_imports", testActionCallAmbiguousBetweenTwoImports)
	t.Run("action_call_selects_an_action_over_a_more_specific_calc", testActionCallSelectsAnActionOverAMoreSpecificCalc)
	t.Run("action_call_naming_only_a_calc_is_not_an_action", testActionCallNamingOnlyACalcIsNotAnAction)
	t.Run("action_call_receiver_binds_first_input", testActionCallReceiverBindsFirstInput)
	t.Run("action_call_receiver_with_named_arguments_refused", testActionCallReceiverWithNamedArgumentsRefused)
	t.Run("action_call_named_argument_twice_refused", testActionCallNamedArgumentTwiceRefused)
	t.Run("action_call_names_a_parameter_as_the_checker_does", testActionCallNamesAParameterAsTheCheckerDoes)
	t.Run("action_call_omits_optional_inputs", testActionCallOmitsOptionalInputs)
	t.Run("action_call_arguments_read_the_performing_object", testActionCallArgumentsReadThePerformingObject)
	t.Run("state_behavior_call_arguments_read_the_performing_object", testStateBehaviorCallArgumentsReadThePerformingObject)
	t.Run("action_call_argument_cannot_name_a_feature_out_of_scope", testActionCallArgumentCannotNameAFeatureOutOfScope)
}

// overloadedActionsSrc declares two imported same-named actions, told apart
// by the type of their one input.
const overloadedActionsSrc = `
	package A {
		private import ScalarValues::*;
		action def tag { in x : Integer; out code : Integer; first start; action set { assign code := 1; } done; succession first start then set; succession first set then done; }
	}
	package B {
		private import ScalarValues::*;
		action def tag { in x : %s; out code : Integer; first start; action set { assign code := 2; } done; succession first start then set; succession first set then done; }
	}
	package test {
		private import ScalarValues::*;
		private import A::*;
		private import B::*;
		action def Outer {
			attribute code : Integer = 0;
			first start;
			action call = tag(%s);
			done;
			succession first start then call;
			succession first call then done;
		}
	}
`

func runOverloadedAction(t *testing.T, paramType, arg string) (map[string]Value, error) {
	t.Helper()
	src := fmt.Sprintf(overloadedActionsSrc, paramType, arg)
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	outer := findSymbolByName(idx.DocumentRoot("<test>"), "Outer", ast.DefAction)
	if outer == nil {
		t.Fatal("Outer action not found")
	}
	return ctx.ExecuteAction(outer)
}

// A nested action invocation runs the same-named action its argument fits,
// as a calc call does.
func testActionCallSelectsByArgumentType(t *testing.T) {
	for arg, want := range map[string]int64{"3": 1, `"s"`: 2} {
		outputs, err := runOverloadedAction(t, "String", arg)
		if err != nil {
			t.Fatalf("tag(%s): %v", arg, err)
		}
		if got := intOutput(t, outputs, "code"); got != want {
			t.Errorf("tag(%s) code = %d, want %d", arg, got, want)
		}
	}
}

// An action performed by name runs an action of that name: a same-named calc a
// calc call would prefer, its parameter fitting the argument more closely, is not a
// candidate for an action performance.
func testActionCallSelectsAnActionOverAMoreSpecificCalc(t *testing.T) {
	src := `
		package A {
			private import ScalarValues::*;
			calc def tag { in x : Integer; return : Integer = x + 1; }
		}
		package B {
			private import ScalarValues::*;
			action def tag { in x : Real; out code : Integer; first start; action set { assign code := 2; } done; succession first start then set; succession first set then done; }
		}
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			action def Outer {
				attribute code : Integer = 0;
				first start;
				action call = tag(3);
				done;
				succession first start then call;
				succession first call then done;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	outer := findSymbolByName(idx.DocumentRoot("<test>"), "Outer", ast.DefAction)
	if outer == nil {
		t.Fatal("Outer action not found")
	}
	outputs, err := ctx.ExecuteAction(outer)
	if err != nil {
		t.Fatalf("tag(3): %v", err)
	}
	if got := intOutput(t, outputs, "code"); got != 2 {
		t.Errorf("tag(3) code = %d, want 2 from B::tag", got)
	}
}

// An action performed by a name only calcs declare is refused as not an action,
// whether the name denotes one calc or several.
func testActionCallNamingOnlyACalcIsNotAnAction(t *testing.T) {
	const src = `
		package A {
			private import ScalarValues::*;
			calc def tag { in x : Integer; return : Integer = x + 1; }
		}
		package B {
			private import ScalarValues::*;
			calc def tag { in x : String; return : Integer = 2; }
		}
		package test {
			private import ScalarValues::*;
			private import A::*;
			%s
			action def Outer {
				attribute code : Integer = 0;
				first start;
				action call = tag(3);
				done;
				succession first start then call;
				succession first call then done;
			}
		}
	`
	for _, imports := range []string{``, `private import B::*;`} {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, fmt.Sprintf(src, imports)))
		outer := findSymbolByName(idx.DocumentRoot("<test>"), "Outer", ast.DefAction)
		if outer == nil {
			t.Fatal("Outer action not found")
		}
		_, err := ctx.ExecuteAction(outer)
		if err == nil || !strings.Contains(err.Error(), "tag is not an action (calcDef)") {
			t.Errorf("imports %q: error = %v, want 'tag is not an action (calcDef)'", imports, err)
		}
	}
}

// receiverActionsSrc declares two same-named actions whose outputs depend on
// the input, so a call shows both which one ran and what it was bound to.
const receiverActionsSrc = `
	package A {
		private import ScalarValues::*;
		action def tag { in x : Integer; out code : Integer; first start; action set { assign code := x + 10; } done; succession first start then set; succession first set then done; }
	}
	package B {
		private import ScalarValues::*;
		action def tag { in x : Integer; in y : Integer; out code : Integer; first start; action set { assign code := x * 100 + y; } done; succession first start then set; succession first set then done; }
	}
	package test {
		private import ScalarValues::*;
		private import A::*;
		private import B::*;
		action def Outer {
			attribute code : Integer = 0;
			first start;
			action call = %s;
			done;
			succession first start then call;
			succession first call then done;
		}
	}
`

func runReceiverAction(t *testing.T, call string) (map[string]Value, error) {
	t.Helper()
	src := fmt.Sprintf(receiverActionsSrc, call)
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	outer := findSymbolByName(idx.DocumentRoot("<test>"), "Outer", ast.DefAction)
	if outer == nil {
		t.Fatal("Outer action not found")
	}
	return ctx.ExecuteAction(outer)
}

// A receiver written before a nested action call is its first argument, both
// for choosing among the same-named actions and for binding the chosen one.
func testActionCallReceiverBindsFirstInput(t *testing.T) {
	for call, want := range map[string]int64{"3->tag()": 13, "3->tag(4)": 304, "tag(3, 4)": 304} {
		outputs, err := runReceiverAction(t, call)
		if err != nil {
			t.Fatalf("%s: %v", call, err)
		}
		if got := intOutput(t, outputs, "code"); got != want {
			t.Errorf("%s code = %d, want %d", call, got, want)
		}
	}
}

// A receiver binds by position and so has no place beside named arguments.
func testActionCallReceiverWithNamedArgumentsRefused(t *testing.T) {
	outputs, err := runReceiverAction(t, "3->tag(y = 4)")
	if err == nil {
		t.Fatalf("expected a refusal, action returned %v", outputs)
	}
	if !errors.Is(err, ErrReceiverWithNamedArgs) {
		t.Fatalf("expected ErrReceiverWithNamedArgs, got: %v", err)
	}
}

// A named argument written twice is refused before any action runs, whether the
// name denotes one action or several, as a calc call refuses it.
func testActionCallNamedArgumentTwiceRefused(t *testing.T) {
	outputs, err := runReceiverAction(t, "tag(x = 3, x = 4)")
	if err == nil {
		t.Fatalf("overloaded tag(x = 3, x = 4): expected a refusal, action returned %v", outputs)
	}
	if !errors.Is(err, ErrDuplicateArgument) || !strings.Contains(err.Error(), `input parameter "x" of tag`) {
		t.Fatalf("overloaded tag(x = 3, x = 4): error = %v, want ErrDuplicateArgument naming x", err)
	}
	const single = `
		package test {
			private import ScalarValues::*;
			action def tag { in x : Integer; out code : Integer; first start; action set { assign code := x + 10; } done; succession first start then set; succession first set then done; }
			action def Outer {
				attribute code : Integer = 0;
				first start;
				action call = tag(x = 3, x = 4);
				done;
				succession first start then call;
				succession first call then done;
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, single))
	outer := findSymbolByName(idx.DocumentRoot("<test>"), "Outer", ast.DefAction)
	if outer == nil {
		t.Fatal("Outer action not found")
	}
	outputs, err = ctx.ExecuteAction(outer)
	if err == nil {
		t.Fatalf("single tag(x = 3, x = 4): expected a refusal, action returned %v", outputs)
	}
	if !errors.Is(err, ErrDuplicateArgument) || !strings.Contains(err.Error(), `input parameter "x" of tag`) {
		t.Fatalf("single tag(x = 3, x = 4): error = %v, want ErrDuplicateArgument naming x", err)
	}
}

// An action's named argument binds the parameter the checker resolves its label
// to — by short name, alias, or qualified name — and two labels naming one
// parameter are one binding made twice.
func testActionCallNamesAParameterAsTheCheckerDoes(t *testing.T) {
	const tmpl = `
		package test {
			private import ScalarValues::*;
			action def A { in <ys> y : Integer; alias yy for y; out code : Integer; first start; action set { assign code := y + 10; } done; succession first start then set; succession first set then done; }
			package Other { attribute y : Integer = 5; }
			action def B { in 'Other::y' : Integer; out code : Integer; first start; action set { assign code := 'Other::y' + 10; } done; succession first start then set; succession first set then done; }
			action def Outer {
				attribute code : Integer = 0;
				first start;
				action call = %s;
				done;
				succession first start then call;
				succession first call then done;
			}
		}
	`
	run := func(call string) (map[string]Value, error) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, fmt.Sprintf(tmpl, call)))
		outer := findSymbolByName(idx.DocumentRoot("<test>"), "Outer", ast.DefAction)
		if outer == nil {
			t.Fatal("Outer action not found")
		}
		return ctx.ExecuteAction(outer)
	}
	for _, call := range []string{"A(ys = 3)", "A(yy = 3)", "A(A::y = 3)", "B('Other::y' = 3)"} {
		outputs, err := run(call)
		if err != nil {
			t.Fatalf("%s: %v", call, err)
		}
		if code := outputs["call.code"]; code.Kind != ValConst || code.Const.Int != 13 {
			t.Fatalf("%s: call.code = %+v, want 13", call, code)
		}
	}
	for _, call := range []string{"A(y = 1, ys = 2)", "A(y = 1, ys = 2, Other::y = 3)"} {
		outputs, err := run(call)
		if err == nil {
			t.Fatalf("%s: expected a refusal, action returned %v", call, outputs)
		}
		if !errors.Is(err, ErrDuplicateArgument) || !strings.Contains(err.Error(), `input parameter "y" of A`) {
			t.Fatalf("%s: error = %v, want ErrDuplicateArgument naming y", call, err)
		}
	}
	for _, call := range []string{"A(Other::y = 1)", "B(Other::y = 1)"} {
		outputs, err := run(call)
		if err == nil {
			t.Fatalf("%s: expected a refusal, action returned %v", call, outputs)
		}
		if !errors.Is(err, ErrUnknownParameter) || !strings.Contains(err.Error(), `has no parameter named "Other::y"`) {
			t.Fatalf("%s: error = %v, want ErrUnknownParameter naming Other::y", call, err)
		}
	}
}

// An empty call binds an input that may hold no value, read as empty, and one whose
// default reaches it along its redefinitions; an input with neither stays unbound.
func testActionCallOmitsOptionalInputs(t *testing.T) {
	const outer = `
		package test {
			private import ScalarValues::*;
			private import B::*;
			action def Outer {
				attribute code : Integer = 0;
				first start;
				action call = tag();
				done;
				succession first start then call;
				succession first call then done;
			}
		}
	`
	run := func(tag string) (map[string]Value, error) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, tag+outer))
		outer := findSymbolByName(idx.DocumentRoot("<test>"), "Outer", ast.DefAction)
		if outer == nil {
			t.Fatal("Outer action not found")
		}
		return ctx.ExecuteAction(outer)
	}

	outputs, err := run(`
		package B {
			private import ScalarValues::*;
			private import SequenceFunctions::*;
			action def tag { in x : Integer[0..1]; out code : Integer; first start; action set { assign code := x->size() + (if x->isEmpty() ? 100 else 200); } done; succession first start then set; succession first set then done; }
		}`)
	if err != nil {
		t.Fatalf("tag() with x : Integer[0..1]: %v", err)
	}
	if got := intOutput(t, outputs, "code"); got != 100 {
		t.Errorf("tag() with x : Integer[0..1]: code = %d, want 100 (x empty)", got)
	}

	outputs, err = run(`
		package B {
			private import ScalarValues::*;
			action def base { in x : Integer = 3; out code : Integer; }
			action def tag :> base { in x : Integer :>> x; first start; action set { assign code := x + 10; } done; succession first start then set; succession first set then done; }
		}`)
	if err != nil {
		t.Fatalf("tag() with x redefining a defaulted input: %v", err)
	}
	if got := intOutput(t, outputs, "code"); got != 13 {
		t.Errorf("tag() with x redefining a defaulted input: code = %d, want 13 (the inherited default 3)", got)
	}

	_, err = run(`
		package B {
			private import ScalarValues::*;
			action def base { in x : Integer = 3; out code : Integer; }
			action def tag :> base { in x : Integer :>> x; in y : Integer; first start; action set { assign code := x + y; } done; succession first start then set; succession first set then done; }
		}`)
	if !errors.Is(err, ErrUnboundParameter) || !strings.Contains(err.Error(), "input parameter y") {
		t.Fatalf("tag() with a required y: err = %v, want ErrUnboundParameter for y", err)
	}
}

// performedCallSrc has a part perform a behavior calling an action with the part's own
// feature, whose instance value differs from the default; receiver and argument forms.
const performedCallSrc = `
	package P {
		private import ScalarValues::*;
		action def report { in x : Integer; out code : Integer; first start; action set { assign code := x + 10; } done; succession first start then set; succession first set then done; }
		part def Rover {
			attribute speed : Integer = 7;
			action def drive {
				attribute code : Integer = 0;
				first start;
				%s
				done;
				succession first start then call;
				succession first call then done;
			}
			state def Drive {
				attribute code : Integer = 0;
				entry; then run;
				state run { %s }
				transition first run then done;
			}
		}
		part rover1 : Rover { attribute redefines speed = 9; }
	}
`

// performedCallRuntime builds the model with the given action and state calls
// and instantiates the performing part.
func performedCallRuntime(t *testing.T, actionCall, stateCall string) (*symbols.Index, *Context, *Instance) {
	t.Helper()
	src := fmt.Sprintf(performedCallSrc, actionCall, stateCall)
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	self, err := ctx.Instantiate(oneSymbol(t, idx, "P::rover1"))
	if err != nil {
		t.Fatalf("instantiate rover1: %v", err)
	}
	return idx, ctx, self
}

// A nested action call's arguments, receiver included, read the performing object:
// `speed->report()` passes the instance's speed, not the declared default.
func testActionCallArgumentsReadThePerformingObject(t *testing.T) {
	for _, call := range []string{"action call = speed->report();", "perform action call = report(speed);"} {
		idx, ctx, self := performedCallRuntime(t, call, "")
		outputs, err := ctx.ExecuteActionPerformedBy(oneSymbol(t, idx, "P::Rover::drive"), self, nil)
		if err != nil {
			t.Fatalf("%s: %v", call, err)
		}
		if got := intOutput(t, outputs, "code"); got != 19 {
			t.Errorf("%s code = %d, want 19 (rover1's speed 9 + 10)", call, got)
		}
	}
}

// A state behavior's action call reads the performing object the same way.
func testStateBehaviorCallArgumentsReadThePerformingObject(t *testing.T) {
	for _, call := range []string{"entry perform action w = speed->report();", "exit perform action w = report(speed);"} {
		idx, ctx, self := performedCallRuntime(t, "", call)
		data, _, err := ctx.ExecuteStatePerformedBy(oneSymbol(t, idx, "P::Rover::Drive"), self, nil)
		if err != nil {
			t.Fatalf("%s: %v", call, err)
		}
		if got := intOutput(t, data, "code"); got != 19 {
			t.Errorf("%s code = %d, want 19 (rover1's speed 9 + 10)", call, got)
		}
	}
}

// An argument reaches the performing object only through names the caller's
// scope resolves to its features, not through every feature the instance has.
func testActionCallArgumentCannotNameAFeatureOutOfScope(t *testing.T) {
	src := `package P {
		private import ScalarValues::*;
		action def report { in x : Integer; out code : Integer; first start; action set { assign code := x + 10; } done; succession first start then set; succession first set then done; }
		part def Rover { attribute speed : Integer = 7; }
		action def drive {
			attribute code : Integer = 0;
			first start;
			action call = report(speed);
			done;
			succession first start then call;
			succession first call then done;
		}
		part rover1 : Rover;
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	self, err := ctx.Instantiate(oneSymbol(t, idx, "P::rover1"))
	if err != nil {
		t.Fatalf("instantiate rover1: %v", err)
	}
	outputs, err := ctx.ExecuteActionPerformedBy(oneSymbol(t, idx, "P::drive"), self, nil)
	if err == nil {
		t.Fatalf("expected an unresolved reference, action returned %v", outputs)
	}
	if !errors.Is(err, ErrUnresolvedReference) || !strings.Contains(err.Error(), "speed") {
		t.Fatalf("expected ErrUnresolvedReference naming speed, got: %v", err)
	}
}

// Two imported actions taking the same argument type are refused as ambiguous.
func testActionCallAmbiguousBetweenTwoImports(t *testing.T) {
	outputs, err := runOverloadedAction(t, "Integer", "3")
	if err == nil {
		t.Fatalf("expected an ambiguity error, action returned %v", outputs)
	}
	if !errors.Is(err, ErrAmbiguousInvocation) {
		t.Fatalf("expected ErrAmbiguousInvocation, got: %v", err)
	}
	for _, want := range []string{"A::tag", "B::tag"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name candidate %s", err, want)
		}
	}
}

// Two imported calcs with the same name take the same argument type, and the
// call is refused as ambiguous rather than answered by whichever came first.
func testCalcCallAmbiguousBetweenTwoImports(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def pick { in x : Integer; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def pick { in x : Integer; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			calc choose { in v : Integer; pick(v) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "choose", ast.DefCalc)
	if sym == nil {
		t.Fatal("choose calc not found")
	}
	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	result, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if err == nil {
		t.Fatalf("expected an ambiguity error, calc returned %+v", result)
	}
	if !errors.Is(err, ErrAmbiguousInvocation) {
		t.Fatalf("expected ErrAmbiguousInvocation, got: %v", err)
	}
	for _, want := range []string{"A::pick", "B::pick"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name candidate %s", err, want)
		}
	}
}

// A broader overload the arguments also fit is beaten by the tied ones, so the
// ambiguity error names only those.
func testCalcCallAmbiguityNamesOnlyTheTiedBest(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def pick { in x : Integer; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def pick { in x : Integer; return : Integer = 2; } }
		package C { private import ScalarValues::*; calc def pick { in x : Real; return : Integer = 3; } }
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			private import C::*;
			calc choose { pick(3) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "choose", ast.DefCalc)
	if sym == nil {
		t.Fatal("choose calc not found")
	}
	_, err := ctx.InvokeCalc(sym, nil, rootScope)
	if !errors.Is(err, ErrAmbiguousInvocation) {
		t.Fatalf("expected ErrAmbiguousInvocation, got: %v", err)
	}
	if !strings.HasSuffix(err.Error(), "pick denotes A::pick, B::pick") {
		t.Errorf("error %q should name exactly the tied overloads A::pick, B::pick", err)
	}
}

// No candidate takes a String, so the call fails with a type mismatch that
// still names the first candidate's expectation.
func testCalcCallFitsNoVisibleCandidate(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def pick { in x : Integer; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def pick { in x : Boolean; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			calc choose { in v : String; pick(v) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "choose", ast.DefCalc)
	if sym == nil {
		t.Fatal("choose calc not found")
	}
	arg := NewStringValue("x")
	result, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
	if err == nil {
		t.Fatalf("expected a type mismatch, calc returned %+v", result)
	}
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("expected ErrTypeMismatch, got: %v", err)
	}
}

// A non-callable declaration found before the callable one does not take the
// call: the mismatch is reported against the callable's parameter.
func testCalcCallFitsNoVisibleCandidateBehindANonCallable(t *testing.T) {
	src := `
		package A { attribute def pick; }
		package B { private import ScalarValues::*; calc def pick { in x : Integer; return : Integer = x; } }
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			calc choose { in v : String; pick(v) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "choose", ast.DefCalc)
	if sym == nil {
		t.Fatal("choose calc not found")
	}
	result, err := ctx.InvokeCalc(sym, []Value{NewStringValue("x")}, rootScope)
	if err == nil {
		t.Fatalf("expected a type mismatch, calc returned %+v", result)
	}
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("expected ErrTypeMismatch, got: %v", err)
	}
}

// The candidate whose parameter type the argument conforms to is the one run.
func testCalcCallSelectsByArgumentType(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def pick { in x : Integer; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def pick { in x : String; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			calc chooseInt { in v : Integer; pick(v) }
			calc chooseStr { in v : String; pick(v) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	cases := []struct {
		calc string
		arg  Value
		want int64
	}{
		{"chooseInt", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}, 1},
		{"chooseStr", NewStringValue("s"), 2},
	}
	for _, tc := range cases {
		sym := findSymbolByName(rootScope, tc.calc, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", tc.calc)
		}
		result, err := ctx.InvokeCalc(sym, []Value{tc.arg}, rootScope)
		if err != nil {
			t.Fatalf("%s: %v", tc.calc, err)
		}
		if result.Kind != ValConst || result.Const.Int != tc.want {
			t.Fatalf("%s = %+v, want %d", tc.calc, result, tc.want)
		}
	}
}

// A call leaving a default-less input of the only candidate its argument fits dispatches
// to that candidate, as the checker binds it, and fails there on the unbound parameter;
// a candidate the argument binds completely is run instead when there is one.
func testCalcCallOmittingAnInputRunsTheFittingCandidate(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def pick { in x : Integer; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def pick { in s : String; in y : Real; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			calc choose { in v : String; pick(v) }
			calc complete { in v : Integer; pick(v) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	choose := findSymbolByName(rootScope, "choose", ast.DefCalc)
	if choose == nil {
		t.Fatal("choose calc not found")
	}
	result, err := ctx.InvokeCalc(choose, []Value{NewStringValue("x")}, rootScope)
	if err == nil {
		t.Fatalf("pick(v) with y unbound: expected a refusal, calc returned %+v", result)
	}
	if !errors.Is(err, ErrUnboundParameter) || errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("pick(v) with y unbound: error = %v, want ErrUnboundParameter", err)
	}
	complete := findSymbolByName(rootScope, "complete", ast.DefCalc)
	if complete == nil {
		t.Fatal("complete calc not found")
	}
	arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	result, err = ctx.InvokeCalc(complete, []Value{arg}, rootScope)
	if err != nil {
		t.Fatalf("pick(3): %v", err)
	}
	if result.Kind != ValConst || result.Const.Int != 1 {
		t.Fatalf("pick(3) = %+v, want 1", result)
	}
}

// A call writing no argument tells two candidates apart by nothing, so it is ambiguous
// rather than dispatched to whichever is found first.
func testCalcCallOmittingEveryInputIsAmbiguous(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def pick { in x : Integer; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def pick { in s : String; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			calc choose { pick() }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "choose", ast.DefCalc)
	if sym == nil {
		t.Fatal("choose calc not found")
	}
	result, err := ctx.InvokeCalc(sym, nil, rootScope)
	if err == nil {
		t.Fatalf("pick(): expected an ambiguity error, calc returned %+v", result)
	}
	if !errors.Is(err, ErrAmbiguousInvocation) {
		t.Fatalf("pick(): expected ErrAmbiguousInvocation, got: %v", err)
	}
}

// A feature typed by a calc is a candidate performing that calc: it is run when
// only its signature fits, and the same-named calc when only that one does.
func testCalcCallSelectsAFeatureTypedByACalc(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def pick { in x : String; return : Integer = 1; } }
		package B {
			private import ScalarValues::*;
			calc def Twice { in x : Integer; return : Integer = 2 * x; }
			ref pick : Twice;
		}
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			calc chooseInt { in v : Integer; pick(v) }
			calc chooseStr { in v : String; pick(v) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	cases := []struct {
		calc string
		arg  Value
		want int64
	}{
		{"chooseInt", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}, 6},
		{"chooseStr", NewStringValue("s"), 1},
	}
	for _, tc := range cases {
		sym := findSymbolByName(rootScope, tc.calc, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", tc.calc)
		}
		result, err := ctx.InvokeCalc(sym, []Value{tc.arg}, rootScope)
		if err != nil {
			t.Fatalf("%s: %v", tc.calc, err)
		}
		if result.Kind != ValConst || result.Const.Int != tc.want {
			t.Fatalf("%s = %+v, want %d", tc.calc, result, tc.want)
		}
	}
}

// A feature typed by a library function — directly, or through bodiless model calcs
// specializing it — performs that function's implementation, by position or by
// parameter name, as does a bodiless calc def specializing it. A model calc stating
// its own body keeps it.
func testCalcCallAppliesTheLibraryFunctionAFeatureIsTypedBy(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			private import RealFunctions::sqrt;
			private import SequenceFunctions::size;
			ref root : sqrt;
			calc rootCalc : sqrt;
			ref count : size;
			calc def halfRoot :> sqrt { in x : Real; return : Real = sqrt(x) / 2.0; }
			ref half : halfRoot;
			calc def Root :> sqrt;
			calc def Root2 :> Root;
			ref viaDefs : Root2;
			calc positional { root(16.0) }
			calc named { root(x = 16.0) }
			calc viaCalcUsage { rootCalc(16.0) }
			calc collection { count((1, 2, 3)) }
			calc ownBody { half(16.0) }
			calc aliasDef { Root(16.0) }
			calc aliasDefNamed { Root2(x = 16.0) }
			calc viaAliasDefs { viaDefs(16.0) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	cases := []struct {
		calc string
		want Value
	}{
		{"positional", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 4}}},
		{"named", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 4}}},
		{"viaCalcUsage", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 4}}},
		{"collection", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}},
		{"ownBody", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 2}}},
		{"aliasDef", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 4}}},
		{"aliasDefNamed", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 4}}},
		{"viaAliasDefs", Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 4}}},
	}
	for _, tc := range cases {
		sym := findSymbolByName(rootScope, tc.calc, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", tc.calc)
		}
		result, err := ctx.InvokeCalc(sym, nil, rootScope)
		if err != nil {
			t.Fatalf("%s: %v", tc.calc, err)
		}
		if !valueEqual(result, tc.want) {
			t.Fatalf("%s = %+v, want %+v", tc.calc, result, tc.want)
		}
	}
}

// A bodiless calc def specializing a library function binds a call's arguments
// through its own effective inputs — renamed, defaulted, re-multiplied or added by
// position — and hands them to the library implementation in its parameter order;
// so does a feature typed by it, and a direct invocation of either.
func testCalcCallBindsALibraryFunctionThroughRedeclaredInputs(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			private import RealFunctions::sqrt;
			private import RealFunctions::max;
			private import SequenceFunctions::isEmpty;
			calc def Renamed :> sqrt { in y :>> x; }
			calc def Defaulted :> sqrt { in x :>> x = 16.0; }
			calc def ByPosition :> sqrt { in q : Real; }
			calc def Floor :> max { in floor :>> y = 0.0; }
			calc def Empty :> isEmpty { in items :>> seq; }
			calc def Required :> isEmpty { in seq :>> seq [1]; }
			calc def Narrowed :> sqrt { in n : Integer :>> x; }
			calc def AtLeastNext :> max { in x :>> x; in y :>> y = x + 1.0; }
			calc def OwnBody :> sqrt { in y :>> x; return : Real = y; }
			ref renamed : Renamed;
			ref defaulted : Defaulted;
			calc renamedNamed { Renamed(y = 16.0) }
			calc renamedPositional { Renamed(16.0) }
			calc renamedOldName { Renamed(x = 16.0) }
			calc renamedUnbound { Renamed() }
			calc defaulted0 { Defaulted() }
			calc defaultedGiven { Defaulted(x = 25.0) }
			calc byPosition { ByPosition(4.0) }
			calc floorApplies { Floor(x = -3.0) }
			calc floorGiven { Floor(x = 5.0, floor = 7.0) }
			calc floorUnbound { Floor(2.0) }
			calc emptyOmitted { Empty() }
			calc emptyGiven { Empty(items = 3) }
			calc requiredOmitted { Required() }
			calc requiredNull { Required(null) }
			calc narrowedFits { Narrowed(4) }
			calc narrowedReal { Narrowed(2.5) }
			calc nextDefault { AtLeastNext(2.0) }
			calc nextGiven { AtLeastNext(2.0, 1.0) }
			calc ownBody { OwnBody(16.0) }
			calc featureNamed { renamed(y = 16.0) }
			calc featureDefault { defaulted() }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	realVal := func(f float64) Value {
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: f}}
	}
	boolean := func(b bool) Value {
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: b}}
	}
	for _, tc := range []struct {
		calc string
		want Value
	}{
		{"renamedNamed", realVal(4)},
		{"renamedPositional", realVal(4)},
		{"defaulted0", realVal(4)},
		{"defaultedGiven", realVal(5)},
		{"byPosition", realVal(2)},
		{"floorApplies", realVal(0)},
		{"floorGiven", realVal(7)},
		{"emptyOmitted", boolean(true)},
		{"emptyGiven", boolean(false)},
		{"narrowedFits", realVal(2)},
		{"nextDefault", realVal(3)},
		{"nextGiven", realVal(2)},
		{"ownBody", realVal(16)},
		{"featureNamed", realVal(4)},
		{"featureDefault", realVal(4)},
	} {
		sym := findSymbolByName(rootScope, tc.calc, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", tc.calc)
		}
		result, err := ctx.InvokeCalc(sym, nil, rootScope)
		if err != nil {
			t.Fatalf("%s: %v", tc.calc, err)
		}
		if !valueEqual(result, tc.want) {
			t.Fatalf("%s = %+v, want %+v", tc.calc, result, tc.want)
		}
	}
	for _, tc := range []struct {
		calc string
		want error
	}{
		{"renamedOldName", ErrUnknownParameter},
		{"renamedUnbound", ErrUnboundParameter},
		{"floorUnbound", ErrUnboundParameter},
		{"requiredOmitted", ErrUnboundParameter},
		{"requiredNull", ErrMultiplicityViolation},
		{"narrowedReal", ErrTypeMismatch},
	} {
		sym := findSymbolByName(rootScope, tc.calc, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", tc.calc)
		}
		if _, err := ctx.InvokeCalc(sym, nil, rootScope); !errors.Is(err, tc.want) {
			t.Fatalf("%s error = %v, want %v", tc.calc, err, tc.want)
		}
	}

	renamed := findSymbolByName(rootScope, "Renamed", ast.DefCalc)
	if result, err := ctx.InvokeCalcNamed(renamed, map[string]Value{"y": realVal(9)}, rootScope); err != nil || !valueEqual(result, realVal(3)) {
		t.Fatalf("InvokeCalcNamed(Renamed, y = 9) = %+v, %v; want 3", result, err)
	}
	if result, err := ctx.InvokeCalc(renamed, []Value{realVal(9)}, rootScope); err != nil || !valueEqual(result, realVal(3)) {
		t.Fatalf("InvokeCalc(Renamed, 9) = %+v, %v; want 3", result, err)
	}
	if _, err := ctx.InvokeCalcNamed(renamed, map[string]Value{"x": realVal(9)}, rootScope); !errors.Is(err, ErrUnknownParameter) {
		t.Fatalf("InvokeCalcNamed(Renamed, x = 9) error = %v, want ErrUnknownParameter", err)
	}
	defaulted := findSymbolByName(rootScope, "Defaulted", ast.DefCalc)
	if result, err := ctx.InvokeCalc(defaulted, nil, rootScope); err != nil || !valueEqual(result, realVal(4)) {
		t.Fatalf("InvokeCalc(Defaulted) = %+v, %v; want 4", result, err)
	}
	floor := findSymbolByName(rootScope, "Floor", ast.DefCalc)
	if result, err := ctx.InvokeCalcNamed(floor, map[string]Value{"x": realVal(-3)}, rootScope); err != nil || !valueEqual(result, realVal(0)) {
		t.Fatalf("InvokeCalcNamed(Floor, x = -3) = %+v, %v; want 0", result, err)
	}
	narrowed := findSymbolByName(rootScope, "Narrowed", ast.DefCalc)
	if _, err := ctx.InvokeCalc(narrowed, []Value{realVal(2.5)}, rootScope); !errors.Is(err, ErrTypeMismatch) || !strings.Contains(err.Error(), "calc test::Narrowed") {
		t.Fatalf("InvokeCalc(Narrowed, 2.5) error = %v, want ErrTypeMismatch from test::Narrowed", err)
	}
	next := findSymbolByName(rootScope, "AtLeastNext", ast.DefCalc)
	if result, err := ctx.InvokeCalcNamed(next, map[string]Value{"x": realVal(2)}, rootScope); err != nil || !valueEqual(result, realVal(3)) {
		t.Fatalf("InvokeCalcNamed(AtLeastNext, x = 2) = %+v, %v; want 3", result, err)
	}
}

// An evaluated call runs a calc: a same-named action whose input fits the argument
// more closely is not a candidate, whichever import names it first; a name only
// actions declare is still refused as not a calc.
func testCalcCallSelectsACalcOverAMoreSpecificAction(t *testing.T) {
	const src = `
		package A { private import ScalarValues::*; action def pick { in x : Integer; out r : Integer; } }
		package B { private import ScalarValues::*; calc def pick { in x : Real; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			%s
			calc choose { in v : Integer; pick(v) }
			calc chooseAction { in v : Integer; A::pick(v) }
		}
	`
	three := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	for _, imports := range []string{
		"private import A::*; private import B::*;",
		"private import B::*; private import A::*;",
	} {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, fmt.Sprintf(src, imports)))
		rootScope := idx.DocumentRoot("<test>")
		choose := findSymbolByName(rootScope, "choose", ast.DefCalc)
		if choose == nil {
			t.Fatal("choose calc not found")
		}
		result, err := ctx.InvokeCalc(choose, []Value{three}, rootScope)
		if err != nil {
			t.Fatalf("%s: pick(3): %v", imports, err)
		}
		if result.Kind != ValConst || result.Const.Int != 2 {
			t.Fatalf("%s: pick(3) = %+v, want 2 from B::pick", imports, result)
		}
		chooseAction := findSymbolByName(rootScope, "chooseAction", ast.DefCalc)
		if chooseAction == nil {
			t.Fatal("chooseAction calc not found")
		}
		if _, err := ctx.InvokeCalc(chooseAction, []Value{three}, rootScope); !errors.Is(err, ErrNotACalc) {
			t.Fatalf("%s: A::pick(3) error = %v, want ErrNotACalc", imports, err)
		}
	}
	// With only the action fitting the argument, the calc is still the one called, and
	// the argument is refused against it as the checker reports it.
	onlyAction := strings.Replace(src, "in x : Real; return : Integer = 2;", "in x : String; return : Integer = 2;", 1)
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, fmt.Sprintf(onlyAction, "private import A::*; private import B::*;")))
	rootScope := idx.DocumentRoot("<test>")
	choose := findSymbolByName(rootScope, "choose", ast.DefCalc)
	_, err := ctx.InvokeCalc(choose, []Value{three}, rootScope)
	if !errors.Is(err, ErrTypeMismatch) || !strings.Contains(err.Error(), "calc B::pick") {
		t.Fatalf("pick(3) fitting only A::pick: error = %v, want ErrTypeMismatch from B::pick", err)
	}
}

// `$::pick(v)` selects among every root-level pick, not only the first indexed;
// the name is built since the expression parser does not read `$::`.
func testGlobalRootCallSelectsByArgumentType(t *testing.T) {
	src := `
		private import ScalarValues::*;
		calc def pick { in x : Integer; return : Integer = 1; }
		calc def pick { in x : String; return : Integer = 2; }
		calc def pick { in x : Boolean; return : Integer = 3; }
		package test { calc def pick { in x : Boolean; return : Integer = 4; } }
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	pkg, ok := rootScope.LookupLocal("test")
	if !ok || pkg.Scope == nil {
		t.Fatal("package test not indexed")
	}
	call := func(arg ast.Node) *ast.InvocationExpr {
		qn := &ast.QualifiedName{Global: true}
		qn.Parts = append(qn.Parts, ast.NameSegment{Text: "pick"})
		return &ast.InvocationExpr{Type: qn, Args: []ast.Node{arg}}
	}
	cases := []struct {
		name string
		arg  ast.Node
		want string
	}{
		{"3", &ast.LiteralInteger{Value: "3"}, "1"},
		{`"s"`, &ast.LiteralString{Value: "s"}, "2"},
		{"true", &ast.LiteralBool{Value: true}, "3"},
	}
	for _, tc := range cases {
		val, err := ctx.EvalWithScope(call(tc.arg), pkg.Scope)
		if err != nil || FormatValue(val) != tc.want {
			t.Errorf("$::pick(%s) = (%v, %v), want %s", tc.name, val, err, tc.want)
		}
	}
}

// A bare `pick(v)` that only other documents' root declarations answer selects
// among all of them, not only the first indexed.
func testBareCallSelectsAmongOtherDocumentsRootDeclarations(t *testing.T) {
	idx := libs.NewModelIndex()
	idx.AddDocument("ints.sysml", parseAndBuild(t, `
		private import ScalarValues::*;
		calc def pick { in x : Integer; return : Integer = 1; }
		calc def pick { in x : String; return : Integer = 2; }
	`))
	idx.AddDocument("bools.sysml", parseAndBuild(t, `
		private import ScalarValues::*;
		calc def pick { in x : Boolean; return : Integer = 3; }
	`))
	idx.AddDocument("<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			calc byInt { pick(3) }
			calc byString { pick("s") }
			calc byBool { pick(true) }
		}
	`))
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	ctx := NewContext(NewModel(semantics.NewModel(resolver), resolver), 10000)
	rootScope := idx.DocumentRoot("<test>")
	for calc, want := range map[string]int64{"byInt": 1, "byString": 2, "byBool": 3} {
		sym := findSymbolByName(rootScope, calc, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", calc)
		}
		result, err := ctx.InvokeCalc(sym, nil, rootScope)
		if err != nil {
			t.Fatalf("%s: %v", calc, err)
		}
		if result.Kind != ValConst || result.Const.Int != want {
			t.Fatalf("%s = %+v, want %d", calc, result, want)
		}
	}
}

// A root namespace has no owner to hide its members from (KerML 8.2.3.5; the pilot
// resolves them alike), so a private or protected root overload in another
// document is selected exactly as a public one is.
func testBareCallReachesPrivateRootDeclarationsOfOtherDocuments(t *testing.T) {
	idx := libs.NewModelIndex()
	idx.AddDocument("ints.sysml", parseAndBuild(t, `
		private import ScalarValues::*;
		calc def pick { in x : Integer; return : Integer = 1; }
	`))
	idx.AddDocument("strings.sysml", parseAndBuild(t, `
		private import ScalarValues::*;
		private calc def pick { in x : String; return : Integer = 2; }
	`))
	idx.AddDocument("bools.sysml", parseAndBuild(t, `
		private import ScalarValues::*;
		protected calc def pick { in x : Boolean; return : Integer = 3; }
	`))
	idx.AddDocument("<test>", parseAndBuild(t, `
		package test {
			private import ScalarValues::*;
			calc byInt { pick(3) }
			calc byString { pick("s") }
			calc byBool { pick(true) }
		}
	`))
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	ctx := NewContext(NewModel(semantics.NewModel(resolver), resolver), 10000)
	rootScope := idx.DocumentRoot("<test>")
	for calc, want := range map[string]int64{"byInt": 1, "byString": 2, "byBool": 3} {
		sym := findSymbolByName(rootScope, calc, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", calc)
		}
		result, err := ctx.InvokeCalc(sym, nil, rootScope)
		if err != nil {
			t.Fatalf("%s: %v", calc, err)
		}
		if result.Kind != ValConst || result.Const.Int != want {
			t.Fatalf("%s = %+v, want %d", calc, result, want)
		}
	}
}

// An untyped parameter takes anything, so the candidate typing it runs for the
// arguments it fits, whichever import lists first.
func testCalcCallTypedParameterBeatsUntyped(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def pick { in x : Real; in y; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def pick { in x : Real; in y : Integer; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			calc typed { in v : Integer; pick(1, v) }
			calc loose { in v : Integer; pick(1, "b") }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	for calc, want := range map[string]int64{"typed": 2, "loose": 1} {
		sym := findSymbolByName(rootScope, calc, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", calc)
		}
		arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
		result, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
		if err != nil {
			t.Fatalf("%s: %v", calc, err)
		}
		if result.Kind != ValConst || result.Const.Int != want {
			t.Fatalf("%s = %+v, want %d", calc, result, want)
		}
	}
}

// A parameter written `: Anything` and one declaring no type are the same parameter,
// so two candidates differing only in that spelling tie and the call is refused.
func testCalcCallExplicitAnythingTiesWithUntyped(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; private import Base::Anything; calc def pick { in x : Real; in y : Anything; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def pick { in x : Real; in y; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			calc choose { in w : Real; in v : Integer; pick(w, v) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "choose", ast.DefCalc)
	if sym == nil {
		t.Fatal("choose calc not found")
	}
	realVal := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 1.5}}
	integer := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	result, err := ctx.InvokeCalc(sym, []Value{realVal, integer}, rootScope)
	if err == nil {
		t.Fatalf("expected an ambiguity error, calc returned %+v", result)
	}
	if !errors.Is(err, ErrAmbiguousInvocation) {
		t.Fatalf("expected ErrAmbiguousInvocation, got: %v", err)
	}
	for _, want := range []string{"A::pick", "B::pick"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name candidate %s", err, want)
		}
	}
}

// Candidates each narrower on a different parameter are incomparable: neither is
// selected, whether the wide parameter is written `: Anything` or left untyped.
func testCalcCallCrossedSpecificityIsAmbiguous(t *testing.T) {
	src := `
		package Types { attribute def Foo; }
		package A { private import ScalarValues::*; private import Base::Anything; calc def pick { in x : Anything; in y : Real; return : Integer = 1; } }
		package B { private import ScalarValues::*; private import Types::Foo; calc def pick { in x : Foo; in y; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import Types::Foo;
			private import A::*;
			private import B::*;
			calc choose { in f : Foo; in w : Real; pick(f, w) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "choose", ast.DefCalc)
	if sym == nil {
		t.Fatal("choose calc not found")
	}
	inst, err := ctx.Instantiate(oneSymbol(t, idx, "Types::Foo"))
	if err != nil {
		t.Fatalf("instantiate Foo: %v", err)
	}
	foo := Value{Kind: ValInstance, Instance: inst.ID}
	realVal := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 1.5}}
	result, err := ctx.InvokeCalc(sym, []Value{foo, realVal}, rootScope)
	if err == nil {
		t.Fatalf("expected an ambiguity error, calc returned %+v", result)
	}
	if !errors.Is(err, ErrAmbiguousInvocation) {
		t.Fatalf("expected ErrAmbiguousInvocation, got: %v", err)
	}
	for _, want := range []string{"A::pick", "B::pick"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name candidate %s", err, want)
		}
	}
}

// An argument of statically unknown type leaves the overloads tied for the checker;
// the run selects by the values' types, and refuses values that still tie.
func testCalcCallUndeterminedStaticallyIsSettledByTheValues(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def pick { in x : Integer; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def pick { in x : String; return : Integer = 2; } }
		package C { private import ScalarValues::*; calc def pick { in x : Integer; in y : Real; return : Integer = 3; } }
		package D { private import ScalarValues::*; calc def pick { in x : Real; in y : Integer; return : Integer = 4; } }
		package test {
			private import A::*;
			private import B::*;
			private import C::*;
			private import D::*;
			calc choose { in v; pick(v) }
			calc choosePair { in v; in w; pick(v, w) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	choose := findSymbolByName(rootScope, "choose", ast.DefCalc)
	choosePair := findSymbolByName(rootScope, "choosePair", ast.DefCalc)
	if choose == nil || choosePair == nil {
		t.Fatal("choose calcs not found")
	}
	intVal := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	strVal := NewStringValue("s")
	for _, tc := range []struct {
		name string
		arg  Value
		want int64
	}{{"integer", intVal, 1}, {"string", strVal, 2}} {
		result, err := ctx.InvokeCalc(choose, []Value{tc.arg}, rootScope)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if result.Kind != ValConst || result.Const.Int != tc.want {
			t.Fatalf("%s: result = %+v, want %d", tc.name, result, tc.want)
		}
	}
	result, err := ctx.InvokeCalc(choosePair, []Value{intVal, intVal}, rootScope)
	if err == nil {
		t.Fatalf("expected an ambiguity error, calc returned %+v", result)
	}
	if !errors.Is(err, ErrAmbiguousInvocation) {
		t.Fatalf("expected ErrAmbiguousInvocation, got: %v", err)
	}
	for _, want := range []string{"C::pick", "D::pick"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name candidate %s", err, want)
		}
	}
	for _, unwanted := range []string{"A::pick", "B::pick"} {
		if strings.Contains(err.Error(), unwanted) {
			t.Errorf("error %q names %s, which the values do not fit", err, unwanted)
		}
	}
}

// A named argument written twice is refused among several candidates as it is for
// one, naming the parameter, rather than binding the later value.
func testCalcCallRepeatedNamedArgumentIsRefused(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def pick { in x : Integer; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def pick { in x : String; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			calc twice { in i : Integer; in s : String; pick(x = i, x = s) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "twice", ast.DefCalc)
	if sym == nil {
		t.Fatal("twice calc not found")
	}
	intArg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	_, err := ctx.InvokeCalc(sym, []Value{intArg, NewStringValue("s")}, rootScope)
	if !errors.Is(err, ErrCalcArity) || !strings.Contains(err.Error(), `binds parameter "x" twice`) {
		t.Fatalf("twice = %v, want ErrCalcArity naming x", err)
	}
}

// A named argument binds the parameter the checker resolves its label to — by
// short name, alias, or qualified inherited name — and two labels naming one
// parameter are one binding made twice.
func testCalcCallNamesAParameterAsTheCheckerDoes(t *testing.T) {
	src := `
		package test {
			private import ScalarValues::*;
			calc def F { in <xs> x : Integer; return : Integer = x + 1; }
			calc def G { in b : Integer; alias bs for b; return : Integer = b * 2; }
			calc def Base { in n : Integer; return : Integer; }
			calc def Derived :> Base { return : Integer = n + 10; }
			package Other { attribute x : Integer = 5; }
			calc def H { in 'Other::x' : Integer; return : Integer = 'Other::x' * 3; }
			calc viaShort { F(xs = 1) }
			calc viaAlias { G(bs = 2) }
			calc viaQualified { F(F::x = 3) }
			calc viaInherited { Derived(Base::n = 4) }
			calc viaUnrestricted { H('Other::x' = 5) }
			calc twice { F(x = 1, xs = 2) }
			calc twiceThenElsewhere { F(x = 1, xs = 2, Other::x = 3) }
			calc elsewhere { F(Other::x = 1) }
			calc elsewhereSpelled { H(Other::x = 1) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	for calc, want := range map[string]int64{"viaShort": 2, "viaAlias": 4, "viaQualified": 4, "viaInherited": 14, "viaUnrestricted": 15} {
		sym := findSymbolByName(rootScope, calc, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", calc)
		}
		result, err := ctx.InvokeCalc(sym, nil, rootScope)
		if err != nil {
			t.Fatalf("%s: %v", calc, err)
		}
		if result.Kind != ValConst || result.Const.Int != want {
			t.Fatalf("%s = %+v, want %d", calc, result, want)
		}
	}
	for _, calc := range []string{"twice", "twiceThenElsewhere"} {
		sym := findSymbolByName(rootScope, calc, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", calc)
		}
		_, err := ctx.InvokeCalc(sym, nil, rootScope)
		if !errors.Is(err, ErrCalcArity) || !strings.Contains(err.Error(), `binds parameter "x" twice`) {
			t.Fatalf("%s = %v, want ErrCalcArity naming x", calc, err)
		}
	}
	for _, calc := range []string{"elsewhere", "elsewhereSpelled"} {
		sym := findSymbolByName(rootScope, calc, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", calc)
		}
		_, err := ctx.InvokeCalc(sym, nil, rootScope)
		if !errors.Is(err, ErrUnknownParameter) || !strings.Contains(err.Error(), `has no parameter named "Other::x"`) {
			t.Fatalf("%s = %v, want ErrUnknownParameter naming Other::x", calc, err)
		}
	}
}

// Two candidates whose parameters specialize the same scalar type are told
// apart by the argument's declared type, not by the scalar both reduce to.
func testCalcCallSelectsBySiblingScalarType(t *testing.T) {
	src := `
		package Q { private import ScalarValues::*; attribute def Mass :> Real; attribute def Volume :> Real; }
		package A { private import ScalarValues::*; private import Q::*; calc def pick { in m : Mass; return : Integer = 1; } }
		package B { private import ScalarValues::*; private import Q::*; calc def pick { in v : Volume; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import Q::*;
			private import A::*;
			private import B::*;
			calc byMass { in kg : Mass; pick(kg) }
			calc byVolume { in litre : Volume; pick(litre) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	for calc, want := range map[string]int64{"byMass": 1, "byVolume": 2} {
		sym := findSymbolByName(rootScope, calc, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", calc)
		}
		arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: 3}}
		result, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
		if err != nil {
			t.Fatalf("%s: %v", calc, err)
		}
		if result.Kind != ValConst || result.Const.Int != want {
			t.Fatalf("%s = %+v, want %d", calc, result, want)
		}
	}
}

// Every protected import of a general type contributes its overload to the
// specializing body, not only the first import's.
func testCalcCallSelectsAmongInheritedImports(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def pick { in x : Integer; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def pick { in x : String; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			part def Base { protected import A::*; protected import B::*; }
			part def Derived :> Base {
				attribute i : Integer = pick(3);
				attribute s : Integer = pick("s");
			}
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	derived := findSymbolByName(idx.DocumentRoot("<test>"), "Derived", ast.DefPart)
	if derived == nil {
		t.Fatal("Derived part def not found")
	}
	inst, err := ctx.Instantiate(derived)
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	for attr, want := range map[string]string{"i": "1", "s": "2"} {
		fv, err := inst.GetFeatureValue(ctx, attr)
		if err != nil {
			t.Fatalf("%s: %v", attr, err)
		}
		if got := FormatTraceValue(fv.HeldValue()); got != want {
			t.Fatalf("%s = %s, want %s", attr, got, want)
		}
	}
}

// A collection literal is typed by its elements, so overloads differing only
// in element type are told apart.
func testCalcCallSelectsByCollectionLiteralElementType(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def count { in xs : String[*]; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def count { in xs : Integer[*]; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			calc ofText { count(("a", "b")) }
			calc ofNumbers { count((1, 2, 3)) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	for calc, want := range map[string]int64{"ofText": 1, "ofNumbers": 2} {
		sym := findSymbolByName(rootScope, calc, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", calc)
		}
		result, err := ctx.InvokeCalc(sym, nil, rootScope)
		if err != nil {
			t.Fatalf("%s: %v", calc, err)
		}
		if result.Kind != ValConst || result.Const.Int != want {
			t.Fatalf("%s = %+v, want %d", calc, result, want)
		}
	}
}

// Overloads reached as owned members, through two generals, and through two descendants
// of one recursive import are all candidates; each call runs the one its argument fits.
func testCalcCallSelectsAmongOwnedInheritedAndRecursiveImport(t *testing.T) {
	src := `
		package Owned {
			private import ScalarValues::*;
			calc def pick { in x : Integer; return : Integer = 1; }
			calc def pick { in x : String; return : Integer = 2; }
			calc byInt { in v : Integer; pick(v) }
			calc byStr { in v : String; pick(v) }
			calc qualifiedInt { in v : Integer; Owned::pick(v) }
			calc qualifiedStr { in v : String; Owned::pick(v) }
		}
		package Inherited {
			private import ScalarValues::*;
			calc def ByNumber { calc def pick { in x : Integer; return : Integer = 1; } }
			calc def ByText { calc def pick { in x : String; return : Integer = 2; } }
			calc def byInt :> ByNumber, ByText { in v : Integer; return : Integer = pick(v); }
			calc def byStr :> ByNumber, ByText { in v : String; return : Integer = pick(v); }
			calc def Base {
				calc def pick { in x : Integer; return : Integer = 1; }
				calc def pick { in x : String; return : Integer = 2; }
			}
			calc def oneBaseInt :> Base { in v : Integer; return : Integer = pick(v); }
			calc def oneBaseStr :> Base { in v : String; return : Integer = pick(v); }
		}
		package Recursive {
			private import ScalarValues::*;
			package Lib {
				package Numbers { calc def pick { in x : Integer; return : Integer = 1; } }
				package Text { calc def pick { in x : String; return : Integer = 2; } }
			}
			package C {
				private import ScalarValues::*;
				private import Lib::**;
				calc byInt { in v : Integer; pick(v) }
				calc byStr { in v : String; pick(v) }
			}
		}
		package Qualified {
			private import ScalarValues::*;
			package A { calc def pick { in x : Integer; return : Integer = 1; } }
			package B { calc def pick { in x : String; return : Integer = 2; } }
			package Both {
				public import A::*;
				public import B::*;
			}
			calc def Derived :> Inherited::ByNumber, Inherited::ByText;
			calc def Importing :> Inherited::ByText { public import A::*; }
			calc reexportedInt { in v : Integer; Both::pick(v) }
			calc reexportedStr { in v : String; Both::pick(v) }
			calc inheritedInt { in v : Integer; Derived::pick(v) }
			calc inheritedStr { in v : String; Derived::pick(v) }
			calc inheritedHidesImportStr { in v : String; Importing::pick(v) }
			calc inheritedHidesImportInt { in v : Integer; Importing::pick(v) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	intArg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
	cases := []struct {
		qual string
		arg  Value
		want int64
	}{
		{"Owned::byInt", intArg, 1},
		{"Owned::byStr", NewStringValue("s"), 2},
		{"Owned::qualifiedInt", intArg, 1},
		{"Owned::qualifiedStr", NewStringValue("s"), 2},
		{"Inherited::byInt", intArg, 1},
		{"Inherited::byStr", NewStringValue("s"), 2},
		{"Inherited::oneBaseInt", intArg, 1},
		{"Inherited::oneBaseStr", NewStringValue("s"), 2},
		{"Recursive::C::byInt", intArg, 1},
		{"Recursive::C::byStr", NewStringValue("s"), 2},
		{"Qualified::reexportedInt", intArg, 1},
		{"Qualified::reexportedStr", NewStringValue("s"), 2},
		{"Qualified::inheritedInt", intArg, 1},
		{"Qualified::inheritedStr", NewStringValue("s"), 2},
		{"Qualified::inheritedHidesImportStr", NewStringValue("s"), 2},
	}
	for _, tc := range cases {
		matches := idx.LookupQualified(tc.qual)
		if len(matches) != 1 {
			t.Fatalf("%s: %d symbols, want 1", tc.qual, len(matches))
		}
		result, err := ctx.InvokeCalc(matches[0], []Value{tc.arg}, rootScope)
		if err != nil {
			t.Fatalf("%s: %v", tc.qual, err)
		}
		if result.Kind != ValConst || result.Const.Int != tc.want {
			t.Fatalf("%s = %+v, want %d", tc.qual, result, tc.want)
		}
	}
	hidden := idx.LookupQualified("Qualified::inheritedHidesImportInt")
	if len(hidden) != 1 {
		t.Fatalf("inheritedHidesImportInt: %d symbols, want 1", len(hidden))
	}
	if _, err := ctx.InvokeCalc(hidden[0], []Value{intArg}, rootScope); err == nil ||
		!strings.Contains(err.Error(), "typed by String") {
		t.Fatalf("Importing::pick(3) ran the import the inherited pick hides: err = %v", err)
	}
}

// A named argument binds by the parameter's name, which is what tells two
// same-arity candidates apart.
func testCalcCallSelectsByNamedArgument(t *testing.T) {
	src := `
		package A { private import ScalarValues::*; calc def pick { in width : Integer; return : Integer = 1; } }
		package B { private import ScalarValues::*; calc def pick { in height : Integer; return : Integer = 2; } }
		package test {
			private import ScalarValues::*;
			private import A::*;
			private import B::*;
			calc byWidth { in v : Integer; pick(width = v) }
			calc byHeight { in v : Integer; pick(height = v) }
		}
	`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	rootScope := idx.DocumentRoot("<test>")
	for calc, want := range map[string]int64{"byWidth": 1, "byHeight": 2} {
		sym := findSymbolByName(rootScope, calc, ast.DefCalc)
		if sym == nil {
			t.Fatalf("%s calc not found", calc)
		}
		arg := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 3}}
		result, err := ctx.InvokeCalc(sym, []Value{arg}, rootScope)
		if err != nil {
			t.Fatalf("%s: %v", calc, err)
		}
		if result.Kind != ValConst || result.Const.Int != want {
			t.Fatalf("%s = %+v, want %d", calc, result, want)
		}
	}
}
