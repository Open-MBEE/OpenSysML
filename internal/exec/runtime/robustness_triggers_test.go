package runtime

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessTriggers covers time and change triggers: non-numeric or wrongly
// dimensioned times, non-boolean change conditions and clock advance.
func TestRuntimeRobustnessTriggers(t *testing.T) {
	t.Run("action_accept_time_waits", testActionAcceptTimeWaits)
	t.Run("clock_advance", testClockAdvance)
	t.Run("action_accept_non_boolean_change_trigger", testActionAcceptNonBooleanChangeTrigger)
	t.Run("non_numeric_time_trigger", testNonNumericTimeTrigger)
	t.Run("time_trigger_of_a_non_time_dimension", testTimeTriggerOfANonTimeDimension)
	t.Run("time_trigger_of_the_type_validation_refuses", testTimeTriggerOfTheTypeValidationRefuses)
	t.Run("action_return_parameter_validation_refuses", testActionReturnParameterValidationRefuses)
	t.Run("change_condition_that_never_holds", testChangeConditionThatNeverHolds)
}

// testNonNumericTimeTrigger: a timed trigger whose duration is not a number
// cannot be scheduled and must be reported rather than silently dropped, even
// with no library loaded for the static judgement to name a type from.
func testNonNumericTimeTrigger(t *testing.T) {
	_, _, err := executeStateSource(t, "Machine", `package test {
		state Machine {
			entry; then init;
			state init;
			state waiting;
			accept at "noon" then done;
			succession first init then waiting;
		}
	}`)
	if err == nil {
		t.Fatal("expected an error for a non-numeric time trigger")
	}
	if !strings.Contains(err.Error(), "time duration must be constant, got string") {
		t.Errorf("expected a numeric-duration error, got: %v", err)
	}
}

// testTimeTriggerOfANonTimeDimension: a duration whose unit measures something
// other than time, held by a feature whose type does not resolve so only its
// value can tell, cannot be scheduled and fails as the typed error it is.
func testTimeTriggerOfANonTimeDimension(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			private import SI::*;
			state Machine {
				attribute load : Nowhere::Mass = 5 [kg];
				entry; then init;
				state init;
				state waiting;
				accept after load then done;
				state done;
				succession first init then waiting;
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
	if sym == nil {
		t.Fatal("state Machine not found")
	}

	_, err := ctx.ExecuteState(sym)
	if !errors.Is(err, ErrIncommensurableUnits) {
		t.Fatalf("err = %v; want ErrIncommensurableUnits", err)
	}
	if !strings.Contains(err.Error(), "5 [kg] is not a time") {
		t.Errorf("err = %v; want it to name the quantity", err)
	}
}

// testTimeTriggerOfTheTypeValidationRefuses: an argument validation refuses is
// refused as one typed error before it is evaluated or converted, whatever
// evaluating it would have said.
func testTimeTriggerOfTheTypeValidationRefuses(t *testing.T) {
	for _, tc := range []struct{ name, trigger string }{
		{"unitless after", "after 5"},
		{"duration at", "at 2 [min]"},
		{"mass after", "after 5 [kg]"},
		{"string at", `at "noon"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
				package test {
					private import SI::*;
					state Machine {
						entry; then init;
						state init;
						state waiting;
						accept `+tc.trigger+` then done;
						state done;
						succession first init then waiting;
					}
				}
			`))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "Machine", ast.DefState)
			if sym == nil {
				t.Fatal("state Machine not found")
			}

			_, err := ctx.ExecuteState(sym)
			if !errors.Is(err, errTimeTriggerType) {
				t.Fatalf("err = %v; want errTimeTriggerType", err)
			}
			if !strings.Contains(err.Error(), "`"+tc.trigger+"`") {
				t.Errorf("err = %v; want it to quote the trigger as written", err)
			}
		})
	}
}

// testActionReturnParameterValidationRefuses: an action declaring a `return`
// parameter — which validation refuses, only a function or expression owning
// one — is refused at initialize with a typed error naming the parameter,
// whether the action, an action it specializes, or a node of its flow declares it.
func testActionReturnParameterValidationRefuses(t *testing.T) {
	for _, tc := range []struct{ name, src, want string }{
		{"own", `
			action def Run {
				return total : Integer;
				first start;
				then action go { assign total := 1; }
				then done;
			}`, "action Run declares `return total`; write `out total`"},
		{"inherited", `
			action def Base { return total : Integer; }
			action def Run :> Base {
				first start;
				then action go { assign total := 1; }
				then done;
			}`, "action Run declares `return total`; write `out total`"},
		{"node", `
			action def Run {
				out total : Integer;
				first start;
				then action go { return partial : Integer; assign total := 1; }
				then done;
			}`, "action node go declares `return partial`; write `out partial`"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `
				package test {
					private import ScalarValues::*;
					`+tc.src+`
				}
			`))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "Run", ast.DefAction)
			if sym == nil {
				t.Fatal("action Run not found")
			}

			_, err := ctx.ExecuteAction(sym)
			if !errors.Is(err, errActionResultParameter) {
				t.Fatalf("err = %v; want errActionResultParameter", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v; want it to contain %q", err, tc.want)
			}
		})
	}
}

// testChangeConditionThatNeverHolds: a machine whose only outgoing transition
// watches a false condition suspends within its budget and says what it waits
// on, rather than hanging or reporting silent completion.
func testChangeConditionThatNeverHolds(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", `package test {
		state Machine {
			attribute ready : Boolean = false;
			entry; then init;
			state init;
			state waiting;
			accept when ready then done;
			state done;
			succession first init then waiting;
		}
	}`)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	if exec.State() != StateSuspended {
		t.Fatalf("state = %v; want suspended", exec.State())
	}
	if reason := exec.SuspendReason(); !strings.Contains(reason, "when ready") ||
		!strings.Contains(reason, "condition is false") {
		t.Errorf("reason = %q; want the false condition it waits on", reason)
	}
}

// testActionAcceptTimeWaits: an action's `accept after`/`accept at` waits on the
// context's clock; a past instant fires at once, bad arguments are typed errors.
func testActionAcceptTimeWaits(t *testing.T) {
	run := func(t *testing.T, trigger string) (map[string]Value, *Context, error) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
			package test {
				private import SI::*;
				private import ISQ::*;
				private import Time::*;
				action maintain {
					attribute maintenanceTime : TimeInstantValue = 3 [s];
					attribute past : TimeInstantValue = 0 [s];
					attribute wrongWay : DurationValue = -2 [s];
					attribute load : MassValue = 5 [kg];
					attribute done : Integer = 0;
					first start;
					action waitForIt `+trigger+`;
					action work { assign done := 1; }
					done;
					succession first start then waitForIt;
					succession first waitForIt then work;
					succession first work then done;
				}
			}
		`))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "maintain", ast.DefAction)
		if sym == nil {
			t.Fatal("action maintain not found")
		}
		results, err := ctx.ExecuteAction(sym)
		return results, ctx, err
	}
	fires := func(t *testing.T, trigger string, wantNow float64) {
		results, ctx, err := run(t, trigger)
		if err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		if got := results["done"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("done = %v; want 1: the accept fired", got)
		}
		if got := ctx.Clock().Now(); got != wantNow {
			t.Errorf("clock = %v after the run; want %v", got, wantNow)
		}
		if waits := ctx.Clock().Waits(); len(waits) != 0 {
			t.Errorf("%d wait(s) left on the clock after the run; want none", len(waits))
		}
	}
	t.Run("after advances the clock", func(t *testing.T) { fires(t, "accept after 5 [s]", 5) })
	t.Run("at advances the clock", func(t *testing.T) { fires(t, "accept at maintenanceTime", 3) })
	t.Run("at an instant already past fires at once", func(t *testing.T) { fires(t, "accept at past", 0) })
	t.Run("negative after", func(t *testing.T) {
		_, _, err := run(t, "accept after wrongWay")
		if !errors.Is(err, ErrNegativeDuration) {
			t.Fatalf("err = %v; want ErrNegativeDuration", err)
		}
	})
	t.Run("non-time dimension", func(t *testing.T) {
		_, _, err := run(t, "accept after load")
		if !errors.Is(err, errTimeTriggerType) {
			t.Fatalf("err = %v; want errTimeTriggerType", err)
		}
		if !strings.Contains(err.Error(), "`after load` must be a ISQBase::DurationValue, found MassValue") {
			t.Errorf("err = %v; want it to name the type found", err)
		}
	})
}

// testClockAdvance: advancing the context's clock is bounded and total — zero,
// nothing waiting, a wait beyond the advance, a negative advance, a budget stop.
func testClockAdvance(t *testing.T) {
	newRun := func(t *testing.T) (*Context, *ActionExecutor, *symbols.Scope) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
			package test {
				private import SI::*;
				private import ScalarValues::*;
				action patient {
					attribute done : Integer = 0;
					first start;
					then action wait accept after 8 [s];
					then action work assign done := 1;
					then done;
				}
				action tardy {
					attribute two : Time::TimeInstantValue = 2 [s];
					attribute done : Integer = 0;
					first start;
					then action wait accept at two;
					then action work assign done := 1;
					then done;
				}
				state metronome {
					attribute beats : Integer = 0;
					entry; then ticking;
					state ticking;
					transition ticking then ticking accept after 1 [s] do assign beats := beats + 1;
				}
			}
		`))
		root := idx.DocumentRoot("<test>")
		action := findSymbolByName(root, "patient", ast.DefAction)
		machine := findSymbolByName(root, "metronome", ast.DefState)
		if action == nil || machine == nil {
			t.Fatal("behaviors not found")
		}
		exec, err := ctx.CreateActionExecutor(action)
		if err != nil {
			t.Fatalf("CreateActionExecutor: %v", err)
		}
		if err := exec.Step(); err != nil {
			t.Fatalf("Step: %v", err)
		}
		return ctx, exec, root
	}
	t.Run("zero", func(t *testing.T) {
		ctx, exec, _ := newRun(t)
		report, err := ctx.Advance(0)
		if err != nil {
			t.Fatalf("Advance(0): %v", err)
		}
		if report.From != 0 || report.To != 0 || report.Steps != 0 || report.Events != 0 {
			t.Errorf("report = %+v; want nothing moved", report)
		}
		if got := len(ctx.Clock().Waits()); got != 1 {
			t.Errorf("%d wait(s) on the clock; want the action's one, still queued", got)
		}
		if exec.State() == StateCompleted {
			t.Error("the action completed without the clock moving")
		}
	})
	t.Run("nothing waiting", func(t *testing.T) {
		_, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `package test { part def P; }`))
		report, err := ctx.Advance(12.5)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if report.To != 12.5 || ctx.Clock().Now() != 12.5 {
			t.Errorf("clock at %v, report %+v; want 12.5 with nothing run", ctx.Clock().Now(), report)
		}
	})
	t.Run("wait beyond the advance stays queued", func(t *testing.T) {
		ctx, exec, _ := newRun(t)
		if _, err := ctx.Advance(5); err != nil {
			t.Fatalf("Advance(5): %v", err)
		}
		if got := ctx.Clock().Now(); got != 5 {
			t.Errorf("clock = %v; want 5", got)
		}
		waits := ctx.Clock().Waits()
		if len(waits) != 1 || waits[0].Due != 8 {
			t.Fatalf("waits = %+v; want the action's wait still due at 8", waits)
		}
		if err := exec.Step(); !errors.Is(err, ErrNothingDue) {
			t.Errorf("Step at t=5 = %v; want ErrNothingDue, the token waits on the clock", err)
		}
		if _, err := ctx.Advance(3); err != nil {
			t.Fatalf("Advance(3): %v", err)
		}
		if got := exec.Results()["done"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("done = %v after the clock reached 8; want 1", got)
		}
		if got := len(ctx.Clock().Waits()); got != 0 {
			t.Errorf("%d wait(s) left after the action completed; want none", got)
		}
	})
	t.Run("at an instant the clock has passed fires at once", func(t *testing.T) {
		ctx, _, root := newRun(t)
		if _, err := ctx.Advance(4); err != nil {
			t.Fatalf("Advance(4): %v", err)
		}
		tardy := findSymbolByName(root, "tardy", ast.DefAction)
		if tardy == nil {
			t.Fatal("action tardy not found")
		}
		results, err := ctx.ExecuteAction(tardy)
		if err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		if got := results["done"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("done = %v; want 1: an instant already reached is not waited for", got)
		}
		if got := ctx.Clock().Now(); got != 4 {
			t.Errorf("clock = %v; want 4, never moved back to 2", got)
		}
	})
	t.Run("negative", func(t *testing.T) {
		ctx, _, _ := newRun(t)
		_, err := ctx.Advance(-1)
		if !errors.Is(err, ErrNegativeDuration) {
			t.Fatalf("Advance(-1) = %v; want ErrNegativeDuration", err)
		}
		if got := ctx.Clock().Now(); got != 0 {
			t.Errorf("clock moved to %v on a refused advance", got)
		}
	})
	t.Run("infinite", func(t *testing.T) {
		ctx, exec, _ := newRun(t)
		for _, duration := range []float64{math.Inf(1), math.Inf(-1), math.NaN()} {
			_, err := ctx.Advance(duration)
			if !errors.Is(err, ErrNegativeDuration) {
				t.Errorf("Advance(%v) = %v; want ErrNegativeDuration", duration, err)
			}
		}
		if got := ctx.Clock().Now(); got != 0 {
			t.Errorf("clock moved to %v on a refused advance", got)
		}
		if exec.State() == StateCompleted {
			t.Error("the action completed without the clock moving")
		}
	})
	t.Run("past the last instant the clock can hold", func(t *testing.T) {
		ctx, exec, _ := newRun(t)
		ctx.clock.now = math.MaxFloat64
		_, err := ctx.Advance(math.MaxFloat64)
		if !errors.Is(err, ErrNegativeDuration) {
			t.Fatalf("Advance(MaxFloat64) from MaxFloat64 = %v; want ErrNegativeDuration", err)
		}
		if got := ctx.Clock().Now(); got != math.MaxFloat64 {
			t.Errorf("clock = %v on a refused advance; want it left at MaxFloat64", got)
		}
		if exec.State() == StateCompleted {
			t.Error("the action completed on a refused advance")
		}
	})
	t.Run("a wait past the last instant the clock can hold", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
			package test {
				private import SI::*;
				action eon {
					first start;
					then action wait accept after 1.0e308 [s];
					then done;
				}
			}
		`))
		action := findSymbolByName(idx.DocumentRoot("<test>"), "eon", ast.DefAction)
		if action == nil {
			t.Fatal("action eon not found")
		}
		ctx.clock.now = math.MaxFloat64
		_, err := ctx.ExecuteAction(action)
		if !errors.Is(err, ErrNegativeDuration) {
			t.Fatalf("ExecuteAction = %v; want ErrNegativeDuration for a wait the clock cannot reach", err)
		}
		if got := ctx.Clock().Now(); got != math.MaxFloat64 {
			t.Errorf("clock = %v; want it left where it was", got)
		}
		if got := len(ctx.Clock().Waits()); got != 0 {
			t.Errorf("%d wait(s) left on the clock by the refused accept; want none", got)
		}
	})
	t.Run("a nested flow's wait keeps to the advance", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, nestedWaitModel))
		action := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
		if action == nil {
			t.Fatal("action outer not found")
		}
		exec, err := ctx.CreateActionExecutor(action)
		if err != nil {
			t.Fatalf("CreateActionExecutor: %v", err)
		}
		if err := exec.RunToQuiescence(); err != nil {
			t.Fatalf("RunToQuiescence: %v", err)
		}
		if got := ctx.Clock().Now(); got != 0 {
			t.Fatalf("clock = %v after a run at the current instant; want 0", got)
		}
		if waits := ctx.Clock().Waits(); len(waits) != 1 || waits[0].Due != 5 {
			t.Fatalf("waits = %+v; want the nested flow's wait due at 5", waits)
		}
		if err := exec.Step(); !errors.Is(err, ErrNothingDue) {
			t.Errorf("Step = %v; want ErrNothingDue, the nested flow waits on the clock", err)
		}
		report, err := ctx.Advance(2)
		if err != nil {
			t.Fatalf("Advance(2): %v", err)
		}
		if got := ctx.Clock().Now(); got != 2 || report.To != 2 {
			t.Errorf("clock = %v, report %+v; want the advance to stop at 2, short of the nested wait", got, report)
		}
		if got := exec.Results()["n"]; got.Kind != ValConst || got.Const.Int != 0 {
			t.Errorf("n = %v at t=2; want 0, the nested flow has not gone on", got)
		}
		if exec.State() != StateWaiting {
			t.Errorf("state = %v at t=2; want Waiting", exec.State())
		}
		if _, err := ctx.Advance(3); err != nil {
			t.Fatalf("Advance(3): %v", err)
		}
		if got := exec.Results()["n"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("n = %v once the clock reached 5; want 1", got)
		}
		if exec.State() != StateCompleted || ctx.Clock().Now() != 5 {
			t.Errorf("state = %v, clock = %v; want Completed at 5", exec.State(), ctx.Clock().Now())
		}
	})
	t.Run("a nested flow's wait is advanced to by a run of its own", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, nestedWaitModel))
		action := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
		results, err := ctx.ExecuteAction(action)
		if err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		if got := results["n"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("n = %v; want 1", got)
		}
		if got := ctx.Clock().Now(); got != 5 {
			t.Errorf("clock = %v; want 5, where the nested wait came due", got)
		}
	})
	t.Run("a performed action's wait keeps to the advance", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, performedWaitModel))
		action := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
		if action == nil {
			t.Fatal("action outer not found")
		}
		exec, err := ctx.CreateActionExecutor(action)
		if err != nil {
			t.Fatalf("CreateActionExecutor: %v", err)
		}
		if err := exec.RunToQuiescence(); err != nil {
			t.Fatalf("RunToQuiescence: %v", err)
		}
		if _, err := ctx.Advance(2); err != nil {
			t.Fatalf("Advance(2): %v", err)
		}
		if got := ctx.Clock().Now(); got != 2 {
			t.Errorf("clock = %v; want the advance to stop at 2, short of the performed action's wait", got)
		}
		if got := exec.Results()["n"]; got.Kind != ValConst || got.Const.Int != 0 {
			t.Errorf("n = %v at t=2; want 0, the performed action has not gone on", got)
		}
		if waits := ctx.Clock().Waits(); len(waits) != 1 || waits[0].Due != 6 {
			t.Errorf("waits = %+v; want the performed action's wait due at 6", waits)
		}
		if next, ok := exec.NextWait(); !ok || next != 6 {
			t.Errorf("NextWait = %v, %v; want the performed action's wait at 6", next, ok)
		}
		if waits := exec.describeTimeWaits(); len(waits) != 1 || !strings.Contains(waits[0], "t=6.0") {
			t.Errorf("describeTimeWaits = %v; want the performed action's wait", waits)
		}
		if err := exec.Step(); !errors.Is(err, ErrNothingDue) || !strings.Contains(err.Error(), "t=6.0") {
			t.Errorf("Step = %v; want ErrNothingDue naming the performed action's wait", err)
		}
		if _, err := ctx.Advance(4); err != nil {
			t.Fatalf("Advance(4): %v", err)
		}
		if got := exec.Results()["n"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("n = %v once the clock reached 6; want 1", got)
		}
		if exec.State() != StateCompleted || ctx.Clock().Now() != 6 {
			t.Errorf("state = %v, clock = %v; want Completed at 6", exec.State(), ctx.Clock().Now())
		}
		if got := len(ctx.Clock().Waits()); got != 0 {
			t.Errorf("%d wait(s) left after the action completed; want none", got)
		}
	})
	t.Run("a performed action's wait is advanced to by a run of its own", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, performedWaitModel))
		action := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
		results, err := ctx.ExecuteAction(action)
		if err != nil {
			t.Fatalf("ExecuteAction: %v", err)
		}
		if got := results["n"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("n = %v; want 1", got)
		}
		if got := ctx.Clock().Now(); got != 6 {
			t.Errorf("clock = %v; want 6, where the performed action's wait came due", got)
		}
	})
	t.Run("a token held at a join beside a wait is not work due", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
			package test {
				private import SI::*;
				private import ScalarValues::*;
				action outer {
					attribute n : Integer = 0;
					first start;
					then fork split;
						then quick;
						then nap;
					action quick assign n := n + 1;
					then meet;
					action nap accept after 5 [s];
					then meet;
					join meet;
					then done;
				}
			}
		`))
		action := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
		if action == nil {
			t.Fatal("action outer not found")
		}
		exec, err := ctx.CreateActionExecutor(action)
		if err != nil {
			t.Fatalf("CreateActionExecutor: %v", err)
		}
		if err := exec.RunToQuiescence(); err != nil {
			t.Fatalf("RunToQuiescence: %v", err)
		}
		report, err := ctx.Advance(2)
		if err != nil {
			t.Fatalf("Advance(2): %v", err)
		}
		if report.To != 2 || len(report.Notes) != 0 {
			t.Errorf("report = %+v; want the clock at 2 and no choice drawn for one action", report)
		}
		if exec.State() != StateWaiting {
			t.Errorf("state = %v at t=2; want Waiting", exec.State())
		}
		if _, err := ctx.Advance(3); err != nil {
			t.Fatalf("Advance(3): %v", err)
		}
		if exec.State() != StateCompleted || ctx.Clock().Now() != 5 {
			t.Errorf("state = %v, clock = %v; want Completed at 5", exec.State(), ctx.Clock().Now())
		}
		if got := exec.Results()["n"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("n = %v; want 1", got)
		}
	})
	t.Run("a message for another flow does not wake a nested wait", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
			package test {
				private import SI::*;
				private import ScalarValues::*;
				action outer {
					attribute n : Integer = 0;
					attribute got : Integer = 0;
					first start;
					then fork split;
						then reader;
						then body;
					action reader accept m : Integer;
					then action record assign got := m;
					then meet;
					action body {
						if true {
							action inner {
								first start;
								then action nap accept after 5 [s];
								then action mark assign n := 1;
								then done;
							}
						}
					}
					then meet;
					join meet;
					then done;
				}
			}
		`))
		action := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
		if action == nil {
			t.Fatal("action outer not found")
		}
		exec, err := ctx.CreateActionExecutor(action)
		if err != nil {
			t.Fatalf("CreateActionExecutor: %v", err)
		}
		if err := exec.RunToQuiescence(); err != nil {
			t.Fatalf("RunToQuiescence: %v", err)
		}
		var nested *actionFrame
		for _, token := range exec.tokens {
			if token.Wait != nil && token.Wait.Timed {
				nested = token.frame
			}
		}
		if nested == nil {
			t.Fatalf("tokens = %+v; want one parked on the clock in the nested flow", exec.tokens)
		}
		seven := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 7}}
		ctx.PostMessage(Message{SignalType: "Integer", Target: "reader", Value: &seven})
		if !exec.HasPendingSignal() {
			t.Error("HasPendingSignal = false; want the reader's message pending for the action")
		}
		if exec.hasPendingSignal(nested) || exec.dueNow(nested) {
			t.Error("the nested flow counts the reader's message as its own")
		}
		if err := exec.RunToQuiescence(); err != nil {
			t.Fatalf("RunToQuiescence after the message: %v", err)
		}
		if got := exec.Results()["got"]; got.Kind != ValConst || got.Const.Int != 7 {
			t.Errorf("got = %v; want 7, the reader took its message", got)
		}
		if got := exec.Results()["n"]; got.Kind != ValConst || got.Const.Int != 0 {
			t.Errorf("n = %v at t=0; want 0, the nested flow still waits on the clock", got)
		}
		if got := ctx.Clock().Now(); got != 0 {
			t.Errorf("clock = %v after a run at the current instant; want 0", got)
		}
		if _, err := ctx.Advance(5); err != nil {
			t.Fatalf("Advance(5): %v", err)
		}
		if got := exec.Results()["n"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("n = %v once the clock reached 5; want 1", got)
		}
		if exec.State() != StateCompleted {
			t.Errorf("state = %v; want Completed", exec.State())
		}
	})
	t.Run("event budget bounds a machine that never settles", func(t *testing.T) {
		ctx, _, root := newRun(t)
		budgets := ctx.Budgets()
		budgets.MaxStateEvents = 5
		if err := ctx.SetBudgets(budgets); err != nil {
			t.Fatal(err)
		}
		machine := findSymbolByName(root, "metronome", ast.DefState)
		if _, err := ctx.CreateStateExecutor(machine); err != nil {
			t.Fatalf("CreateStateExecutor: %v", err)
		}
		_, err := ctx.Advance(1000)
		if !errors.Is(err, ErrStateEventLimitExceeded) {
			t.Fatalf("Advance = %v; want ErrStateEventLimitExceeded", err)
		}
		if !strings.Contains(err.Error(), MaxStateEventsEnvVar) {
			t.Errorf("err = %v; want it to say how to raise the budget", err)
		}
		if got := ctx.Clock().Now(); got > 6 {
			t.Errorf("clock = %v; want it stopped where the budget ran out", got)
		}
	})
	t.Run("step budget spans every instant of one advance", func(t *testing.T) {
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
			package test {
				private import SI::*;
				private import ScalarValues::*;
				action ticker {
					attribute ticks : Integer = 0;
					first start;
					then action tick assign ticks := ticks + 1;
					then action rest accept after 1 [s];
					then tick;
				}
			}
		`))
		budgets := ctx.Budgets()
		budgets.MaxActionSteps = 40
		if err := ctx.SetBudgets(budgets); err != nil {
			t.Fatal(err)
		}
		action := findSymbolByName(idx.DocumentRoot("<test>"), "ticker", ast.DefAction)
		if action == nil {
			t.Fatal("action ticker not found")
		}
		exec, err := ctx.CreateActionExecutor(action)
		if err != nil {
			t.Fatalf("CreateActionExecutor: %v", err)
		}
		if err := exec.Step(); err != nil {
			t.Fatalf("Step: %v", err)
		}
		report, err := ctx.Advance(1000)
		if !errors.Is(err, ErrActionStepLimitExceeded) {
			t.Fatalf("Advance = %v (report %+v); want ErrActionStepLimitExceeded once the steps of every wake add up", err, report)
		}
		if !strings.Contains(err.Error(), MaxActionStepsEnvVar) {
			t.Errorf("err = %v; want it to say how to raise the budget", err)
		}
		if report.Steps > budgets.MaxActionSteps {
			t.Errorf("report.Steps = %d; want at most the budget of %d", report.Steps, budgets.MaxActionSteps)
		}
		if got := ctx.Clock().Now(); got >= 1000 {
			t.Errorf("clock = %v; want it stopped where the budget ran out", got)
		}
		if got := exec.Results()["ticks"]; got.Kind != ValConst || got.Const.Int >= budgets.MaxActionSteps {
			t.Errorf("ticks = %v; want fewer than the %d steps of the budget", got, budgets.MaxActionSteps)
		}
	})
	t.Run("event budget of one lets a lone action wake", func(t *testing.T) {
		ctx, exec, _ := newRun(t)
		budgets := ctx.Budgets()
		budgets.MaxStateEvents = 1
		if err := ctx.SetBudgets(budgets); err != nil {
			t.Fatal(err)
		}
		report, err := ctx.Advance(10)
		if err != nil {
			t.Fatalf("Advance = %v (report %+v); want the wake of one action, which dispatches no state event, within the budget", err, report)
		}
		if report.Events != 0 {
			t.Errorf("report.Events = %d; want none, an action step is no state event", report.Events)
		}
		if exec.State() != StateCompleted {
			t.Errorf("state = %v; want Completed", exec.State())
		}
		if got := exec.Results()["done"]; got.Kind != ValConst || got.Const.Int != 1 {
			t.Errorf("done = %v; want 1", got)
		}
		if got := ctx.Clock().Now(); got != 10 {
			t.Errorf("clock = %v; want 10", got)
		}
	})
	t.Run("accept when beside a timer fires when another executor changes the value", func(t *testing.T) {
		const model = `
			package test {
				private import SI::*;
				private import ScalarValues::*;
				part def Worker {
					attribute stage : Integer = 0;
					exhibit state shift {
						entry; then working;
						state working;
						accept after 2 [s] then armed;
						state armed { entry assign stage := 1; }
						accept after 1 [s] then later;
						state later { entry assign stage := 2; }
					}
				}
				part worker : Worker;
				action watcher {
					attribute seen : Integer = -1;
					attribute timed : Integer = -1;
					first start;
					fork split;
					action quick accept when worker.stage > 0;
					action noteQuick assign seen := worker.stage;
					action slow accept after 6 [s];
					action noteSlow assign timed := worker.stage;
					join meet;
					done;
					succession first start then split;
					succession first split then quick;
					succession first split then slow;
					succession first quick then noteQuick;
					succession first noteQuick then meet;
					succession first slow then noteSlow;
					succession first noteSlow then meet;
					succession first meet then done;
				}
				state lookout {
					attribute seen : Integer = -1;
					entry; then waiting;
					state waiting;
					accept when worker.stage > 0 then noticed;
					accept after 6 [s] then late;
					state noticed { entry assign seen := worker.stage; }
					state late { entry assign seen := 100 + worker.stage; }
				}
			}`
		// The change branch sees stage 1 at t=2 while the timer holds the run to t=6;
		// seeing 2 means it was only re-tested once the timer woke the executor.
		wantSeen := func(t *testing.T, what string, got Value) {
			t.Helper()
			if got.Kind != ValConst || got.Const.Int != 1 {
				t.Errorf("%s saw stage %v; want 1, the value at the instant the condition rose", what, got)
			}
		}
		t.Run("action driving the clock", func(t *testing.T) {
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, model))
			action := findSymbolByName(idx.DocumentRoot("<test>"), "watcher", ast.DefAction)
			out, err := ctx.ExecuteAction(action)
			if err != nil {
				t.Fatalf("ExecuteAction: %v", err)
			}
			wantSeen(t, "the accept when branch", out["seen"])
			if got := out["timed"]; got.Kind != ValConst || got.Const.Int != 2 {
				t.Errorf("the accept after branch saw stage %v; want 2 at t=6", got)
			}
			if got := ctx.Clock().Now(); got != 6 {
				t.Errorf("clock = %v; want 6, where the timer branch woke", got)
			}
		})
		t.Run("action driven by an advance", func(t *testing.T) {
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, model))
			action := findSymbolByName(idx.DocumentRoot("<test>"), "watcher", ast.DefAction)
			exec, err := ctx.CreateActionExecutor(action)
			if err != nil {
				t.Fatalf("CreateActionExecutor: %v", err)
			}
			if err := exec.RunToQuiescence(); err != nil {
				t.Fatalf("RunToQuiescence: %v", err)
			}
			if _, err := ctx.Advance(3); err != nil {
				t.Fatalf("Advance(3): %v", err)
			}
			wantSeen(t, "the accept when branch", exec.Results()["seen"])
			if exec.State() == StateCompleted {
				t.Error("the action completed before its timer branch was due")
			}
			if _, err := ctx.Advance(3); err != nil {
				t.Fatalf("Advance(3): %v", err)
			}
			if exec.State() != StateCompleted {
				t.Errorf("state = %v; want Completed once the timer branch woke", exec.State())
			}
		})
		t.Run("state machine driving the clock", func(t *testing.T) {
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, model))
			machine := findSymbolByName(idx.DocumentRoot("<test>"), "lookout", ast.DefState)
			exec, err := ctx.CreateStateExecutor(machine)
			if err != nil {
				t.Fatalf("CreateStateExecutor: %v", err)
			}
			if err := exec.RunToCompletion(); err != nil {
				t.Fatalf("RunToCompletion: %v", err)
			}
			wantSeen(t, "the change transition", exec.StateData()["seen"])
			if got := ctx.Clock().Now(); got != 2 {
				t.Errorf("clock = %v; want 2, the instant the condition rose", got)
			}
		})
	})
	t.Run("behaviors of an object that failed to start leave the clock", func(t *testing.T) {
		ctx, _, err := instantiateWithLibraries(t, `
			package test {
				private import SI::*;
				private import ScalarValues::*;
				part def Rig {
					attribute beats : Integer = 0;
					exhibit state pulse {
						entry; then idle;
						state idle;
						transition first idle accept after 1 [s] do assign beats := beats + 1 then idle;
					}
					perform action tick {
						first start;
						then action rest accept after 1 [s];
						then done;
					}
					exhibit state broken {
						state lonely;
					}
				}
			}`, "test::Rig")
		if !errors.Is(err, errNoInitialState) {
			t.Fatalf("instantiate = %v; want errNoInitialState from the machine with no initial state", err)
		}
		if got := len(ctx.clock.waiters); got != 0 {
			t.Errorf("clock drives %d executors after the failed start; want none", got)
		}
		if waits := ctx.Clock().Waits(); len(waits) != 0 {
			t.Errorf("clock waits = %v after the failed start; want none", waits)
		}
		if _, ok := ctx.Clock().NextDue(); ok {
			t.Error("clock has a next due instant after the failed start; want none")
		}
		report, err := ctx.Advance(10)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if report.Events != 0 || report.Steps != 0 || report.DoSteps != 0 {
			t.Errorf("report = %+v; want nothing run for behaviors the failed start withdrew", report)
		}
		if got := ctx.Clock().Now(); got != 10 {
			t.Errorf("clock = %v; want 10", got)
		}
	})
	t.Run("a do behavior paused in a timed action leaves the clock with its dropped machine", func(t *testing.T) {
		ctx, _, err := instantiateWithLibraries(t, `
			package test {
				private import SI::*;
				private import ScalarValues::*;
				action def Poll { inout n : Integer; action wait accept after 5 [s]; then assign n := n + 1; }
				part def Rig {
					attribute ticks : Integer = 0;
					exhibit state pulse {
						entry; then busy;
						state busy { do action poll : Poll { inout n = ticks; } }
					}
					perform action bad { action crash assign ticks := ticks / 0; }
				}
			}`, "test::Rig")
		if !errors.Is(err, ErrDivisionByZero) {
			t.Fatalf("instantiate = %v; want ErrDivisionByZero from the action that fails the start", err)
		}
		if got := len(ctx.clock.waiters); got != 0 {
			t.Errorf("clock drives %d executors after the failed start; want none: the performed Poll left with its machine", got)
		}
		if waits := ctx.Clock().Waits(); len(waits) != 0 {
			t.Errorf("clock waits = %v after the failed start; want none", waits)
		}
		if len(ctx.objectBehaviors) != 0 {
			t.Errorf("%d behavior(s) still attached after the failed start; want none", len(ctx.objectBehaviors))
		}
		report, err := ctx.Advance(10)
		if err != nil {
			t.Fatalf("Advance: %v", err)
		}
		if report.Events != 0 || report.Steps != 0 || report.DoSteps != 0 {
			t.Errorf("report = %+v; want nothing run for the do behavior the failed start withdrew", report)
		}
	})
}

// testActionAcceptNonBooleanChangeTrigger: a change trigger states a condition,
// so one that evaluates to something else is reported rather than read as true.
func testActionAcceptNonBooleanChangeTrigger(t *testing.T) {
	src := `
		package test {
			action monitor {
				attribute temp : Integer = 10;
				first start;
				action awaitWarm accept when temp;
				done;
				succession first start then awaitWarm;
				succession first awaitWarm then done;
			}
		}
	`
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, src))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "monitor", ast.DefAction)
	if sym == nil {
		t.Fatal("action monitor not found")
	}

	_, err := ctx.ExecuteAction(sym)
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("ExecuteAction error = %v, want ErrTypeMismatch", err)
	}
}
