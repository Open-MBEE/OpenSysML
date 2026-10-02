package runtime

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestRuntimeRobustnessExploreFailures(t *testing.T) {
	t.Run("fresh context failure fails the exploration", testExploreFreshContextFailure)
	t.Run("root action initialization failure is setup", testExploreActionInitializationFailureIsSetup)
	t.Run("unordered and empty actions are not setup failures", testExploreUnorderedActionsAreNotSetup)
	t.Run("root state initialization failure is setup", testExploreStateInitializationFailureIsSetup)
	t.Run("initial entry failure after a choice is a runtime outcome", testExploreInitialEntryFailureAfterChoiceIsRuntimeOutcome)
	t.Run("deterministic entry failure is a runtime outcome", testExploreDeterministicEntryFailureIsRuntimeOutcome)
	t.Run("tool performance failure is a runtime outcome", testExploreToolPerformanceFailureIsRuntimeOutcome)
	t.Run("input binding failure is not setup", testExploreInputBindingFailureIsNotSetup)
	t.Run("performer failure is not setup", testExplorePerformerFailureIsOutcome)
	t.Run("decision failure is a runtime outcome", testExploreDecisionFailureIsOutcome)
	t.Run("join deadlock is a runtime outcome", testExploreJoinDeadlockIsOutcome)
	t.Run("step budget failure is a runtime outcome", testExploreStepBudgetFailureIsOutcome)
	t.Run("checker setup failure is returned without a violation", testCheckSetupFailureIsReturned)
}

func TestWrapSetupErrorPreservesInputAndPerformerErrors(t *testing.T) {
	cause := errors.New("input binding failed")
	for _, err := range []error{
		inputBindingError{Err: cause},
		fmt.Errorf("performer failed: %w", ErrOccurrenceLifetime),
	} {
		wrapped := wrapSetupError(err)
		var setup *SetupError
		if errors.As(wrapped, &setup) {
			t.Errorf("wrapSetupError(%v) = %v; want the run error unchanged", err, wrapped)
		}
	}

	cause = errors.New("executor initialization failed")
	wrapped := wrapSetupError(cause)
	var setup *SetupError
	if !errors.As(wrapped, &setup) || !errors.Is(wrapped, cause) || wrapped.Error() != cause.Error() {
		t.Errorf("wrapSetupError(%v) = %v; want SetupError preserving the message and cause", cause, wrapped)
	}
}

func testExploreFreshContextFailure(t *testing.T) {
	want := errors.New("fresh context unavailable")
	called := false
	exploration, err := ExploreWith(
		context.Background(),
		mustPolicy(t, "explore"),
		1,
		func(int) (*Context, error) { return nil, want },
		func(*Context) (Outcome, error) {
			called = true
			return Outcome{}, nil
		},
	)
	var setup *SetupError
	if !errors.As(err, &setup) || !errors.Is(err, want) {
		t.Fatalf("ExploreWith error = %v, want SetupError wrapping %v", err, want)
	}
	if exploration != nil || called {
		t.Fatalf("exploration = %v, run called = %v; want no exploration or run", exploration, called)
	}
}

func testExploreActionInitializationFailureIsSetup(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		action Broken {
			action a;
			action b;
			succession first a then b;
			succession first b then a;
		}
	}`)
	sym := m.action(t, "Broken")
	exploration, err := Explore(context.Background(), mustPolicy(t, "explore"), m.fresh, func(ctx *Context) (Outcome, error) {
		return ctx.ActionOutcomePerformedBy(sym, nil, nil)
	})
	var setup *SetupError
	if !errors.As(err, &setup) || exploration != nil {
		t.Fatalf("exploration = %v, error = %v; want root setup failure without outcomes", exploration, err)
	}
	if !errors.Is(err, ErrInvalidActionFlow) {
		t.Errorf("setup error = %v, want ErrInvalidActionFlow", err)
	}
}

func testExploreUnorderedActionsAreNotSetup(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		action empty {}
		action unordered {
			attribute c : Integer := 0;
			action a { assign c := c + 1; }
			action b { assign c := c + 10; }
		}
		action joined {
			attribute c : Integer := 0;
			action a { assign c := c + 1; }
			action b { assign c := c + 10; }
			action d { assign c := c + 100; }
			succession first a then d;
			succession first b then d;
		}
	}`)
	for name, runs := range map[string]int{"empty": 1, "unordered": 2, "joined": 2} {
		exploration := exploreActionWith(t, m, name, 0)
		if !exploration.Complete() || exploration.Runs != runs || exploration.FailedLinearizations() != 0 {
			t.Errorf("explore %s: %s with %d runs and %d failures; want a complete %d-run search without failures",
				name, exploration.Status(), exploration.Runs, exploration.FailedLinearizations(), runs)
		}
		if len(exploration.Outcomes) != 1 || exploration.Outcomes[0].Outcome.Err != nil {
			t.Errorf("explore %s outcomes = %+v; want one successful outcome", name, exploration.Outcomes)
		}
	}
}

func testExploreStateInitializationFailureIsSetup(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		state def Machine {
			state idle;
			state active;
		}
	}`)
	sym := m.state(t, "Machine")
	exploration, err := Explore(context.Background(), mustPolicy(t, "explore"), m.fresh, func(ctx *Context) (Outcome, error) {
		return ctx.StateOutcomeWithEvents(sym, nil)
	})
	var setup *SetupError
	if !errors.As(err, &setup) || exploration != nil {
		t.Fatalf("exploration = %v, error = %v; want root setup failure without outcomes", exploration, err)
	}
	if !errors.Is(err, ErrNoInitialState) {
		t.Errorf("setup error = %v, want ErrNoInitialState", err)
	}
}

func testExploreInitialEntryFailureAfterChoiceIsRuntimeOutcome(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		action def Dec {
			attribute x : Integer = 0;
			first start;
			then fork f;
				then a;
				then b;
			action a { assign x := 1; }
			action b { assign x := 2; }
			succession a then j;
			succession b then j;
			join j;
			then decide d;
			if x == 1 then ok;
			action ok { assign x := 5; }
			then done;
		}
		state def Machine {
			entry; then s;
			state s { entry action e : Dec; }
		}
	}`)
	sym := m.state(t, "Machine")
	exploration, err := Explore(context.Background(), mustPolicy(t, "explore"), m.fresh, func(ctx *Context) (Outcome, error) {
		return ctx.StateOutcomeWithEvents(sym, nil)
	})
	if err != nil {
		t.Fatalf("Explore: %v; want entry failures as outcomes", err)
	}
	if exploration == nil || !exploration.Complete() || len(exploration.Outcomes) != 2 ||
		exploration.FailedLinearizations() != 1 {
		t.Fatalf("exploration = %v; want two complete outcomes with one failed linearization", exploration)
	}
	values, failures := 0, 0
	for _, outcome := range exploration.Outcomes {
		if outcome.Outcome.Err == nil {
			values++
			continue
		}
		failures += outcome.Linearizations
		var setup *SetupError
		if errors.As(outcome.Outcome.Err, &setup) {
			t.Errorf("entry error outcome = %v; want a runtime error, not SetupError", outcome.Outcome.Err)
		}
		if !errors.Is(outcome.Outcome.Err, ErrNoEnabledSuccession) {
			t.Errorf("entry error outcome = %v; want ErrNoEnabledSuccession", outcome.Outcome.Err)
		}
	}
	if values != 1 || failures != 1 {
		t.Errorf("exploration has %d value outcomes and %d failing linearizations; want 1 each", values, failures)
	}
}

func testExploreDeterministicEntryFailureIsRuntimeOutcome(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		action def AllFalse {
			attribute x : Integer = 0;
			first start;
			then decide d;
			if x == 1 then ok;
			action ok { assign x := 5; }
			then done;
		}
		state def Machine {
			entry; then s;
			state s { entry action e : AllFalse; }
		}
	}`)
	sym := m.state(t, "Machine")
	exploration, err := Explore(context.Background(), mustPolicy(t, "explore"), m.fresh, func(ctx *Context) (Outcome, error) {
		return ctx.StateOutcomeWithEvents(sym, nil)
	})
	if err != nil {
		t.Fatalf("Explore: %v; want the entry failure as an outcome", err)
	}
	if exploration == nil || !exploration.Complete() || len(exploration.Outcomes) != 1 ||
		exploration.FailedLinearizations() != 1 {
		t.Fatalf("exploration = %v; want one complete error outcome", exploration)
	}
	errOutcome := exploration.Outcomes[0].Outcome.Err
	var setup *SetupError
	if errOutcome == nil || errors.As(errOutcome, &setup) || !errors.Is(errOutcome, ErrNoEnabledSuccession) {
		t.Fatalf("entry error outcome = %v; want ErrNoEnabledSuccession, not SetupError", errOutcome)
	}
}

func testExploreToolPerformanceFailureIsRuntimeOutcome(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import AnalysisTooling::*;
		action def ToolAction {
			metadata ToolExecution {
				toolName = "Unregistered";
				uri = "aserv://localhost/ToolAction";
			}
		}
	}`)
	sym := m.action(t, "ToolAction")
	exploration, err := Explore(context.Background(), mustPolicy(t, "explore"), m.fresh, func(ctx *Context) (Outcome, error) {
		exec, err := ctx.CreateActionExecutorFor(sym, nil)
		if err != nil {
			return Outcome{}, err
		}
		defer exec.Release()
		return Outcome{}, errors.New("tool action unexpectedly succeeded")
	})
	if err != nil {
		t.Fatalf("Explore: %v; want the tool performance failure as an outcome", err)
	}
	if exploration == nil || !exploration.Complete() || len(exploration.Outcomes) != 1 ||
		exploration.FailedLinearizations() != 1 {
		t.Fatalf("exploration = %v; want one complete error outcome", exploration)
	}
	errOutcome := exploration.Outcomes[0].Outcome.Err
	var setup *SetupError
	if errOutcome == nil || errors.As(errOutcome, &setup) || !errors.Is(errOutcome, ErrToolNotRegistered) {
		t.Fatalf("tool error outcome = %v; want ErrToolNotRegistered, not SetupError", errOutcome)
	}
}

func testExploreInputBindingFailureIsNotSetup(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		action NeedsInput {
			in value : Integer;
			first start;
			then done;
		}
	}`)
	sym := m.action(t, "NeedsInput")
	exploration, err := Explore(context.Background(), mustPolicy(t, "explore"), m.fresh, func(ctx *Context) (Outcome, error) {
		return ctx.ActionOutcomePerformedBy(sym, nil, map[string]Value{"unknown": integerValue(1)})
	})
	if err != nil || exploration == nil || !exploration.Complete() || len(exploration.Outcomes) != 1 {
		t.Fatalf("exploration = %v, error = %v; want one input-binding error outcome", exploration, err)
	}
	runErr := exploration.Outcomes[0].Outcome.Err
	var setup *SetupError
	if runErr == nil || errors.As(runErr, &setup) || !errors.Is(runErr, ErrUnknownActionInput) {
		t.Fatalf("input-binding error = %v; want an unwrapped ErrUnknownActionInput runtime outcome", runErr)
	}
}

func testExplorePerformerFailureIsOutcome(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		part def Probe {
			attribute n : Integer = 0;
			action bump { assign n := n + 1; }
		}
		part p : Probe;
		action reaper { first start; then action kill { terminate test::p; } then done; }
	}`)
	bump := m.action(t, "Probe::bump")
	reaper := m.action(t, "reaper")
	performers := make(map[*Context]*Instance)
	exploration, err := Explore(context.Background(), mustPolicy(t, "explore"), func() (*Context, error) {
		ctx, err := m.fresh()
		if err != nil {
			return nil, err
		}
		part := namedOrFoundSymbol(t, m.idx, "test::p", m.idx.DocumentRoot(m.path), ast.DefPart, ast.UsagePart)
		self, err := ctx.Instantiate(part)
		if err != nil {
			return nil, err
		}
		if _, err := ctx.ExecuteAction(reaper); err != nil {
			return nil, err
		}
		performers[ctx] = self
		return ctx, nil
	}, func(ctx *Context) (Outcome, error) {
		return ctx.ActionOutcomePerformedBy(bump, performers[ctx], nil)
	})
	if err != nil {
		t.Fatalf("Explore: %v; want a runtime-error outcome", err)
	}
	if exploration == nil || len(exploration.Outcomes) != 1 || exploration.FailedLinearizations() != 1 {
		t.Fatalf("exploration = %v; want one failing linearization", exploration)
	}
	if !errors.Is(exploration.Outcomes[0].Outcome.Err, ErrOccurrenceLifetime) {
		t.Errorf("outcome error = %v, want ErrOccurrenceLifetime", exploration.Outcomes[0].Outcome.Err)
	}
}

func testExploreDecisionFailureIsOutcome(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		action mixed {
			attribute x : Integer = 0;
			attribute y : Integer = 0;
			first start;
			fork split;
			action a { assign x := 1; }
			action b { assign x := 2; }
			join sync;
			then decide select;
			if x == 1 then succeed;
			action succeed { assign y := 1; }
			then done;
			succession first start then split;
			succession first split then a;
			succession first split then b;
			succession first a then sync;
			succession first b then sync;
		}
	}`)
	exploration := exploreActionWith(t, m, "mixed", 0)
	assertPartialFailureExploration(t, exploration, ErrNoEnabledSuccession)
}

func testExploreJoinDeadlockIsOutcome(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		action mixed {
			attribute x : Integer = 0;
			first start;
			fork split;
			action a { assign x := 1; }
			action b { assign x := 2; }
			join sync;
			then decide route;
			if x == 1 then done;
			if x == 2 then starve;
			action starve {
				first start;
				ref action stranded;
				join wait;
				done;
				succession first start then wait;
				succession first stranded then wait;
				succession first wait then done;
			}
			succession first start then split;
			succession first split then a;
			succession first split then b;
			succession first a then sync;
			succession first b then sync;
		}
	}`)
	exploration := exploreActionWith(t, m, "mixed", 0)
	assertPartialFailureExploration(t, exploration, ErrActionDeadlock)
}

func testExploreStepBudgetFailureIsOutcome(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		action mixed {
			attribute x : Integer = 0;
			attribute ticks : Integer = 0;
			first start;
			fork split;
			action a { assign x := 1; }
			action b { assign x := 2; }
			join sync;
			then decide choose;
			if x == 1 then done;
			if x == 2 then spin;
			action spin {
				first leg;
				action leg {
					first a;
					action a;
					action b;
					succession first a then b;
					succession first b then a;
				}
			}
			succession first start then split;
			succession first split then a;
			succession first split then b;
			succession first a then sync;
			succession first b then sync;
		}
	}`)
	exploration := exploreActionWith(t, m, "mixed", 24)
	assertPartialFailureExploration(t, exploration, ErrActionStepLimitExceeded)
}

func exploreActionWith(t *testing.T, m *exploreModel, name string, actionSteps int64) *Exploration {
	t.Helper()
	sym := m.action(t, name)
	exploration, err := Explore(context.Background(), mustPolicy(t, "explore"), func() (*Context, error) {
		ctx, err := m.fresh()
		if err != nil || actionSteps == 0 {
			return ctx, err
		}
		budgets := ctx.Budgets()
		budgets.MaxActionSteps = actionSteps
		if err := ctx.SetBudgets(budgets); err != nil {
			return nil, err
		}
		return ctx, nil
	}, func(ctx *Context) (Outcome, error) {
		outputs, err := ctx.ExecuteAction(sym)
		if err != nil {
			return Outcome{}, err
		}
		return ctx.ActionOutcome(outputs), nil
	})
	if err != nil {
		t.Fatalf("explore %s: %v", name, err)
	}
	return exploration
}

func assertPartialFailureExploration(t *testing.T, exploration *Exploration, want error) {
	t.Helper()
	if !exploration.Complete() || exploration.FailedLinearizations() == 0 {
		t.Fatalf("exploration %s with %d failures, want a complete partial-failure search", exploration.Status(), exploration.FailedLinearizations())
	}
	if exploration.Weighted() || exploration.ProbabilitiesBounded() {
		t.Errorf("unweighted exploration reports weighted=%v bounded=%v", exploration.Weighted(), exploration.ProbabilitiesBounded())
	}
	values, failures := 0, 0
	for _, outcome := range exploration.Outcomes {
		if outcome.Probability != nil {
			t.Errorf("unweighted outcome %q has probability range %+v", outcome.Outcome, outcome.Probability)
		}
		if outcome.Outcome.Err == nil {
			values++
		} else {
			failures += outcome.Linearizations
			if !errors.Is(outcome.Outcome.Err, want) {
				t.Errorf("runtime error outcome = %v, want %v", outcome.Outcome.Err, want)
			}
		}
	}
	if values == 0 || failures != exploration.FailedLinearizations() {
		t.Errorf("exploration has %d value outcomes, %d failed linearizations; failure count reports %d", values, failures, exploration.FailedLinearizations())
	}
}

func testCheckSetupFailureIsReturned(t *testing.T) {
	cause := errors.New("root executor refused")
	m := parseLibraryModel(t, `package test {
		action pass { first start; then done; }
	}`)
	start := func(*Context) (*Invocation, error) {
		return nil, &SetupError{Err: cause}
	}
	report, err := Check(context.Background(), m.fresh, start, CheckBudget{}, CheckOptions{}, nil)
	var setup *SetupError
	if !errors.As(err, &setup) || !errors.Is(err, cause) {
		t.Fatalf("Check error = %v, want SetupError wrapping %v", err, cause)
	}
	if report != nil {
		t.Fatalf("Check report = %+v, want no report for setup failure", report)
	}
}
