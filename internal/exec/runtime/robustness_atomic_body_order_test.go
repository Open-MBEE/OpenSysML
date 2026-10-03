package runtime

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
)

func TestRuntimeRobustnessAtomicBodyOrder(t *testing.T) {
	t.Run("explore_run_budget", testAtomicBodyOrderRunBudget)
	t.Run("commuting_calc_has_no_choice", testAtomicBodyOrderCommutingCalc)
	t.Run("compiled_caller_falls_back_for_ordered_schedules", testAtomicBodyOrderCompiledFallback)
	t.Run("compiled_cycles_propagate_reordering", testAtomicBodyOrderCompiledCycle)
	t.Run("nested_calc_lists_have_order_choices", testAtomicBodyOrderNestedLists)
	t.Run("fixed_policies_preserve_calc_and_action_order", testAtomicBodyOrderFixedPolicies)
	t.Run("fixed_policy_orders_backward_then", testAtomicBodyOrderFixedPolicyBackwardThen)
	t.Run("calc_statements_do_not_yield_to_action_body", testAtomicBodyOrderCalcDoesNotYield)
	t.Run("checker_sweep_cap_is_bounded", testAtomicBodyOrderCheckerSweepCap)
	t.Run("explore_sweep_cap_is_bounded", testAtomicBodyOrderExploreSweepCap)
	t.Run("recursive_result_choices_are_linear", testAtomicBodyOrderRecursiveResultChoices)
	t.Run("enclosing_calc_calls_do_not_reuse_result_memo", testAtomicBodyOrderEnclosingCalcMemo)
	t.Run("invocation_errors_are_result_alternatives", testAtomicBodyOrderInvocationErrorResults)
	t.Run("transitive_order_sensitivity_reaches_guards", testAtomicBodyOrderTransitiveGuards)
	t.Run("constraint_invocation_has_one_result_choice", testAtomicBodyOrderConstraintInvocationChoice)
	t.Run("nested_preview_refuses_order_dependent_invocation", testAtomicBodyOrderNestedPreviewInvocation)
	t.Run("calc_flow_refusal_names_the_construct", testAtomicBodyOrderCalcFlowRefusal)
	t.Run("calc_unmatched_succession_is_ignored", testAtomicBodyOrderCalcUnmatchedSuccession)
	t.Run("constraint_property_finds_an_order_violation", testAtomicBodyOrderConstraintViolation)
	t.Run("derived_final_orders_replay", testAtomicBodyOrderDerivedFinalReplay)
	t.Run("nested_preview_refuses_order_dependent_guard", testAtomicBodyOrderNestedPreviewGuard)
	t.Run("guard_refuses_external_constraint_writes", testAtomicBodyOrderGuardExternalWrite)
	t.Run("guard_preserves_error_as_an_outcome", testAtomicBodyOrderGuardErrorOutcome)
	t.Run("false_guard_witness_replays", testAtomicBodyOrderFalseGuardReplay)
	t.Run("recursive_invocation_preserves_typed_budget_error", testAtomicBodyOrderRecursiveBudget)
}

func testAtomicBodyOrderRunBudget(t *testing.T) {
	m := conformanceModel(t, "calc_explore_statement_order")
	x := m.exploreAction(t, "explore:runs=1", "Use")
	if x.Complete() || x.Runs != 1 || !slices.Contains(x.BudgetsHit, "runs") {
		t.Fatalf("exploration %s after %d runs, want the one-run budget hit", x.Status(), x.Runs)
	}
}

func testAtomicBodyOrderCommutingCalc(t *testing.T) {
	m := conformanceModel(t, "calc_explore_statement_order_commuting")
	x := m.exploreAction(t, "explore", "Use")
	if !x.Complete() || x.Runs != 1 || len(x.Outcomes) != 1 {
		t.Fatalf("exploration %s after %d runs, want one complete run and outcome", x.Status(), x.Runs)
	}
	if got := outcomeValue(t, x.Outcomes[0].Outcome, "r"); got != "5" {
		t.Fatalf("r = %s, want 5", got)
	}
	if len(x.Outcomes[0].Witness) != 0 {
		t.Fatalf("commuting calc recorded choice points: %v", x.Outcomes[0].Witness)
	}
}

func testAtomicBodyOrderCompiledFallback(t *testing.T) {
	source := `package test {
		private import ScalarValues::*;
		calc def Ord {
			return : Integer;
			if true {
				return : Integer = 12;
			} else {
				return : Integer = 13;
			}
			if true {
				return : Integer = 30;
			} else {
				return : Integer = 31;
			}
		}
		calc def Caller {
			return : Integer;
			return : Integer = Ord();
		}
		action def Use {
			out attribute r : Integer := 0;
			assign r := Caller();
		}
	}`
	m := parseLibraryModel(t, source)
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	ctx.SetCalcCompile(true)
	callerSymbol := namedOrFoundSymbol(
		t, m.idx, "test::Caller", m.idx.DocumentRoot(m.path), ast.DefCalc, ast.UsageCalc,
	)
	caller, err := ctx.calcShapeOf(callerSymbol)
	if err != nil {
		t.Fatal(err)
	}
	compiled := ctx.compiledCalcOf(caller)
	if compiled == nil || !ctx.reordersTransitively(callerSymbol) {
		t.Fatalf("compiled caller = %#v (ineligible: %q), transitively reordered = %t; want an eligible compiled body bypassed by runtime analysis",
			compiled, caller.ineligibleWhy, ctx.reordersTransitively(callerSymbol))
	}
	policy := mustPolicy(t, "explore")
	x, err := ExploreWith(context.Background(), policy, 1, func(int) (*Context, error) {
		fresh, err := m.fresh()
		if err == nil {
			fresh.SetCalcCompile(true)
		}
		return fresh, err
	}, actionRun(t, m.idx, m.path, "Use"))
	if err != nil {
		t.Fatal(err)
	}
	values := featureValues(t, x, "r")
	slices.Sort(values)
	if !x.Complete() || x.Runs != 2 || !slices.Equal(values, []string{"12", "30"}) {
		t.Fatalf("compiled caller exploration %s after %d runs: %v, want both results in two runs",
			x.Status(), x.Runs, outcomeTexts(x))
	}
	for _, outcome := range x.Outcomes {
		if len(outcome.Witness) != 1 || outcome.Witness[0].Kind != ChoiceStatementOrder {
			t.Fatalf("compiled caller witness = %v, want one result-level statement-order choice", outcome.Witness)
		}
	}
}

func testAtomicBodyOrderCompiledCycle(t *testing.T) {
	source := `package test {
		private import ScalarValues::*;
		calc def Even {
			in n : Integer;
			return : Integer;
			if n <= 0 { return : Integer = 1; }
			if n <= 0 { return : Integer = 2; }
			return : Integer = Odd(n - 1);
		}
		calc def Odd {
			in n : Integer;
			return : Integer = Even(n - 1);
		}
		action def Use {
			out attribute r : Integer := 0;
			assign r := Even(1);
		}
	}`
	m := parseLibraryModel(t, source)
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	ctx.SetCalcCompile(true)
	evenSymbol := namedOrFoundSymbol(
		t, m.idx, "test::Even", m.idx.DocumentRoot(m.path), ast.DefCalc, ast.UsageCalc,
	)
	oddSymbol := namedOrFoundSymbol(
		t, m.idx, "test::Odd", m.idx.DocumentRoot(m.path), ast.DefCalc, ast.UsageCalc,
	)
	even, err := ctx.calcShapeOf(evenSymbol)
	if err != nil {
		t.Fatal(err)
	}
	odd, err := ctx.calcShapeOf(oddSymbol)
	if err != nil {
		t.Fatal(err)
	}
	compiledEven := ctx.compiledCalcOf(even)
	compiledOdd := ctx.compiledCalcOf(odd)
	if compiledEven == nil || compiledOdd == nil ||
		!ctx.reordersTransitively(evenSymbol) || !ctx.reordersTransitively(oddSymbol) {
		t.Fatalf("compiled cycle = (%#v, %#v), transitive reordering = (%t, %t); want eligible bodies and cycle-safe analysis",
			compiledEven, compiledOdd, ctx.reordersTransitively(evenSymbol), ctx.reordersTransitively(oddSymbol))
	}
	mustSchedule(t, ctx, mustPolicy(t, "seed:1"))
	outputs, err := ctx.ExecuteAction(m.action(t, "Use"))
	if err != nil {
		t.Fatal(err)
	}
	if got := outcomeValue(t, ctx.ActionOutcome(outputs), "r"); !slices.Contains([]string{"1", "2"}, got) {
		t.Fatalf("recursive result = %s, want an admitted statement order", got)
	}
	if !slices.ContainsFunc(ctx.Choices(), func(choice ChoicePoint) bool {
		return choice.Kind == ChoiceStatementOrder
	}) {
		t.Fatalf("compiled cycle did not fall back to ordered execution: %v", ctx.Choices())
	}
}

func testAtomicBodyOrderNestedLists(t *testing.T) {
	source := `package test {
		private import ScalarValues::*;
		calc def Nested {
			return : Integer;
			attribute y : Integer := 1;
			if true {
				assign y := y * 10;
				assign y := y + 2;
			}
			attribute i : Integer := 0;
			while i < 1 {
				assign y := y * 10;
				assign y := y + 2;
				assign i := i + 1;
			}
			y
		}
		action def Use {
			out attribute r : Integer := 0;
			assign r := Nested();
		}
	}`
	m := parseLibraryModel(t, source)
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, mustPolicy(t, "seed:1"))
	outputs, err := ctx.ExecuteAction(m.action(t, "Use"))
	if err != nil {
		t.Fatal(err)
	}
	if got := outcomeValue(t, ctx.ActionOutcome(outputs), "r"); !slices.Contains([]string{"122", "140", "302", "320"}, got) {
		t.Fatalf("nested calc r = %s, want an ordering-dependent result", got)
	}
	choices := 0
	for _, choice := range ctx.Choices() {
		if choice.Kind == ChoiceStatementOrder {
			choices++
			if !strings.HasPrefix(choice.Where, resultOrderWherePrefix) ||
				!slices.Equal(choice.Alternatives, []string{"122", "140", "302", "320"}) {
				t.Fatalf("nested calc choice = %+v, want one result choice among [122 140 302 320]", choice)
			}
		}
	}
	if choices != 1 {
		t.Fatalf("nested calc recorded %d statement-order choices, want one result choice: %v", choices, ctx.Choices())
	}
}

func testAtomicBodyOrderFixedPolicies(t *testing.T) {
	m := conformanceModel(t, "calc_explore_statement_order")
	for _, policy := range []string{"default", "declared", "reverse"} {
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		if policy != "default" {
			mustSchedule(t, ctx, mustPolicy(t, policy))
		}
		outputs, err := ctx.ExecuteAction(m.action(t, "Use"))
		if err != nil {
			t.Fatalf("%s calc: %v", policy, err)
		}
		if got := outcomeValue(t, ctx.ActionOutcome(outputs), "r"); got != "12" {
			t.Errorf("%s calc r = %s, want declaration-order result 12", policy, got)
		}
	}

	source := `package test {
	private import ScalarValues::*;
		action def Ordered {
			out attribute r : Integer := 0;
			attribute y : Integer := 1;
			first start then step;
			action step {
				assign y := y * 10;
				assign y := y + 2;
			}
			then action finish { assign r := y; }
			then done;
		}
	}`
	actionModel := parseLibraryModel(t, source)
	for _, policy := range []string{"declared", "reverse"} {
		ctx, err := actionModel.fresh()
		if err != nil {
			t.Fatal(err)
		}
		mustSchedule(t, ctx, mustPolicy(t, policy))
		outputs, err := ctx.ExecuteAction(actionModel.action(t, "Ordered"))
		if err != nil {
			t.Fatalf("%s action: %v", policy, err)
		}
		if got := outcomeValue(t, ctx.ActionOutcome(outputs), "r"); got != "12" {
			t.Errorf("%s action r = %s, want declaration-order result 12", policy, got)
		}
	}
}

func testAtomicBodyOrderFixedPolicyBackwardThen(t *testing.T) {
	source := `package test {
		private import ScalarValues::*;
		calc def Backward {
			return : Integer;
			attribute y : Integer := 1;
			assign y := y + 2;
			assign y := y * 10;
			then y;
			y
		}
		action def Use {
			out attribute r : Integer := 0;
			assign r := Backward();
		}
	}`
	m := parseLibraryModel(t, source)
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	sym := namedOrFoundSymbol(
		t, m.idx, "test::Backward", m.idx.DocumentRoot(m.path), ast.DefCalc, ast.UsageCalc,
	)
	shape, err := ctx.calcShapeOf(sym)
	if err != nil {
		t.Fatal(err)
	}
	order, ok := shape.statementOrders.Load(&shape.Steps[0])
	if !ok || !order.(*lower.StatementOrder).HasReversePrecedence() {
		t.Fatal("calc body did not retain the backward succession precedence")
	}
	for _, policy := range []string{"declared", "reverse"} {
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		mustSchedule(t, ctx, mustPolicy(t, policy))
		outputs, err := ctx.ExecuteAction(m.action(t, "Use"))
		if err != nil {
			t.Fatalf("%s calc: %v", policy, err)
		}
		if got := outcomeValue(t, ctx.ActionOutcome(outputs), "r"); got != "30" {
			t.Errorf("%s calc r = %s, want the fixed stable order result 30", policy, got)
		}
	}
}

func testAtomicBodyOrderCalcDoesNotYield(t *testing.T) {
	source := `package test {
		private import ScalarValues::*;
		calc def Ord {
			return : Integer;
			attribute y : Integer := 1;
			assign y := y * 10;
			assign y := y + 2;
			y
		}
		action def Use {
			out attribute r : Integer := 0;
			attribute marker : Integer := 0;
			first start;
			fork split;
			action compute { assign r := Ord(); }
			action observe { assign marker := marker + 1; }
			join sync;
			done;
			succession first start then split;
			succession first split then compute;
			succession first split then observe;
			succession first compute then sync;
			succession first observe then sync;
			succession first sync then done;
		}
	}`
	m := parseLibraryModel(t, source)
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, mustPolicy(t, "seed:1"))
	trace := NewTraceRecorder()
	ctx.SetTrace(trace)
	outputs, err := ctx.ExecuteAction(m.action(t, "Use"))
	if err != nil {
		t.Fatal(err)
	}
	if got := outcomeValue(t, ctx.ActionOutcome(outputs), "r"); !slices.Contains([]string{"12", "30"}, got) {
		t.Fatalf("calc result = %s, want an admitted statement order", got)
	}
	record := trace.String()
	enter := strings.Index(record, "enter calc test::Ord")
	exit := strings.Index(record, "exit calc test::Ord")
	interleaved := strings.Index(record, "stmt assign marker")
	if enter < 0 || exit < enter || interleaved > enter && interleaved < exit {
		t.Fatalf("calc statements were divided by the yielding action body:\n%s", record)
	}
}

func testAtomicBodyOrderCheckerSweepCap(t *testing.T) {
	source := `package test {
		private import ScalarValues::*;
		calc def Many {
			return : Boolean;
			attribute a : Integer := 0;
			assign a := a + 1;
			assign a := a + 1;
			assign a := a + 1;
			assign a := a + 1;
			assign a := a + 1;
			assign a := a + 1;
			assign a := a + 1;
			true
		}
		action def A { out attribute r : Boolean := false; assign r := false; }
	}`
	m := parseLibraryModel(t, source)
	expr, ok := parser.ParseOneExpression(m.path, "test::Many()")
	if !ok {
		t.Fatal("parse calc invocation")
	}
	scope := m.idx.DocumentRoot(m.path)
	prop := CheckProperty{
		Name: "many",
		Holds: func(ctx *Context, _ *Invocation) (bool, error) {
			_, err := ctx.EvalWithScope(expr, scope)
			return err == nil, err
		},
	}
	report := checkStart(t, m, starterOf(m.action(t, "A")), CheckOptions{}, prop)
	if report.Verdict == CheckExhaustive || !slices.Contains(report.BoundsHit, BoundStatementOrders) {
		t.Fatalf("check %s with violations %v, want a non-exhaustive statement-order bound", report.Status(), report.Violations)
	}
}

func testAtomicBodyOrderExploreSweepCap(t *testing.T) {
	source := `package test {
		private import ScalarValues::*;
		calc def Many {
			return : Boolean;
			attribute a : Integer := 0;
			assign a := a + 1;
			assign a := a + 1;
			assign a := a + 1;
			assign a := a + 1;
			assign a := a + 1;
			assign a := a + 1;
			assign a := a + 1;
			true
		}
		action def A { out attribute r : Boolean := false; assign r := Many(); }
	}`
	m := parseLibraryModel(t, source)
	x := m.exploreAction(t, "explore", "A")
	if x.Complete() || !slices.Contains(x.BudgetsHit, BoundStatementOrders) {
		t.Fatalf("exploration %s, want an incomplete statement-orders bound", x.Status())
	}
	if !strings.Contains(x.Status(), "statement orders budget 1024") {
		t.Fatalf("status %q, want the statement-orders limit", x.Status())
	}
}

func testAtomicBodyOrderRecursiveResultChoices(t *testing.T) {
	for _, name := range []string{
		"calc_explore_statement_order",
		"calc_explore_statement_order_recursive",
		"calc_explore_statement_order_recursive_deep",
	} {
		t.Run(name, func(t *testing.T) {
			m := conformanceModel(t, name)
			x := m.exploreAction(t, "explore", "Use")
			if !x.Complete() || x.Runs != 2 || len(x.Outcomes) != 2 {
				t.Fatalf("exploration %s after %d runs reached %v, want two complete runs",
					x.Status(), x.Runs, outcomeTexts(x))
			}
			got := featureValues(t, x, "r")
			slices.Sort(got)
			want := []string{"12", "30"}
			if strings.Contains(name, "recursive") {
				want = []string{"2", "20"}
			}
			if !slices.Equal(got, want) {
				t.Fatalf("results = %v, want %v", got, want)
			}
			for _, outcome := range x.Outcomes {
				if len(outcome.Witness) != 1 || outcome.Witness[0].Kind != ChoiceStatementOrder ||
					!slices.Equal(outcome.Witness[0].Among, want) {
					t.Fatalf("witness = %v, want one result choice among %v", outcome.Witness, want)
				}
				recorded, err := ParseChoices(FormatChoices(outcome.Witness))
				if err != nil {
					t.Fatalf("parse result witness: %v", err)
				}
				replayed, taken, err := replayed(t, m.fresh, caseRun(t, m, "Use"), recorded)
				if err != nil {
					t.Fatalf("replay result witness: %v", err)
				}
				if got := outcomeValue(t, replayed, "r"); got != outcomeValue(t, outcome.Outcome, "r") {
					t.Fatalf("replay returned r = %s, want %s", got, outcomeValue(t, outcome.Outcome, "r"))
				}
				if FormatChoices(taken) != FormatChoices(outcome.Witness) {
					t.Fatalf("replay took %q, want %q", FormatChoices(taken), FormatChoices(outcome.Witness))
				}
			}
		})
	}

	m := conformanceModel(t, "calc_explore_statement_order")
	x := m.exploreAction(t, "explore", "Use")
	if len(x.Outcomes) == 0 || len(x.Outcomes[0].Witness) != 1 {
		t.Fatalf("exploration witness = %v, want a result choice", x.Outcomes)
	}
	bad := slices.Clone(x.Outcomes[0].Witness)
	bad[0].Among = []string{"12", "unreachable"}
	bad[0].Took = "unreachable"
	_, _, err := replayed(t, m.fresh, caseRun(t, m, "Use"), bad)
	if !errors.Is(err, ErrReplayRefused) {
		t.Fatalf("unproduced result replay error = %v, want ErrReplayRefused", err)
	}
}

func testAtomicBodyOrderInvocationErrorResults(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		calc def MayFail {
			return : Real;
			attribute y : Real := 1.0;
			assign y := 10.0 / y;
			assign y := 0.0;
			y
		}
		action def Use {
			out attribute r : Real := 0.0;
			assign r := MayFail();
		}
	}`)
	x := m.exploreAction(t, "explore", "Use")
	if !x.Complete() || x.Runs != 2 || len(x.Outcomes) != 2 {
		t.Fatalf("exploration %s after %d runs: %v, want value and error results",
			x.Status(), x.Runs, outcomeTexts(x))
	}
	value, failed := false, false
	for _, outcome := range x.Outcomes {
		if len(outcome.Witness) != 1 {
			t.Fatalf("invocation witness = %v, want one result choice", outcome.Witness)
		}
		choice := outcome.Witness[0]
		if choice.Kind != ChoiceStatementOrder || len(choice.Among) != 2 ||
			choice.Among[0] != "0.0" || !strings.HasPrefix(choice.Among[1], "error:") {
			t.Fatalf("invocation choice = %+v, want value followed by its distinct error", choice)
		}
		if outcome.Outcome.Err != nil {
			failed = errors.Is(outcome.Outcome.Err, ErrDivisionByZero)
			if !failed {
				t.Fatalf("error result = %v, want ErrDivisionByZero", outcome.Outcome.Err)
			}
			continue
		}
		value = outcomeValue(t, outcome.Outcome, "r") == "0.0"
	}
	if !value || !failed {
		t.Fatalf("invocation results have value=%t, division error=%t; want both", value, failed)
	}
}

func testAtomicBodyOrderEnclosingCalcMemo(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		calc def Wrapper { return : Integer = 0; }
		action def Use {
			attribute local : Integer := 1;
			calc def Read {
				in n : Integer;
				return : Integer;
				attribute y : Integer := local;
				assign y := y * 10;
				assign y := y + n;
				y
			}
		}
	}`)
	wrapper := namedOrFoundSymbol(t, m.idx, "test::Wrapper", m.idx.DocumentRoot(m.path), ast.DefCalc, ast.UsageCalc)
	reader := namedOrFoundSymbol(t, m.idx, "test::Use::Read", m.idx.DocumentRoot(m.path), ast.DefCalc, ast.UsageCalc)
	policy := mustPolicy(t, "explore")
	exploration, err := ExploreWith(context.Background(), policy, 1, func(int) (*Context, error) {
		return m.fresh()
	}, func(ctx *Context) (Outcome, error) {
		wrapperShape, err := ctx.calcShapeOf(wrapper)
		if err != nil {
			return Outcome{}, err
		}
		readerShape, err := ctx.calcShapeOf(reader)
		if err != nil {
			return Outcome{}, err
		}
		scope := m.idx.DocumentRoot(m.path)
		value, err := ctx.invokeWithStatementOrderResults(wrapperShape, calcArgs{}, nil, true, func() (Value, error) {
			locals := map[string]Value{"local": constInt(1)}
			enclosing := []frame{mapFrame(locals)}
			args := calcArgs{positional: []Value{constInt(2)}}
			if _, err := ctx.invokeCalcShapeIn(readerShape, args, scope, nil, enclosing); err != nil {
				return Value{}, err
			}
			locals["local"] = constInt(2)
			return ctx.invokeCalcShapeIn(readerShape, args, scope, nil, enclosing)
		})
		if err != nil {
			return Outcome{}, err
		}
		return Outcome{Outputs: map[string]Value{"r": value}}, nil
	})
	if err != nil {
		t.Fatalf("explore enclosing calc calls: %v", err)
	}
	if !exploration.Complete() || exploration.Runs != 2 || len(exploration.Outcomes) != 2 {
		t.Fatalf("exploration %s after %d runs reached %v, want two complete results",
			exploration.Status(), exploration.Runs, outcomeTexts(exploration))
	}
	got := featureValues(t, exploration, "r")
	slices.Sort(got)
	if want := []string{"22", "40"}; !slices.Equal(got, want) {
		t.Fatalf("results = %v, want %v from the changed enclosing local", got, want)
	}
}

func testAtomicBodyOrderTransitiveGuards(t *testing.T) {
	source := `package test {
		private import ScalarValues::*;
		calc def Ord {
			return : Boolean;
			attribute y : Integer := 1;
			assign y := y * 10;
			assign y := y + 2;
			y == 12
		}
		calc def Wrapper { return : Boolean; Ord() }
		constraint def UsesCalc { Ord() }
		action def WrapperGuard {
			out attribute r : Integer := 0;
			first start then choose;
			decide choose;
			if Wrapper() then yes; else no;
			action yes { assign r := 1; } then done;
			action no { assign r := 2; } then done;
		}
		action def ConstraintGuard {
			out attribute r : Integer := 0;
			first start then choose;
			decide choose;
			if UsesCalc() then yes; else no;
			action yes { assign r := 1; } then done;
			action no { assign r := 2; } then done;
		}
	}`
	m := parseLibraryModel(t, source)
	for _, name := range []string{"WrapperGuard", "ConstraintGuard"} {
		t.Run(name, func(t *testing.T) {
			x := m.exploreAction(t, "explore", name)
			if !x.Complete() || x.Runs != 2 {
				t.Fatalf("exploration %s after %d runs: %v, want both guard results",
					x.Status(), x.Runs, outcomeTexts(x))
			}
			got := featureValues(t, x, "r")
			slices.Sort(got)
			if !slices.Equal(got, []string{"1", "2"}) {
				t.Fatalf("guard outcomes = %v, want both branches", got)
			}
		})
	}
}

func testAtomicBodyOrderConstraintInvocationChoice(t *testing.T) {
	m := conformanceModel(t, "constraint_explore_statement_order")
	x := m.exploreAction(t, "explore", "A")
	if !x.Complete() || x.Runs != 2 || len(x.Outcomes) != 2 {
		t.Fatalf("constraint exploration %s after %d runs: %v, want two complete results",
			x.Status(), x.Runs, outcomeTexts(x))
	}
	for _, outcome := range x.Outcomes {
		if len(outcome.Witness) != 1 {
			t.Fatalf("constraint witness = %v, want one result-level choice", outcome.Witness)
		}
		choice := outcome.Witness[0]
		if choice.Kind != ChoiceStatementOrder || choice.Step != 1 ||
			!strings.Contains(choice.Where, "result of constraint test::Ok") ||
			!slices.Equal(choice.Among, []string{"true", "false"}) {
			t.Fatalf("constraint choice = %+v, want result values attributed to step 1", choice)
		}
	}
}

func testAtomicBodyOrderNestedPreviewInvocation(t *testing.T) {
	m := conformanceModel(t, "calc_explore_statement_order")
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, mustPolicy(t, "seed:1"))
	expr, ok := parser.ParseOneExpression(m.path, "test::Ord()")
	if !ok {
		t.Fatal("parse calc invocation")
	}
	restore := ctx.beginProbe()
	defer restore()
	_, err = ctx.EvalWithScope(expr, m.idx.DocumentRoot(m.path))
	if !errors.Is(err, ErrOrderDependentPreview) {
		t.Fatalf("invocation error = %v, want ErrOrderDependentPreview", err)
	}
	if !strings.Contains(err.Error(), "Ord") || !strings.Contains(err.Error(), "statement orders") ||
		strings.Contains(err.Error(), "*ast.") {
		t.Fatalf("preview error %q does not name the invocation and body", err)
	}
}

func testAtomicBodyOrderCalcFlowRefusal(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		calc def Bad { return : Integer; first start; 1 }
	}`)
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	expr, ok := parser.ParseOneExpression(m.path, "test::Bad()")
	if !ok {
		t.Fatal("parse calc invocation")
	}
	_, err = ctx.EvalWithScope(expr, m.idx.DocumentRoot(m.path))
	if !errors.Is(err, ErrStatementNotExecutable) {
		t.Fatalf("error = %v, want ErrStatementNotExecutable", err)
	}
	if !strings.Contains(err.Error(), "`first` statement") || strings.Contains(err.Error(), "*ast.") {
		t.Fatalf("error %q does not name the unsupported construct", err)
	}
}

func testAtomicBodyOrderCalcUnmatchedSuccession(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		calc def Bad {
			return : Integer;
			attribute x : Integer := 1;
			succession first missing then x;
			x
		}
	}`)
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	expr, ok := parser.ParseOneExpression(m.path, "test::Bad()")
	if !ok {
		t.Fatal("parse calc invocation")
	}
	value, err := ctx.EvalWithScope(expr, m.idx.DocumentRoot(m.path))
	if err != nil {
		t.Fatalf("evaluation error = %v, want unmatched succession ignored", err)
	}
	if value.Const.Int != 1 {
		t.Fatalf("Bad() = %s, want 1", FormatTraceValue(value))
	}
}

func testAtomicBodyOrderConstraintViolation(t *testing.T) {
	source := `package test {
		private import ScalarValues::*;
		constraint def Ok {
			attribute y : Integer := 1;
			assign y := y * 10;
			assign y := y + 2;
			y == 12
		}
	action def A { out attribute r : Boolean := false; assign r := false; }
	}`
	m := parseLibraryModel(t, source)
	constraint := namedOrFoundSymbol(
		t, m.idx, "test::Ok", m.idx.DocumentRoot(m.path), ast.DefConstraint, ast.UsageConstraint,
	)
	prop := CheckProperty{
		Name: "ok",
		Holds: func(ctx *Context, _ *Invocation) (bool, error) {
			holds, err := ctx.EvaluateConstraint(constraint, constraint.OwnerScope)
			if errors.Is(err, ErrViolated) {
				return false, nil
			}
			return holds, err
		},
	}
	report := checkStart(t, m, starterOf(m.action(t, "A")), CheckOptions{}, prop)
	if report.Verdict != CheckViolation || !slices.ContainsFunc(report.Violations, func(v Violation) bool {
		return v.Kind == ViolationProperty && v.Name == "ok"
	}) {
		t.Fatalf("check %s with violations %v, want a violation of ok", report.Status(), report.Violations)
	}
}

func testAtomicBodyOrderDerivedFinalReplay(t *testing.T) {
	m := conformanceModel(t, "calc_explore_statement_order_derived")
	report := checkModel(t, m, "Use", CheckBudget{}, CheckOptions{Diverge: []string{"r"}})
	if report.Verdict != CheckDivergent {
		t.Fatalf("check %s, want divergent r outcomes", report.Status())
	}
	seen := map[string]bool{}
	for _, final := range report.Finals {
		value := final.Values["r"]
		seen[value] = true
		replayed := replayWitness(t, m, starterOf(m.action(t, "Use")), final.Witness, final.Outcome)
		got, err := replayed.FinalValue("r")
		if err != nil {
			t.Errorf("replay final value: %v", err)
			continue
		}
		if got != value {
			t.Errorf("replayed r = %s, want final variant %s", got, value)
		}
	}
	if !seen["12"] || !seen["30"] || len(seen) != 2 {
		t.Fatalf("check outcomes %v, want r = 12 and 30", seen)
	}
}

func testAtomicBodyOrderNestedPreviewGuard(t *testing.T) {
	m := conformanceModel(t, "action_guard_statement_order")
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, mustPolicy(t, "seed:1"))
	guard, ok := parser.ParseOneExpression(m.path, "test::Ok()")
	if !ok {
		t.Fatal("parse guard")
	}
	restore := ctx.beginProbe()
	defer restore()
	_, err = ctx.guardUnderStatementOrders(guard, 1, m.idx.DocumentRoot(m.path), func() (bool, error) {
		value, err := ctx.EvalWithScope(guard, m.idx.DocumentRoot(m.path))
		if err != nil {
			return false, err
		}
		if value.Kind != ValConst || value.Const.Kind != semantics.ValBool {
			return false, ErrTypeMismatch
		}
		return value.Const.Bool, nil
	})
	if !errors.Is(err, ErrOrderDependentPreview) {
		t.Fatalf("guard error = %v, want ErrOrderDependentPreview", err)
	}
	if !strings.Contains(err.Error(), "Ok") || strings.Contains(err.Error(), "*ast.") {
		t.Fatalf("preview error %q does not name the guard and body", err)
	}
}

func testAtomicBodyOrderGuardExternalWrite(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		attribute outside : Integer := 0;
		constraint def Guard {
			attribute y : Integer := 1;
			if false { assign outside := outside + 1; }
			assign y := y * 10;
			assign y := y + 2;
			y == 12
		}
	}`)
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, mustPolicy(t, "seed:1"))
	guard, ok := parser.ParseOneExpression(m.path, "test::Guard()")
	if !ok {
		t.Fatal("parse guard")
	}
	_, err = ctx.guardUnderStatementOrders(guard, 1, m.idx.DocumentRoot(m.path), func() (bool, error) {
		value, err := ctx.EvalWithScope(guard, m.idx.DocumentRoot(m.path))
		if err != nil {
			return false, err
		}
		if value.Kind != ValConst || value.Const.Kind != semantics.ValBool {
			return false, ErrTypeMismatch
		}
		return value.Const.Bool, nil
	})
	if !errors.Is(err, ErrOrderDependentGuardEffect) {
		t.Fatalf("guard error = %v, want ErrOrderDependentGuardEffect", err)
	}
	if !strings.Contains(err.Error(), "Guard") || strings.Contains(err.Error(), "*ast.") {
		t.Fatalf("guard refusal %q does not identify the guard body", err)
	}
}

func testAtomicBodyOrderGuardErrorOutcome(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		constraint def Mixed {
			attribute d : Integer := 0;
			attribute x : Real := 0.0;
			assign d := 1;
			assign x := 1 / d;
			x == 1.0
		}
		action def Use {
			out attribute r : Boolean := false;
			first start then choose;
			decide choose;
			if Mixed() then yes;
			else no;
			action yes { assign r := true; }
			then done;
			action no { assign r := false; }
			then done;
		}
	}`)
	x := m.exploreAction(t, "explore", "Use")
	holds, fails, hasGuardOrder, errorTaken, hasDoesNotHold := false, false, false, false, false
	for _, outcome := range x.Outcomes {
		for _, choice := range outcome.Witness {
			if choice.Kind != ChoiceGuardOrder {
				continue
			}
			hasGuardOrder = true
			if len(choice.Among) != 2 || choice.Among[0] != "holds" || !strings.HasPrefix(choice.Among[1], "error:") {
				t.Errorf("guard alternatives %v, want holds followed by the distinct error", choice.Among)
			}
			errorTaken = errorTaken || strings.HasPrefix(choice.Took, "error:")
			hasDoesNotHold = hasDoesNotHold || slices.Contains(choice.Among, "does not hold")
		}
		if outcome.Outcome.Err != nil {
			fails = true
			if !strings.Contains(outcome.Outcome.Err.Error(), "division") {
				t.Errorf("guard error = %v, want a division error", outcome.Outcome.Err)
			}
			continue
		}
		if got := outcomeValue(t, outcome.Outcome, "r"); got == "true" {
			holds = true
		}
	}
	if !holds || !fails || !hasGuardOrder || !errorTaken || hasDoesNotHold {
		t.Fatalf("exploration %s has holds=%t, error=%t, guard choice=%t, error taken=%t, false result=%t; want holds and error as distinct results",
			x.Status(), holds, fails, hasGuardOrder, errorTaken, hasDoesNotHold)
	}
}

func testAtomicBodyOrderFalseGuardReplay(t *testing.T) {
	m := conformanceModel(t, "action_guard_statement_order")
	report := checkModel(t, m, "DecisionGuard", CheckBudget{}, CheckOptions{Diverge: []string{"r"}})
	for _, final := range report.Finals {
		if final.Values["r"] != "2" {
			continue
		}
		if !slices.ContainsFunc(final.Witness.Choices, func(choice ChoiceTaken) bool {
			return choice.Kind == ChoiceGuardOrder && choice.Took == "does not hold"
		}) {
			t.Fatalf("false-guard final witness lacks its guard choice: %s", final.Witness)
		}
		replayed := replayWitness(t, m, starterOf(m.action(t, "DecisionGuard")), final.Witness, final.Outcome)
		if got := outcomeValue(t, replayed.Inv.Outcome(), "r"); got != "2" {
			t.Fatalf("replayed false-guard outcome r = %s, want 2", got)
		}
		return
	}
	t.Fatalf("check %s has no r = 2 final", report.Status())
}

func testAtomicBodyOrderRecursiveBudget(t *testing.T) {
	source := `package test {
		private import ScalarValues::*;
		calc def Recur {
			return : Integer;
			attribute y : Integer := 1;
			assign y := y * 10;
			assign y := y + 2;
			Recur()
		}
	}`
	m := parseLibraryModel(t, source)
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, mustPolicy(t, "seed:1"))
	budgets := ctx.Budgets()
	budgets.MaxCalcDepth = 16
	if err := ctx.SetBudgets(budgets); err != nil {
		t.Fatal(err)
	}
	ctx.SetTrace(NewTraceRecorder())
	expr, ok := parser.ParseOneExpression(m.path, "test::Recur()")
	if !ok {
		t.Fatal("parse recursive calc invocation")
	}
	_, err = ctx.EvalWithScope(expr, m.idx.DocumentRoot(m.path))
	if !errors.Is(err, ErrCalcRecursionLimit) {
		t.Fatalf("error = %v, want ErrCalcRecursionLimit", err)
	}
	if slices.ContainsFunc(ctx.ChoicesTaken(), func(choice ChoiceTaken) bool {
		return choice.Kind == ChoiceStatementOrder
	}) {
		t.Fatalf("recursive invocation has one distinct error result, but recorded statement-order choices: %s", ctx.Trace())
	}
}
