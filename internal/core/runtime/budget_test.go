package runtime

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/envvar"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// TestBudgetFromValue covers what each variable resolves to: unset, empty,
// valid, padded, and the shapes that are errors.
func TestBudgetFromValue(t *testing.T) {
	values := []struct {
		name  string
		value string
		// want is relative to the variable's default when useDefault is set.
		want       int64
		useDefault bool
		// errContains is empty when the value is accepted.
		errContains string
	}{
		{name: "unset", value: "", useDefault: true},
		{name: "whitespace_only", value: "   ", useDefault: true},
		{name: "valid", value: "5000000", want: 5000000},
		{name: "surrounding_whitespace", value: "  2500 \n", want: 2500},
		{name: "one", value: "1", want: 1},
		{name: "zero", value: "0", errContains: "must be greater than zero"},
		{name: "negative", value: "-5", errContains: "must be greater than zero"},
		{name: "non_numeric", value: "lots", errContains: "is not an integer"},
		{name: "float", value: "1e6", errContains: "is not an integer"},
		{name: "trailing_garbage", value: "1000steps", errContains: "is not an integer"},
	}

	for _, v := range budgetVars {
		for _, tt := range values {
			// A bound with a ceiling accepts a value only up to it, so the
			// accepted values are read against that bound's own range.
			if v.ceiling > 0 && tt.want > v.ceiling {
				tt.want = v.ceiling
				tt.value = strconv.FormatInt(v.ceiling, 10)
			}
			t.Run(v.env+"/"+tt.name, func(t *testing.T) {
				got, err := budgetFromValue(v, tt.value)
				if tt.errContains != "" {
					if err == nil {
						t.Fatalf("budgetFromValue(%q) = %d, want an error", tt.value, got)
					}
					if !strings.Contains(err.Error(), tt.errContains) {
						t.Errorf("error %q does not contain %q", err, tt.errContains)
					}
					// The message must name the variable and the offending value,
					// so the reader knows what to fix.
					if !strings.Contains(err.Error(), v.env) {
						t.Errorf("error %q does not name %s", err, v.env)
					}
					if !strings.Contains(err.Error(), tt.value) {
						t.Errorf("error %q does not quote the offending value %q", err, tt.value)
					}
					if got != 0 {
						t.Errorf("rejected value yielded budget %d, want 0", got)
					}
					return
				}
				if err != nil {
					t.Fatalf("budgetFromValue(%q) errored: %v", tt.value, err)
				}
				want := tt.want
				if tt.useDefault {
					want = v.def
				}
				if got != want {
					t.Errorf("budgetFromValue(%q) = %d, want %d", tt.value, got, want)
				}
			})
		}
		if v.ceiling == 0 {
			continue
		}
		t.Run(v.env+"/above_the_ceiling", func(t *testing.T) {
			raw := strconv.FormatInt(v.ceiling+1, 10)
			got, err := budgetFromValue(v, raw)
			if err == nil {
				t.Fatalf("budgetFromValue(%q) = %d, want an error", raw, got)
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("must be at most %d", v.ceiling)) {
				t.Errorf("error %q does not report the ceiling %d", err, v.ceiling)
			}
		})
	}
}

// TestBudgetsFromLookup: each variable sets its own bound and leaves the others
// at their defaults, and every unusable value is reported at once.
func TestBudgetsFromLookup(t *testing.T) {
	t.Run("all_unset", func(t *testing.T) {
		got, err := budgetsFromLookup(func(string) string { return "" })
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != DefaultBudgets() {
			t.Errorf("got %+v, want the defaults %+v", got, DefaultBudgets())
		}
	})

	t.Run("each_variable_sets_only_its_own_bound", func(t *testing.T) {
		for _, v := range budgetVars {
			got, err := budgetsFromLookup(func(name string) string {
				if name == v.env {
					return " 4242 "
				}
				return ""
			})
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", v.env, err)
			}
			want := DefaultBudgets()
			*v.field(&want) = 4242
			if got != want {
				t.Errorf("%s: got %+v, want %+v", v.env, got, want)
			}
		}
	})

	t.Run("reports_every_unusable_value", func(t *testing.T) {
		got, err := budgetsFromLookup(func(name string) string {
			if name == MaxStepsEnvVar || name == MaxDoStepsEnvVar {
				return "lots"
			}
			return ""
		})
		if err == nil {
			t.Fatalf("got %+v, want an error", got)
		}
		for _, name := range []string{MaxStepsEnvVar, MaxDoStepsEnvVar} {
			if !strings.Contains(err.Error(), name) {
				t.Errorf("error %q does not name %s", err, name)
			}
		}
		if got != (Budgets{}) {
			t.Errorf("rejected environment yielded %+v, want the zero value", got)
		}
	})
}

// TestBudgetsFromEnv: the resolver reads the process environment, and reports an
// unusable value there the same way.
func TestBudgetsFromEnv(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		for _, v := range budgetVars {
			t.Setenv(v.env, "")
		}
		got, err := BudgetsFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != DefaultBudgets() {
			t.Errorf("got %+v, want the defaults %+v", got, DefaultBudgets())
		}
	})

	t.Run("raised", func(t *testing.T) {
		for _, v := range budgetVars {
			t.Setenv(v.env, "")
		}
		t.Setenv(MaxStepsEnvVar, "250000")
		t.Setenv(MaxStateEventsEnvVar, "20000")
		got, err := BudgetsFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.MaxSteps != 250000 || got.MaxStateEvents != 20000 {
			t.Errorf("got %+v, want MaxSteps 250000 and MaxStateEvents 20000", got)
		}
		if got.MaxDoSteps != DefaultMaxDoSteps {
			t.Errorf("MaxDoSteps = %d, want the untouched default %d", got.MaxDoSteps, DefaultMaxDoSteps)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Setenv(MaxActionStepsEnvVar, "many")
		if _, err := BudgetsFromEnv(); err == nil {
			t.Fatal("expected an error for a non-numeric value")
		}
	})

	t.Run("legacy names", func(t *testing.T) {
		for _, v := range budgetVars {
			t.Setenv(v.env, "")
			t.Setenv(envvar.Legacy(v.env), "")
		}
		t.Setenv(envvar.Legacy(MaxStepsEnvVar), "300000")
		t.Setenv(envvar.Legacy(MaxStateEventsEnvVar), "40000")
		got, err := BudgetsFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.MaxSteps != 300000 || got.MaxStateEvents != 40000 {
			t.Errorf("got %+v, want MaxSteps 300000 and MaxStateEvents 40000 from the legacy names", got)
		}
	})

	t.Run("both set, new name wins", func(t *testing.T) {
		for _, v := range budgetVars {
			t.Setenv(v.env, "")
			t.Setenv(envvar.Legacy(v.env), "")
		}
		t.Setenv(MaxStepsEnvVar, "500000")
		t.Setenv(envvar.Legacy(MaxStepsEnvVar), "111")
		got, err := BudgetsFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.MaxSteps != 500000 {
			t.Errorf("MaxSteps = %d, want the OPENSYSML_ value 500000 over the legacy 111", got.MaxSteps)
		}
	})
}

// TestSetBudgets: a context runs under the bounds it is given, and rejects a set
// holding a non-positive bound rather than running under it.
func TestSetBudgets(t *testing.T) {
	model, resolver, _ := parseAndBuildModel(t, `part def Simple {}`)
	ctx := NewContext(model, resolver, DefaultMaxSteps)
	if got := ctx.Budgets(); got != DefaultBudgets() {
		t.Errorf("a new context runs under %+v, want the defaults %+v", got, DefaultBudgets())
	}

	want := Budgets{MaxSteps: 11, MaxActionSteps: 22, MaxStateEvents: 33, MaxDoSteps: 44, MaxElements: 55, MaxCalcDepth: 66, MaxSweepRuns: 77}
	if err := ctx.SetBudgets(want); err != nil {
		t.Fatalf("SetBudgets: %v", err)
	}
	if got := ctx.Budgets(); got != want {
		t.Errorf("Budgets() = %+v, want %+v", got, want)
	}

	for _, bad := range []Budgets{
		{MaxSteps: 0, MaxActionSteps: 1, MaxStateEvents: 1, MaxDoSteps: 1, MaxElements: 1, MaxCalcDepth: 1, MaxSweepRuns: 1},
		{MaxSteps: 1, MaxActionSteps: -1, MaxStateEvents: 1, MaxDoSteps: 1, MaxElements: 1, MaxCalcDepth: 1, MaxSweepRuns: 1},
		{MaxSteps: 1, MaxActionSteps: 1, MaxStateEvents: 1, MaxDoSteps: 1, MaxElements: 0, MaxCalcDepth: 1, MaxSweepRuns: 1},
		{MaxSteps: 1, MaxActionSteps: 1, MaxStateEvents: 1, MaxDoSteps: 1, MaxElements: 1, MaxCalcDepth: 0, MaxSweepRuns: 1},
		{MaxSteps: 1, MaxActionSteps: 1, MaxStateEvents: 1, MaxDoSteps: 1, MaxElements: 1, MaxCalcDepth: MaxCalcDepthCeiling + 1, MaxSweepRuns: 1},
		{MaxSteps: 1, MaxActionSteps: 1, MaxStateEvents: 1, MaxDoSteps: 1, MaxElements: 1, MaxCalcDepth: 1, MaxSweepRuns: 0},
		{},
	} {
		if err := ctx.SetBudgets(bad); err == nil {
			t.Errorf("SetBudgets(%+v) was accepted", bad)
		}
	}
	if got := ctx.Budgets(); got != want {
		t.Errorf("a rejected set changed the bounds to %+v, want %+v", got, want)
	}
}

// TestStepLimitErrorNamesEffectiveBudgetAndVariable: the reported limit is the
// one in force, and the message says which variable raises it.
func TestStepLimitErrorNamesEffectiveBudgetAndVariable(t *testing.T) {
	src := `part def Simple {}`
	model, resolver, _ := parseAndBuildModel(t, src)
	ctx := NewContext(model, resolver, 3)

	// The bound is per run, so the evaluations have to spend one run's budget
	// rather than a run each.
	defer ctx.beginRun()()

	var err error
	for i := 0; i < 4 && err == nil; i++ {
		_, err = ctx.Eval(&ast.LiteralInteger{Value: "1"})
	}
	if err == nil {
		t.Fatal("expected the step budget to be exceeded")
	}
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("expected ErrStepLimitExceeded, got %v", err)
	}
	if !strings.Contains(err.Error(), "(3 steps") {
		t.Errorf("error %q does not report the effective budget of 3", err)
	}
	if !strings.Contains(err.Error(), MaxStepsEnvVar) {
		t.Errorf("error %q does not name %s", err, MaxStepsEnvVar)
	}
}

// TestRaisedBudgetRunsLongerLoop: a 10 000-iteration loop exhausts the 100 000
// steps that used to be the default and completes under today's, which is the
// point of both raising the default and making it configurable.
func TestRaisedBudgetRunsLongerLoop(t *testing.T) {
	src := `
		package L {
			action loopn {
				attribute i = 0;
				attribute s = 0.0;
				first start;
				action go { while i < 10000 { assign s := s + 1.5; assign i := i + 1; } }
				done;
				succession first start then go;
				succession first go then done;
			}
		}
	`
	file := parseAndBuild(t, src)

	runLoop := func(t *testing.T, maxSteps int64) error {
		idx, _, ctx := buildRuntime(t, "<test>", file)
		ctx.maxSteps = maxSteps
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "loopn", ast.DefAction)
		if sym == nil {
			t.Fatal("action loopn not found")
		}
		_, err := ctx.ExecuteAction(sym)
		return err
	}

	t.Run("old_default_stops_it", func(t *testing.T) {
		err := runLoop(t, 100000)
		if err == nil {
			t.Fatal("expected a budget of 100000 steps to stop the loop")
		}
		if !errors.Is(err, ErrStepLimitExceeded) {
			t.Fatalf("expected ErrStepLimitExceeded, got %v", err)
		}
	})

	t.Run("current_default_completes_it", func(t *testing.T) {
		if err := runLoop(t, DefaultMaxSteps); err != nil {
			t.Fatalf("loop failed under the default budget: %v", err)
		}
	})
}

// TestActionStepBudgetIsConfigurable: the action executor's token-flow bound
// comes from the context, and its error names the variable that raises it.
func TestActionStepBudgetIsConfigurable(t *testing.T) {
	src := `
		package L {
			action seq {
				attribute i = 0;
				first start;
				action a { assign i := i + 1; }
				action b { assign i := i + 1; }
				done;
				succession first start then a;
				succession first a then b;
				succession first b then done;
			}
		}
	`
	file := parseAndBuild(t, src)

	runSeq := func(t *testing.T, maxActionSteps int64) error {
		idx, _, ctx := buildRuntime(t, "<test>", file)
		ctx.maxActionSteps = maxActionSteps
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "seq", ast.DefAction)
		if sym == nil {
			t.Fatal("action seq not found")
		}
		_, err := ctx.ExecuteAction(sym)
		return err
	}

	if err := runSeq(t, DefaultMaxActionSteps); err != nil {
		t.Fatalf("action failed under the default token-flow budget: %v", err)
	}

	err := runSeq(t, 1)
	if err == nil {
		t.Fatal("expected a token-flow budget of 1 to stop the action")
	}
	if !errors.Is(err, ErrActionStepLimitExceeded) {
		t.Fatalf("expected ErrActionStepLimitExceeded, got %v", err)
	}
	if !strings.Contains(err.Error(), MaxActionStepsEnvVar) {
		t.Errorf("error %q does not name %s", err, MaxActionStepsEnvVar)
	}
}

// TestStateBudgetsAreConfigurable: the event and do action bounds come from
// the context too, each naming its own variable.
func TestStateBudgetsAreConfigurable(t *testing.T) {
	// A machine whose state is re-entered every round never settles, so whichever
	// bound is reached first stops it.
	machine := func() *ast.Usage {
		return &ast.Usage{
			Kind:  ast.UsageState,
			Ident: ast.Identification{Name: "Machine"},
			Members: []ast.Node{
				entryStart("init"),
				&ast.StateNode{Name: "init"},
				&ast.StateNode{Name: "spin", Do: []ast.Node{&ast.AssignmentActionNode{
					Target: &ast.QualifiedName{Parts: []ast.NameSegment{{Text: "ticks"}}},
					Value:  &ast.LiteralInteger{Value: "1"},
				}}},
				transitionMember("init", "spin"),
				transitionMember("spin", "spin"),
			},
		}
	}

	tests := []struct {
		name    string
		budgets func(*Context)
		wantVar string
	}{
		{
			name:    "events",
			budgets: func(ctx *Context) { ctx.maxStateEvents = 3 },
			wantVar: MaxStateEventsEnvVar,
		},
		{
			name:    "do_steps",
			budgets: func(ctx *Context) { ctx.maxDoSteps = 1 },
			wantVar: MaxDoStepsEnvVar,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := stateExecutorFor(t, machine())
			tt.budgets(exec.ctx)
			if err := exec.initialize(); err != nil {
				t.Fatalf("initialize: %v", err)
			}
			err := exec.RunToCompletion()
			if err == nil {
				t.Fatal("expected a budget error for a machine that never settles")
			}
			if tt.name == "events" && !errors.Is(err, ErrStateEventLimitExceeded) {
				t.Fatalf("expected ErrStateEventLimitExceeded, got %v", err)
			}
			if tt.name == "do_steps" && !errors.Is(err, ErrDoStepLimitExceeded) {
				t.Fatalf("expected ErrDoStepLimitExceeded, got %v", err)
			}
			if !strings.Contains(err.Error(), tt.wantVar) {
				t.Errorf("error %q does not name %s", err, tt.wantVar)
			}
		})
	}
}

// TestStepBudgetIsPerRun: the budget bounds one run, so a session of many runs
// does not exhaust it, while a runaway inside a single run still trips it and a
// run started from inside another shares the outer one's budget.
func TestStepBudgetIsPerRun(t *testing.T) {
	t.Run("each_run_starts_fresh", func(t *testing.T) {
		src := `part def Simple {}`
		model, resolver, _ := parseAndBuildModel(t, src)
		ctx := NewContext(model, resolver, 4)

		// Far more evaluations than the budget, but one per run.
		for i := 0; i < 100; i++ {
			if _, err := ctx.Eval(&ast.LiteralInteger{Value: "1"}); err != nil {
				t.Fatalf("evaluation %d of its own run failed: %v", i, err)
			}
		}
	})

	t.Run("one_run_still_bounded", func(t *testing.T) {
		src := `
			package L {
				action loopn {
					attribute i = 0;
					first start;
					action go { while i < 10000 { assign i := i + 1; } }
					done;
					succession first start then go;
					succession first go then done;
				}
			}
		`
		file := parseAndBuild(t, src)
		idx, _, ctx := buildRuntime(t, "<test>", file)
		ctx.maxSteps = 100
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "loopn", ast.DefAction)
		if sym == nil {
			t.Fatal("action loopn not found")
		}
		if _, err := ctx.ExecuteAction(sym); !errors.Is(err, ErrStepLimitExceeded) {
			t.Fatalf("expected ErrStepLimitExceeded within one run, got %v", err)
		}
	})

	t.Run("nested_run_shares_the_budget", func(t *testing.T) {
		src := `part def Simple {}`
		model, resolver, _ := parseAndBuildModel(t, src)
		ctx := NewContext(model, resolver, 100)

		// Standing in for an action invoked from an expression: the inner run must
		// not hand the outer one a fresh allowance.
		end := ctx.beginRun()
		defer end()
		for i := 0; i < 10; i++ {
			if _, err := ctx.Eval(&ast.LiteralInteger{Value: "1"}); err != nil {
				t.Fatalf("nested evaluation %d failed: %v", i, err)
			}
		}
		if ctx.run.steps < 10 {
			t.Errorf("nested runs reset the counter: %d steps after 10 evaluations", ctx.run.steps)
		}
	})
}

// TestStepBudgetHoldsAcrossExecutorDrivenRun: a run the caller drives step by
// step - as the REPL's %action debugger does - is one run, so a nested action it
// invokes shares its budget instead of handing it a fresh one.
func TestStepBudgetHoldsAcrossExecutorDrivenRun(t *testing.T) {
	src := `
		package L {
			action outer {
				attribute base = 7;
				attribute result = 0;
				first start;
				perform increment;
				done;
				succession first start then increment;
				succession first increment then done;
			}
			action increment {
				in base;
				out result;
				first begin;
				action bump { assign result := base + 5; }
				done;
				succession first begin then bump;
				succession first bump then done;
			}
		}
	`
	file := parseAndBuild(t, src)

	// Drive the run the way the debugger does, and report what it spent.
	run := func(t *testing.T, maxSteps int64) (int64, error) {
		t.Helper()
		idx, _, ctx := buildRuntime(t, "<test>", file)
		ctx.maxSteps = maxSteps
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
		if sym == nil {
			t.Fatal("action outer not found")
		}
		exec, err := newActionExecutor(ctx, sym, nil)
		if err != nil {
			t.Fatalf("newActionExecutor: %v", err)
		}
		if err := exec.initialize(); err != nil {
			t.Fatalf("initialize: %v", err)
		}
		err = exec.RunToCompletion()
		return ctx.run.steps, err
	}

	// The same action run in one call, whose budget the nested invocation
	// demonstrably shares: what it spends is what the run costs.
	idx, _, ctx := buildRuntime(t, "<test>", file)
	ctx.maxSteps = DefaultMaxSteps
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "outer", ast.DefAction)
	if sym == nil {
		t.Fatal("action outer not found")
	}
	if _, err := ctx.ExecuteAction(sym); err != nil {
		t.Fatalf("ExecuteAction: %v", err)
	}
	cost := ctx.run.steps

	spent, err := run(t, DefaultMaxSteps)
	if err != nil {
		t.Fatalf("run failed under the default budget: %v", err)
	}
	if spent != cost {
		t.Fatalf("the driven run spent %d steps against %d for the same action: the nested invocation reset the counter", spent, cost)
	}

	// One step short of what the run costs: the nested invocation must not hand it
	// a fresh allowance.
	if _, err := run(t, cost-1); !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("expected ErrStepLimitExceeded one step short of the run's cost, got %v", err)
	}
}

// TestStepBudgetIsPerRunForInstancesAndCalcs: instantiating and invoking a calc
// are runs of their own too, so a session of them does not exhaust the budget.
func TestStepBudgetIsPerRunForInstancesAndCalcs(t *testing.T) {
	src := `
		package L {
			part def P {
				attribute n = 1;
				attribute m = n + 1;
			}
			calc twice { in x; return : Real = x * 2; }
		}
	`
	file := parseAndBuild(t, src)
	idx, _, ctx := buildRuntime(t, "<test>", file)
	ctx.maxSteps = 4

	scope := idx.DocumentRoot("<test>")
	partSym := findSymbolByName(scope, "P", ast.DefPart)
	if partSym == nil {
		t.Fatal("part def P not found")
	}
	calcSym := findSymbolByName(scope, "twice", ast.DefCalc)
	if calcSym == nil {
		t.Fatal("calc twice not found")
	}

	for i := 0; i < 100; i++ {
		inst, err := ctx.Instantiate(partSym)
		if err != nil {
			t.Fatalf("instantiation %d failed: %v", i, err)
		}
		if _, err := inst.GetFeatureValue(ctx, "m"); err != nil {
			t.Fatalf("feature value read %d failed: %v", i, err)
		}
		args := []Value{{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 21}}}
		if _, err := ctx.InvokeCalc(calcSym, args, scope); err != nil {
			t.Fatalf("calc invocation %d failed: %v", i, err)
		}
	}
}
