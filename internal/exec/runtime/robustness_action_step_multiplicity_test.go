package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

func TestRuntimeRobustnessActionStepMultiplicity(t *testing.T) {
	tests := []struct {
		name, model, step, multiplicity, code, reason string
		wantRun                                       bool
		state                                         bool
		instantiate                                   bool
		inputs                                        map[string]Value
	}{
		{
			name: "zero-or-more", step: "a", multiplicity: "[0..*]",
			code: lower.StepMultiplicityNotFixedCode,
			model: `package test {
				action def A { first start then a; action a[0..*]; then done; }
			}`,
		},
		{
			name: "unbounded", step: "a", multiplicity: "[*]",
			code: lower.StepMultiplicityNotFixedCode,
			model: `package test {
				action def A { first start then a; action a[*]; then done; }
			}`,
		},
		{
			name: "one-or-more", step: "a", multiplicity: "[1..*]",
			code: lower.StepMultiplicityNotFixedCode,
			model: `package test {
				action def A { first start then a; action a[1..*]; then done; }
			}`,
		},
		{
			name: "finite-range", step: "a", multiplicity: "[2..5]",
			code: lower.StepMultiplicityNotFixedCode,
			model: `package test {
				action def A { first start then a; action a[2..5]; then done; }
			}`,
		},
		{
			name: "nonconstant-bound", step: "a", multiplicity: "[n]",
			code:   lower.StepMultiplicityNotFixedCode,
			inputs: map[string]Value{"n": intOf(3)},
			model: `package test {
				private import ScalarValues::*;
				action def A {
					in n : Integer;
					first start then a;
					action a[n];
					then done;
				}
			}`,
		},
		{
			name: "open-order", step: "a", multiplicity: "[3]",
			code: lower.StepOrderOpenCode,
			model: `package test {
				action def A {
					action p;
					action a[3];
					succession first [0..*] p then [0..*] a;
				}
			}`,
		},
		{
			name: "excluded-count", step: "a", multiplicity: "[3]",
			code: lower.StepOrderUnsatisfiableCode,
			model: `package test {
				action def A {
					action p;
					action a[3];
					succession first [1] p then [1] a;
				}
			}`,
		},
		{
			name: "unordered repeated step with outgoing succession", step: "a", multiplicity: "[3]",
			code: lower.StepOrderUnsatisfiableCode,
			model: `package test {
				private import ScalarValues::*;
				action def A {
					attribute c : Integer = 0;
					action a[3] { assign c := c + 1; } then q;
					action q { }
					then done;
				}
			}`,
		},
		{
			name: "single-count-written-end-excludes-count", step: "a", multiplicity: "[1]",
			code: lower.StepOrderUnsatisfiableCode,
			model: `package test {
				action def A {
					action p;
					action a[1];
					succession first [2] p then [1] a;
				}
			}`,
		},
		{
			name: "single-count-written-ends-run", wantRun: true, step: "a", multiplicity: "[1]",
			model: `package test {
				action def A {
					action p[1];
					action a[1];
					succession first [1] p then [1] a;
					then done;
				}
			}`,
		},
		{
			name: "unwritten-source-exact-one-reading", step: "a", multiplicity: "[3]",
			code: lower.StepOrderUnsatisfiableCode,
			model: `package test {
				action def A {
					action a[3];
					action q;
					succession first a then [1] q;
				}
			}`,
		},
		{
			name: "fork-adjacency", step: "a", multiplicity: "[3]",
			code: lower.StepMultiplicityUnsupportedCode,
			model: `package test {
				action def A {
					fork f;
					action a[3];
					succession first start then a;
					succession first a then f;
					succession first f then done;
				}
			}`,
		},
		{
			name: "guarded-succession", step: "a", multiplicity: "[3]",
			code: lower.StepMultiplicityUnsupportedCode,
			model: `package test {
				action def A {
					action a[3];
					action q;
					succession first a if true then q;
				}
			}`,
		},
		{
			name: "guarded-start-succession", step: "a", multiplicity: "[3]",
			code: lower.StepMultiplicityUnsupportedCode,
			model: `package test {
				action def A {
					action a[3];
					succession first start if true then a;
					then done;
				}
			}`,
		},
		{
			name: "guarded-done-succession", step: "a", multiplicity: "[3]",
			code: lower.StepMultiplicityUnsupportedCode,
			model: `package test {
				action def A {
					action a[3];
					first start then a;
					succession first a if true then done;
				}
			}`,
		},
		{
			name: "flow-at-repeated-pin", step: "a", multiplicity: "[3]",
			code: lower.StepMultiplicityUnsupportedCode,
			model: `package test {
				private import ScalarValues::*;
				action def A {
					action a[3] { out x : Integer; assign x := 1; }
					action q { in x : Integer; }
					first start then a;
					then q;
					then done;
					flow a.x to q.x;
				}
			}`,
		},
		{
			name: "external-feature-read", step: "a", multiplicity: "[3]",
			code: lower.StepMultiplicityUnsupportedCode,
			model: `package test {
				private import ScalarValues::*;
				action def A {
					attribute total : Integer = 0;
					first start then a;
					action a[3] { attribute x : Integer = 1; }
					then q;
					action q { assign total := a.x; }
					then done;
				}
			}`,
		},
		{
			name: "nested-external-feature-read", step: "inner", multiplicity: "[3]",
			code: lower.StepMultiplicityUnsupportedCode,
			model: `package test {
				private import ScalarValues::*;
				action def A {
					attribute total : Integer = 0;
					first start then outer;
					action outer {
						first start then inner;
						action inner[3] { attribute x : Integer = 1; }
						then done;
					}
					then q;
					action q { assign total := outer.inner.x; }
					then done;
				}
			}`,
		},
		{
			name: "zero-between-real-steps", step: "a", multiplicity: "[0]",
			code: lower.StepOrderOpenCode,
			model: `package test {
				action def A {
					action p;
					action a[0];
					action q;
					succession first start then p;
					succession first p then a;
					succession first a then q;
					succession first q then done;
				}
			}`,
		},
		{
			name: "entry-two", step: "enter", multiplicity: "[2]",
			code:  lower.StepMultiplicityUnsupportedCode,
			state: true,
			model: `package test {
				state def Machine {
					entry; then active;
					state active { entry action enter[2]; }
				}
			}`,
		},
		{
			name: "do-two", step: "tick", multiplicity: "[2]",
			code:  lower.StepMultiplicityUnsupportedCode,
			state: true,
			model: `package test {
				private import ScalarValues::*;
				state def Machine {
					attribute c : Integer = 0;
					entry; then active;
					state active {
						do action tick[2] { assign c := c + 1; }
					}
				}
			}`,
		},
		{
			name: "unordered state do behavior with a sibling", step: "tick", multiplicity: "[2]",
			code:  lower.StepMultiplicityUnsupportedCode,
			state: true,
			model: `package test {
				state def Machine {
					entry; then active;
					state active {
						do action tick[2] { }
						do action sibling { }
					}
				}
			}`,
		},
		{
			name: "bodiless-do-two", step: "tick", multiplicity: "[2]",
			code:  lower.StepMultiplicityUnsupportedCode,
			state: true,
			model: `package test {
				state def Machine {
					entry; then active;
					state active { do action tick[2]; }
				}
			}`,
		},
		{
			name: "while-block-three", step: "tick", multiplicity: "[3]",
			code:   lower.StepMultiplicityUnsupportedCode,
			reason: "a step inside a loop or conditional body is performed once per pass; repeated or zero counts are not executed there",
			model: `package test {
				private import ScalarValues::*;
				action def A {
					first start then worker;
					action worker {
						attribute i : Integer = 0;
						while i < 1 {
							action tick[3] { }
							assign i := i + 1;
						}
					}
					then done;
				}
			}`,
		},
		{
			name: "unordered while-block-three", step: "tick", multiplicity: "[3]",
			code:   lower.StepMultiplicityUnsupportedCode,
			reason: "a step inside a loop or conditional body is performed once per pass; repeated or zero counts are not executed there",
			model: `package test {
				action def A {
					first start then worker;
					action worker {
						while true {
							action anchor;
							first start then anchor;
							action tick[3] { }
						}
					}
					then done;
				}
			}`,
		},
		{
			name: "while-block-zero", step: "tick", multiplicity: "[0]",
			code:   lower.StepMultiplicityUnsupportedCode,
			reason: "a step inside a loop or conditional body is performed once per pass; repeated or zero counts are not executed there",
			model: `package test {
				private import ScalarValues::*;
				action def A {
					first start then worker;
					action worker {
						attribute i : Integer = 0;
						while i < 1 {
							action tick[0] { }
							assign i := i + 1;
						}
					}
					then done;
				}
			}`,
		},
		{
			name: "if-block-three", step: "tick", multiplicity: "[3]",
			code:   lower.StepMultiplicityUnsupportedCode,
			reason: "a step inside a loop or conditional body is performed once per pass; repeated or zero counts are not executed there",
			model: `package test {
				action def A {
					first start then worker;
					action worker {
						if true {
							action tick[3] { }
						}
					}
					then done;
				}
			}`,
		},
		{
			name: "if-block-zero", step: "tick", multiplicity: "[0]",
			code:   lower.StepMultiplicityUnsupportedCode,
			reason: "a step inside a loop or conditional body is performed once per pass; repeated or zero counts are not executed there",
			model: `package test {
				action def A {
					first start then worker;
					action worker {
						if true {
							action tick[0] { }
						}
					}
					then done;
				}
			}`,
		},
		{
			name: "if-block-one", step: "tick", multiplicity: "[1]", wantRun: true,
			model: `package test {
				action def A {
					first start then worker;
					action worker {
						if true {
							action tick[1] { }
						}
					}
					then done;
				}
			}`,
		},
		{
			name: "part-level performed action", step: "run", multiplicity: "[2]",
			code:        lower.StepMultiplicityUnsupportedCode,
			instantiate: true,
			model: `package test {
				action def Act { }
				part def Host { perform action run[2] : Act; }
			}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file := parseAndBuild(t, test.model)
			index, _, ctx := buildRuntimeWithLibraries(t, "<test>", file)
			ctx.maxSteps = 1000
			var err error
			if test.state {
				symbol := findSymbolByName(index.DocumentRoot("<test>"), "Machine", ast.DefState)
				if symbol == nil {
					t.Fatal("state Machine not found")
				}
				_, err = ctx.ExecuteState(symbol)
			} else if test.instantiate {
				symbol := findSymbolByName(index.DocumentRoot("<test>"), "Host", ast.DefPart)
				if symbol == nil {
					t.Fatal("part Host not found")
				}
				_, err = ctx.Instantiate(symbol)
			} else {
				symbol := findSymbolByName(index.DocumentRoot("<test>"), "A", ast.DefAction)
				if symbol == nil {
					t.Fatal("action A not found")
				}
				_, err = ctx.ExecuteActionWithInputs(symbol, test.inputs)
			}
			if test.wantRun {
				if err != nil {
					t.Fatalf("execution error = %v, want action to run", err)
				}
				return
			}
			if !errors.Is(err, ErrActionStepMultiplicity) {
				t.Fatalf("execution error = %v, want ErrActionStepMultiplicity", err)
			}
			var stepErr *lower.StepMultiplicityError
			if !errors.As(err, &stepErr) {
				t.Fatalf("execution error = %v, want *lower.StepMultiplicityError", err)
			}
			if stepErr.Code != test.code {
				t.Errorf("step error code = %q, want %q (%v)", stepErr.Code, test.code, err)
			}
			if !strings.Contains(err.Error(), test.step) || !strings.Contains(err.Error(), test.multiplicity) {
				t.Errorf("execution error = %q, want step %q and multiplicity %q", err, test.step, test.multiplicity)
			}
			if test.reason != "" && !strings.Contains(err.Error(), test.reason) {
				t.Errorf("execution error = %q, want reason %q", err, test.reason)
			}
		})
	}
}

func TestRuntimeActionStepMultiplicityTerminateEndsEveryPerformance(t *testing.T) {
	outputs, err := executeActionSource(t, "A", `package test {
		private import ScalarValues::*;
		action def A {
			attribute c : Integer = 0;
			first start then a;
			action a[3] {
				assign c := c + 1;
				terminate a;
				assign c := c + 10;
			}
			then done;
		}
	}`)
	if err != nil {
		t.Fatalf("executeActionSource: %v", err)
	}
	assertIntOutput(t, outputs, "c", 1)
}

func TestActionStepMultiplicityExploreHasOneExactOutcome(t *testing.T) {
	file := parseAndBuild(t, `package test {
		private import ScalarValues::*;
		action def Rep {
			attribute c : Integer = 0;
			first start then a;
			action a[3] { assign c := c + 1; }
			then done;
		}
	}`)
	index, _, base := buildRuntimeWithLibraries(t, "<test>", file)
	action := findSymbolByName(index.DocumentRoot("<test>"), "Rep", ast.DefAction)
	if action == nil {
		t.Fatal("action Rep not found")
	}
	exploration, err := Explore(
		context.Background(),
		DefaultExploreSchedulePolicy,
		func() (*Context, error) { return NewContext(base.model, 10000), nil },
		func(ctx *Context) (Outcome, error) {
			return ctx.ActionOutcomePerformedBy(action, nil, nil)
		},
	)
	if err != nil {
		t.Fatalf("explore action: %v", err)
	}
	if !exploration.Complete() || len(exploration.Outcomes) != 1 {
		t.Fatalf("exploration = %s with %d outcomes, want one complete outcome", exploration.Status(), len(exploration.Outcomes))
	}
	if got := exploration.Outcomes[0].Outcome.String(); !strings.Contains(got, "c = 3") {
		t.Errorf("explored outcome = %q, want c = 3", got)
	}
}

func TestActionStepMultiplicityUnorderedStartsFollowEachSchedule(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		action def Rep {
			attribute c : Integer = 0;
			action a[3] { assign c := c + 1; }
		}
	}`)
	action := m.action(t, "Rep")
	for _, spelling := range []string{"declared", "reverse"} {
		t.Run(spelling, func(t *testing.T) {
			ctx, err := m.fresh()
			if err != nil {
				t.Fatalf("fresh context: %v", err)
			}
			policy, err := ParseSchedulePolicy(spelling)
			if err != nil {
				t.Fatalf("ParseSchedulePolicy(%q): %v", spelling, err)
			}
			if err := ctx.SetSchedule(policy); err != nil {
				t.Fatalf("SetSchedule(%q): %v", spelling, err)
			}
			outcome, err := ctx.ActionOutcomePerformedBy(action, nil, nil)
			if err != nil {
				t.Fatalf("perform unordered repeated action: %v", err)
			}
			if outcome.Err != nil {
				t.Fatalf("action outcome error = %v", outcome.Err)
			}
			if got := outcome.String(); !strings.Contains(got, "c = 3") {
				t.Errorf("outcome = %q, want c = 3", got)
			}
		})
	}

	exploration, err := Explore(context.Background(), DefaultExploreSchedulePolicy, m.fresh, func(ctx *Context) (Outcome, error) {
		return ctx.ActionOutcomePerformedBy(action, nil, nil)
	})
	if err != nil {
		t.Fatalf("explore unordered repeated action: %v", err)
	}
	if !exploration.Complete() || len(exploration.Outcomes) != 1 {
		t.Fatalf("exploration = %s with %d outcomes, want one complete outcome", exploration.Status(), len(exploration.Outcomes))
	}
	if got := exploration.Outcomes[0].Outcome.String(); !strings.Contains(got, "c = 3") {
		t.Errorf("explored outcome = %q, want c = 3", got)
	}
}

func TestActionStepMultiplicityNestedUnorderedStartsExploreOneOutcome(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		action def Nested {
			attribute c : Integer = 0;
			action a[2] {
				action x { assign c := c + 1; }
				action y { assign c := c + 1; }
			}
		}
	}`)
	action := m.action(t, "Nested")
	exploration, err := Explore(context.Background(), DefaultExploreSchedulePolicy, m.fresh, func(ctx *Context) (Outcome, error) {
		return ctx.ActionOutcomePerformedBy(action, nil, nil)
	})
	if err != nil {
		t.Fatalf("explore nested unordered starts: %v", err)
	}
	if !exploration.Complete() || len(exploration.Outcomes) != 1 {
		t.Fatalf("exploration = %s with %d outcomes, want one complete outcome", exploration.Status(), len(exploration.Outcomes))
	}
	if got := exploration.Outcomes[0].Outcome.String(); !strings.Contains(got, "c = 4") {
		t.Errorf("explored outcome = %q, want c = 4", got)
	}
}

func TestActionStepMultiplicityExploreOmitsRepeatedLocals(t *testing.T) {
	path := filepath.Join("testdata", "conformance", "action_step_multiplicity_shared_writers.sysml")
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read shared-writers fixture: %v", err)
	}
	file := parseAndBuild(t, string(text))
	index, _, base := buildRuntimeWithLibraries(t, "<test>", file)
	action := findSymbolByName(index.DocumentRoot("<test>"), "SharedWriters", ast.DefAction)
	if action == nil {
		t.Fatal("action SharedWriters not found")
	}
	budget := DefaultExploreBudget
	budget.Runs = 65536
	policy, err := ExplorePolicy(budget)
	if err != nil {
		t.Fatalf("explore policy: %v", err)
	}
	exploration, err := Explore(
		context.Background(),
		policy,
		func() (*Context, error) { return NewContext(base.model, 10000), nil },
		func(ctx *Context) (Outcome, error) {
			exec, err := ctx.performAction(action, nil, nil)
			if err != nil {
				return Outcome{}, err
			}
			results := exec.Results()
			if _, ok := results["a.l"]; ok {
				t.Errorf("Results includes repeated local a.l: %v", results)
			}
			return ctx.ActionOutcome(results), nil
		},
	)
	if err != nil {
		t.Fatalf("explore shared writers: %v", err)
	}
	if !exploration.Complete() || len(exploration.Outcomes) != 1 {
		t.Fatalf("exploration = %s with %d outcomes, want one complete outcome", exploration.Status(), len(exploration.Outcomes))
	}
	want := map[string]bool{"c = 3": true}
	for _, explored := range exploration.Outcomes {
		got := explored.Outcome.String()
		if !want[got] {
			t.Errorf("unexpected distinct outcome %q", got)
		}
		delete(want, got)
	}
	if len(want) != 0 {
		t.Errorf("exploration did not reach expected outcomes: %v", want)
	}
}

func TestRuntimeRobustnessActionStepMultiplicityOutcomes(t *testing.T) {
	t.Run("top-level action explore", func(t *testing.T) {
		m := parseLibraryModel(t, `package test {
			action def Rep {
				first start then a;
				action a[0..*];
				then done;
			}
		}`)
		action := m.action(t, "Rep")
		exploration, err := Explore(context.Background(), DefaultExploreSchedulePolicy, m.fresh, func(ctx *Context) (Outcome, error) {
			return ctx.ActionOutcomePerformedBy(action, nil, nil)
		})
		assertActionStepMultiplicityExploreOutcome(t, exploration, err)
	})

	t.Run("unordered top-level action explore", func(t *testing.T) {
		m := parseLibraryModel(t, `package test {
			action def Rep {
				action a[0..*];
			}
		}`)
		action := m.action(t, "Rep")
		exploration, err := Explore(context.Background(), DefaultExploreSchedulePolicy, m.fresh, func(ctx *Context) (Outcome, error) {
			return ctx.ActionOutcomePerformedBy(action, nil, nil)
		})
		assertActionStepMultiplicityExploreOutcome(t, exploration, err)
	})

	t.Run("unaddressable top-level action explore", func(t *testing.T) {
		m := parseLibraryModel(t, `package test {
			action def Rep {
				first start then a;
				action a[2**70] { }
				then done;
			}
		}`)
		action := m.action(t, "Rep")
		exploration, err := Explore(context.Background(), DefaultExploreSchedulePolicy, m.fresh, func(ctx *Context) (Outcome, error) {
			return ctx.ActionOutcomePerformedBy(action, nil, nil)
		})
		if err != nil {
			t.Fatalf("Explore error = %v, want the refusal as an outcome", err)
		}
		if exploration == nil || len(exploration.Outcomes) != 1 || exploration.FailedLinearizations() != 1 {
			t.Fatalf("exploration = %v; want one error outcome and one failed linearization", exploration)
		}
		err = exploration.Outcomes[0].Outcome.Err
		assertActionStepMultiplicityErrorIsNotSetup(t, err)
		if !errors.Is(err, ErrIntegerUnaddressable) {
			t.Errorf("outcome error = %v, want ErrIntegerUnaddressable", err)
		}
		if !strings.Contains(err.Error(), "1180591620717411303424") {
			t.Errorf("outcome error = %q, want the exact bound", err)
		}
	})

	t.Run("unordered unaddressable top-level action explore", func(t *testing.T) {
		m := parseLibraryModel(t, `package test {
			action def Rep {
				action a[2**70] { }
			}
		}`)
		action := m.action(t, "Rep")
		exploration, err := Explore(context.Background(), DefaultExploreSchedulePolicy, m.fresh, func(ctx *Context) (Outcome, error) {
			return ctx.ActionOutcomePerformedBy(action, nil, nil)
		})
		if err != nil {
			t.Fatalf("Explore error = %v, want the refusal as an outcome", err)
		}
		if exploration == nil || len(exploration.Outcomes) != 1 || exploration.FailedLinearizations() != 1 {
			t.Fatalf("exploration = %v; want one error outcome and one failed linearization", exploration)
		}
		err = exploration.Outcomes[0].Outcome.Err
		assertActionStepMultiplicityErrorIsNotSetup(t, err)
		if !errors.Is(err, ErrIntegerUnaddressable) {
			t.Errorf("outcome error = %v, want ErrIntegerUnaddressable", err)
		}
		if !strings.Contains(err.Error(), "1180591620717411303424") {
			t.Errorf("outcome error = %q, want the exact bound", err)
		}
	})

	t.Run("state entry behavior explore", func(t *testing.T) {
		m := parseLibraryModel(t, `package test {
			private import ScalarValues::*;
			state def Machine {
				attribute entries : Integer = 0;
				entry; then active;
				state active {
					entry action e[2] { assign entries := entries + 1; }
				}
			}
		}`)
		machine := m.state(t, "Machine")
		exploration, err := Explore(context.Background(), DefaultExploreSchedulePolicy, m.fresh, func(ctx *Context) (Outcome, error) {
			return ctx.StateOutcomeWithEvents(machine, nil)
		})
		assertActionStepMultiplicityExploreOutcome(t, exploration, err)
	})

	t.Run("part-level performed behavior explore", func(t *testing.T) {
		m := parseLibraryModel(t, `package test {
			action def Act { }
			part def Host {
				perform action p[2] : Act;
			}
		}`)
		host := findSymbolByName(m.idx.DocumentRoot(m.path), "Host", ast.DefPart)
		if host == nil {
			t.Fatal("part Host not found")
		}
		exploration, err := Explore(context.Background(), DefaultExploreSchedulePolicy, m.fresh, func(ctx *Context) (Outcome, error) {
			_, err := ctx.Instantiate(host)
			return Outcome{}, err
		})
		assertActionStepMultiplicityExploreOutcome(t, exploration, err)
	})

	t.Run("check", func(t *testing.T) {
		m := parseLibraryModel(t, `package test {
			action def Rep {
				first start then a;
				action a[0..*];
				then done;
			}
		}`)
		report, err := Check(context.Background(), m.fresh, starterOf(m.action(t, "Rep")), CheckBudget{}, CheckOptions{}, nil)
		if err != nil {
			assertActionStepMultiplicityErrorIsNotSetup(t, err)
			return
		}
		if report == nil {
			t.Fatal("Check returned neither an error nor a report")
		}
		for _, violation := range report.Violations {
			if violation.Kind == ViolationFailure && errors.Is(violation.Err, ErrActionStepMultiplicity) {
				assertActionStepMultiplicityErrorIsNotSetup(t, violation.Err)
				return
			}
		}
		t.Fatalf("check report = %v, want an action-step-multiplicity failure violation", report)
	})
}

func assertActionStepMultiplicityExploreOutcome(t *testing.T, exploration *Exploration, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Explore error = %v, want the refusal as an outcome", err)
	}
	if exploration == nil || len(exploration.Outcomes) != 1 || exploration.FailedLinearizations() != 1 {
		t.Fatalf("exploration = %v; want one error outcome and one failed linearization", exploration)
	}
	assertActionStepMultiplicityErrorIsNotSetup(t, exploration.Outcomes[0].Outcome.Err)
}

func assertActionStepMultiplicityErrorIsNotSetup(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ErrActionStepMultiplicity) {
		t.Fatalf("error = %v, want ErrActionStepMultiplicity", err)
	}
	var setup *SetupError
	if errors.As(err, &setup) {
		t.Fatalf("error = %v, want a runtime refusal, not SetupError", err)
	}
}
