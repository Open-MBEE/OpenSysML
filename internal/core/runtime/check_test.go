package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// checkModel checks the action named in the model under the options given.
func checkModel(t *testing.T, m *exploreModel, name string, budget CheckBudget, opts CheckOptions, props ...CheckProperty) *CheckReport {
	t.Helper()
	report, err := checkModelErr(t, m, name, budget, opts, props...)
	if err != nil {
		t.Fatalf("check %s: %v", name, err)
	}
	return report
}

func checkModelErr(t *testing.T, m *exploreModel, name string, budget CheckBudget, opts CheckOptions, props ...CheckProperty) (*CheckReport, error) {
	t.Helper()
	sym := m.action(t, name)
	return CheckAction(context.Background(), m.fresh, starterOf(sym), budget, opts, props)
}

func starterOf(sym *symbols.Symbol) ActionStarter {
	return func(ctx *Context) (*ActionExecutor, error) {
		return ctx.CreateActionExecutor(sym)
	}
}

func reduced() CheckOptions   { return CheckOptions{Reduce: true} }
func unreduced() CheckOptions { return CheckOptions{} }

func conformanceModel(t *testing.T, name string) *exploreModel {
	t.Helper()
	text, err := os.ReadFile(filepath.Join("testdata", "conformance", name+".sysml"))
	if err != nil {
		t.Fatal(err)
	}
	return parseExploreModel(t, string(text))
}

func divergentValues(report *CheckReport, feature string) []string {
	for _, d := range report.Divergent {
		if d.Feature == feature {
			values := make([]string, len(d.Values))
			for i, v := range d.Values {
				values[i] = v.Value
			}
			return values
		}
	}
	return nil
}

// The oracle: a join waits for its slowest branch, so `arrived` is 3 on every
// schedule and nothing diverges.
func TestCheckJoinWaitsForSlowestBranch(t *testing.T) {
	m := conformanceModel(t, "action_join_waits_for_slowest_branch")
	for _, opts := range []CheckOptions{reduced(), unreduced()} {
		report := checkModel(t, m, "gather", CheckBudget{}, opts)
		if report.Verdict != CheckExhaustive {
			t.Fatalf("reduce=%v: verdict %s, violations %v", opts.Reduce, report.Status(), report.Violations)
		}
		if len(report.Finals) != 1 {
			t.Fatalf("reduce=%v: %d finals, want 1: %+v", opts.Reduce, len(report.Finals), report.Finals)
		}
		if got := report.Finals[0].Values["arrived"]; got != "3" {
			t.Fatalf("reduce=%v: arrived = %s, want 3", opts.Reduce, got)
		}
		if got := report.Finals[0].Values["seen"]; got != "3" {
			t.Fatalf("reduce=%v: seen = %s, want 3", opts.Reduce, got)
		}
	}
}

// The oracle: two branches writing one feature leave it with either value;
// the flags each branch sets alone do not diverge.
func TestCheckForkBranchesWriteOneFeatureDiverge(t *testing.T) {
	m := conformanceModel(t, "action_fork_branches_write_one_feature")
	for _, opts := range []CheckOptions{reduced(), unreduced()} {
		report := checkModel(t, m, "clash", CheckBudget{}, opts)
		if report.Verdict != CheckDivergent {
			t.Fatalf("reduce=%v: verdict %s, violations %v", opts.Reduce, report.Status(), report.Violations)
		}
		if got := divergentValues(report, "x"); strings.Join(got, ",") != "1,2" {
			t.Fatalf("reduce=%v: x diverges over %v, want [1 2]", opts.Reduce, got)
		}
		for _, flag := range []string{"leftRan", "rightRan"} {
			if got := divergentValues(report, flag); got != nil {
				t.Fatalf("reduce=%v: %s diverges over %v, want not", opts.Reduce, flag, got)
			}
		}
		if len(report.Finals) != 2 {
			t.Fatalf("reduce=%v: %d finals, want 2", opts.Reduce, len(report.Finals))
		}
		for _, final := range report.Finals {
			if len(final.Witness.Choices) == 0 || final.Witness.Trace == "" {
				t.Fatalf("reduce=%v: final %s has no witness", opts.Reduce, final.Outcome)
			}
		}
	}
}

// A completed state is a stable state: a property false only there is a
// violation on the schedule completing, at the depth that completes it.
func TestCheckEvaluatesPropertiesAtCompletion(t *testing.T) {
	m := conformanceModel(t, "action_join_waits_for_slowest_branch")
	incomplete := CheckProperty{Name: "incomplete", Holds: func(_ *Context, exec *ActionExecutor) (bool, error) {
		return exec.State() != StateCompleted, nil
	}}
	for _, opts := range []CheckOptions{reduced(), unreduced()} {
		report := checkModel(t, m, "gather", CheckBudget{}, opts, incomplete)
		if report.Verdict != CheckViolation || len(report.Violations) != 1 {
			t.Fatalf("reduce=%v: %s, violations %v; want the one at completion", opts.Reduce, report.Status(), report.Violations)
		}
		v := report.Violations[0]
		if v.Kind != ViolationProperty || v.Name != "incomplete" || v.Depth != report.MaxDepth || len(v.Witness.Choices) == 0 {
			t.Fatalf("reduce=%v: violation %+v, want incomplete false at the completing depth %d", opts.Reduce, v, report.MaxDepth)
		}
		r := replayWitness(t, m, starterOf(m.action(t, "gather")), v.Witness, "completion")
		if r.Err != nil || r.Exec.State() != StateCompleted {
			t.Fatalf("reduce=%v: the replay ends %s with %v, want complete", opts.Reduce, r.Exec.State(), r.Err)
		}
	}
	// An action complete before any move is the same state.
	single := parseExploreModel(t, `package test {
		action lone { first start; then done; }
	}`)
	report := checkModel(t, single, "lone", CheckBudget{}, reduced(), incomplete)
	if report.Verdict != CheckViolation || len(report.Violations) != 1 || report.Violations[0].Depth != report.MaxDepth {
		t.Fatalf("%s, violations %v; want the one at completion", report.Status(), report.Violations)
	}
}

// Naming the features to report narrows divergence to them.
func TestCheckDivergenceOfNamedFeaturesOnly(t *testing.T) {
	m := conformanceModel(t, "action_fork_branches_write_one_feature")
	report := checkModel(t, m, "clash", CheckBudget{}, CheckOptions{Reduce: true, Diverge: []string{"leftRan"}})
	if report.Verdict != CheckExhaustive {
		t.Fatalf("verdict %s, want exhaustive; divergent %v", report.Status(), report.Divergent)
	}
}

// The reduction explores fewer moves than the full search and reaches the same finals.
func TestCheckReductionExploresFewerMoves(t *testing.T) {
	m := conformanceModel(t, "action_join_waits_for_slowest_branch")
	with := checkModel(t, m, "gather", CheckBudget{}, reduced())
	without := checkModel(t, m, "gather", CheckBudget{}, unreduced())
	if with.Moves >= without.Moves {
		t.Fatalf("reduced search made %d moves, unreduced %d", with.Moves, without.Moves)
	}
	if with.States > without.States {
		t.Fatalf("reduced search visited %d states, unreduced %d", with.States, without.States)
	}
}

// A search stopped by its caller reports how far it got.
func TestCheckStopsWhenCancelled(t *testing.T) {
	m := conformanceModel(t, "action_join_waits_for_slowest_branch")
	stop, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := CheckAction(stop, m.fresh, starterOf(m.action(t, "gather")), CheckBudget{}, reduced(), nil)
	var stopped *CheckStopped
	if !errors.As(err, &stopped) || !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want CheckStopped wrapping context.Canceled", err)
	}
}

// Every witness the check writes replays to the state it claims and leaves its trace.
func TestCheckWitnessesReplay(t *testing.T) {
	for _, c := range []struct{ file, action string }{
		{"action_join_waits_for_slowest_branch", "gather"},
		{"action_fork_branches_write_one_feature", "clash"},
	} {
		m := conformanceModel(t, c.file)
		report := checkModel(t, m, c.action, CheckBudget{}, reduced())
		start := starterOf(m.action(t, c.action))
		for _, final := range report.Finals {
			replayWitness(t, m, start, final.Witness, final.Outcome)
		}
		for _, d := range report.Divergent {
			for _, v := range d.Values {
				replayWitness(t, m, start, v.Witness, d.Feature+" = "+v.Value)
			}
		}
	}
}

func replayWitness(t *testing.T, m *exploreModel, start ActionStarter, w Witness, claim string) *Replayed {
	t.Helper()
	parsed, err := ParseWitness(w.String())
	if err != nil {
		t.Fatalf("%s: parsing the witness: %v", claim, err)
	}
	if parsed.Trace != w.Trace || len(parsed.Choices) != len(w.Choices) {
		t.Fatalf("%s: the witness reads back otherwise:\n%s", claim, w)
	}
	r, err := ReplayAction(context.Background(), m.fresh, start, parsed)
	if err != nil {
		t.Fatalf("%s: replay: %v", claim, err)
	}
	return r
}

// A witness altered to another schedule does not replay: the disagreement is reported.
func TestCheckReplayDisagreesWithATamperedWitness(t *testing.T) {
	m := conformanceModel(t, "action_fork_branches_write_one_feature")
	report := checkModel(t, m, "clash", CheckBudget{}, reduced())
	w := report.Finals[0].Witness
	other := Witness{Choices: report.Finals[1].Witness.Choices, Trace: w.Trace}
	_, err := ReplayAction(context.Background(), m.fresh, starterOf(m.action(t, "clash")), other)
	var dis *ReplayDisagreement
	if !errors.As(err, &dis) || !errors.Is(err, ErrReplayDisagrees) {
		t.Fatalf("replay = %v, want a ReplayDisagreement", err)
	}
}

// A replay whose caller goes away mid-run stops with the caller's error, not a
// disagreement: the context is cancelled once the run is under way, and every
// step of the witness left is skipped.
func TestCheckReplayStopsWhenCancelled(t *testing.T) {
	m := conformanceModel(t, "action_fork_branches_write_one_feature")
	report := checkModel(t, m, "clash", CheckBudget{}, reduced())
	w := report.Finals[0].Witness
	stop, cancel := context.WithCancel(context.Background())
	defer cancel()
	fresh := func() (*Context, error) {
		cancel()
		return m.fresh()
	}
	r, err := ReplayAction(stop, fresh, starterOf(m.action(t, "clash")), w)
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrReplayDisagrees) {
		t.Fatalf("replay = %v, want context.Canceled", err)
	}
	if r == nil || r.Exec == nil || r.Ctx.Trace().String() == w.Trace {
		t.Fatalf("the replay ran to the claimed state after being cancelled")
	}
	if _, err := ReplayAction(stop, m.fresh, starterOf(m.action(t, "clash")), w); !errors.Is(err, context.Canceled) {
		t.Fatalf("replay under a cancelled context = %v, want context.Canceled", err)
	}
}

// A performed action's witness lists the choices of every run of the context: the
// performer's own behaviors run before the action starts, and their choice points
// come first. Each divergent value replays, so `this.<name>` is witnessed.
func TestCheckWitnessesAPerformedActionThroughItsPerformer(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		part def Tank {
			attribute level : Integer = 0;
			perform action fill {
				first start;
				fork split;
				action a { assign this.level := 1; }
				action b { assign this.level := 2; }
				join sync;
				done;
				succession first start then split;
				succession first split then a;
				succession first split then b;
				succession first a then sync;
				succession first b then sync;
				succession first sync then done;
			}
		}
	}`)
	fill := m.idx.LookupQualified("test::Tank::fill")[0]
	tank := m.idx.LookupQualified("test::Tank")[0]
	start := func(ctx *Context) (*ActionExecutor, error) {
		self, err := ctx.Instantiate(tank)
		if err != nil {
			return nil, err
		}
		return ctx.CreateActionExecutorFor(fill, self)
	}
	report, err := CheckAction(context.Background(), m.fresh, start, CheckBudget{}, CheckOptions{Reduce: true, Diverge: []string{"this.level"}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := divergentValues(report, "this.level"); !slices.Equal(got, []string{"1", "2"}) {
		t.Fatalf("this.level diverges over %v, want [1 2]: %s", got, report.Status())
	}
	for _, d := range report.Divergent {
		for _, v := range d.Values {
			if len(v.Witness.Choices) != 2 {
				t.Fatalf("%s = %s: witness %s, want the performer's move and the action's", d.Feature, v.Value, FormatChoices(v.Witness.Choices))
			}
			replayWitness(t, m, start, v.Witness, d.Feature+" = "+v.Value)
		}
	}
}

// A name given to Diverge selects one namespace: `level` the action's own
// attribute, `this.level` the performer's, though both are spelled level.
func TestCheckDivergeNamesTellTheActionsFeaturesFromThePerformers(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		part def Tank {
			attribute level : Integer = 0;
			perform action fill {
				attribute level : Integer = 0;
				first start;
				fork split;
				action a { assign this.level := 1; assign level := 1; }
				action b { assign this.level := 2; assign level := 2; }
				join sync;
				done;
				succession first start then split;
				succession first split then a;
				succession first split then b;
				succession first a then sync;
				succession first b then sync;
				succession first sync then done;
			}
		}
	}`)
	fill := m.idx.LookupQualified("test::Tank::fill")[0]
	tank := m.idx.LookupQualified("test::Tank")[0]
	start := func(ctx *Context) (*ActionExecutor, error) {
		self, err := ctx.Instantiate(tank)
		if err != nil {
			return nil, err
		}
		return ctx.CreateActionExecutorFor(fill, self)
	}
	features := func(report *CheckReport) []string {
		var names []string
		for _, d := range report.Divergent {
			names = append(names, d.Feature)
		}
		return names
	}
	for _, tc := range []struct {
		diverge []string
		want    []string
	}{
		{nil, []string{"level", "this.level"}},
		{[]string{"level"}, []string{"level"}},
		{[]string{"this.level"}, []string{"this.level"}},
		{[]string{"level", "this.level"}, []string{"level", "this.level"}},
	} {
		report, err := CheckAction(context.Background(), m.fresh, start, CheckBudget{}, CheckOptions{Reduce: true, Diverge: tc.diverge}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := features(report); !slices.Equal(got, tc.want) {
			t.Errorf("Diverge %v reports %v divergent, want %v", tc.diverge, got, tc.want)
		}
		for _, name := range tc.want {
			if got := divergentValues(report, name); !slices.Equal(got, []string{"1", "2"}) {
				t.Errorf("Diverge %v: %s diverges over %v, want [1 2]", tc.diverge, name, got)
			}
		}
	}
}

// A constraint of the performer evaluated at every state is a property: the
// schedule reaching a state where it is false is the violation, once, and its
// witness replays to that state.
func TestCheckPropertyOfThePerformerIsWitnessed(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		part def Tank {
			attribute level : Integer = 0;
			constraint low { level < 2 }
			perform action fill {
				first start;
				fork split;
				action a { assign this.level := 1; }
				action b { assign this.level := 2; }
				join sync;
				done;
				succession first start then split;
				succession first split then a;
				succession first split then b;
				succession first a then sync;
				succession first b then sync;
				succession first sync then done;
			}
		}
	}`)
	fill := m.idx.LookupQualified("test::Tank::fill")[0]
	tank := m.idx.LookupQualified("test::Tank")[0]
	low := m.idx.LookupQualified("test::Tank::low")[0]
	start := func(ctx *Context) (*ActionExecutor, error) {
		self, err := ctx.Instantiate(tank)
		if err != nil {
			return nil, err
		}
		return ctx.CreateActionExecutorFor(fill, self)
	}
	prop := CheckProperty{Name: "low", Holds: func(ctx *Context, exec *ActionExecutor) (bool, error) {
		result, err := ctx.CheckConstraintOn(low, tank.Scope, exec.Performer())
		if err != nil && !errors.Is(err, ErrViolated) {
			return false, err
		}
		return result.Holds, nil
	}}
	for _, opts := range []CheckOptions{reduced(), unreduced()} {
		report, err := CheckAction(context.Background(), m.fresh, start, CheckBudget{}, opts, []CheckProperty{prop})
		if err != nil {
			t.Fatal(err)
		}
		if report.Verdict != CheckViolation || len(report.Violations) != 1 {
			t.Fatalf("reduce=%v: %s, violations %v; want low false once", opts.Reduce, report.Status(), report.Violations)
		}
		v := report.Violations[0]
		if v.Kind != ViolationProperty || v.Name != "low" {
			t.Fatalf("reduce=%v: violation %+v, want the property low", opts.Reduce, v)
		}
		r := replayWitness(t, m, start, v.Witness, "low false")
		holds, err := prop.Holds(r.Ctx, r.Exec)
		if err != nil || holds {
			t.Fatalf("reduce=%v: replayed to a state where low = %v, %v; want false", opts.Reduce, holds, err)
		}
	}
}

// The failure modes of the robustness tests are violations on the schedule that
// reaches them, each with its witness, not errors of the search.
func TestCheckReportsFailuresAsViolations(t *testing.T) {
	cases := []struct {
		name, action, src string
		kind              ViolationKind
		err               error
	}{
		{"join starvation", "starve", `package test {
			action starve {
				first start;
				action stranded;
				join sync;
				done;
				succession first start then sync;
				succession first stranded then sync;
				succession first sync then done;
			}
		}`, ViolationDeadlock, ErrActionDeadlock},
		{"unbound parameter", "outer", `package test {
			private import ScalarValues::*;
			action def Adder { in a : Integer; in b : Integer; out sum : Integer; first step; action step { assign sum := a + b; } }
			action adder : Adder;
			action outer {
				attribute a : Integer = 1;
				first start;
				then action run {
					if a > 0 {
						perform adder;
					}
				}
				then done;
			}
		}`, ViolationFailure, ErrUnboundParameter},
		{"dangling succession", "outer", `package test {
			action outer {
				first leg;
				action leg {
					first a;
					action a;
					succession first a then missing;
				}
			}
		}`, ViolationFailure, ErrInvalidActionFlow},
		{"all guards false", "pick", `package test {
			private import ScalarValues::*;
			action pick {
				attribute level : Integer = 5;
				first start;
				action low;
				action high;
				done;
				succession first start then choose;
				succession first low then done;
				succession first high then done;
				decide choose;
				if level > 10 then low;
				if level > 20 then high;
			}
		}`, ViolationFailure, ErrNoEnabledSuccession},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := parseExploreModel(t, c.src)
			report := checkModel(t, m, c.action, CheckBudget{}, reduced())
			if report.Verdict != CheckViolation || len(report.Violations) != 1 {
				t.Fatalf("verdict %s, violations %v; want one violation", report.Status(), report.Violations)
			}
			v := report.Violations[0]
			if v.Kind != c.kind || !errors.Is(v.Err, c.err) {
				t.Fatalf("violation %s (%v), want %s wrapping %v", v.Kind, v.Err, c.kind, c.err)
			}
			if v.Depth > 0 {
				r := replayWitness(t, m, starterOf(m.action(t, c.action)), v.Witness, c.name)
				if !errors.Is(r.Err, c.err) {
					t.Fatalf("the replay ends with %v, want %v", r.Err, c.err)
				}
			}
		})
	}
}

// An accept whose `via` port does not resolve is the routing error on every
// schedule that brings it a message to route, never a deadlock: the checker
// reaches the accept's move both before and after the sibling branch's write.
func TestCheckReportsAnUnresolvedViaPortAsTheRoutingError(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		port def Chan { out attribute v : Integer; }
		part def Listener {
			port out : Chan;
			port other : ~Chan;
			port in : ~Chan = 1 / 0;
			connect out to other;
			action listen {
				attribute got : Integer = 0;
				attribute fed : Boolean = false;
				first start;
				fork f;
				action feed { send 4 via out; }
				action noteFed { assign fed := true; }
				action reader accept v : Integer via in;
				action note { assign got := reader.v; }
				join j;
				done;
				succession first start then f;
				succession first f then feed;
				succession first f then reader;
				succession first feed then noteFed;
				succession first reader then note;
				succession first noteFed then j;
				succession first note then j;
				succession first j then done;
			}
		}
	}`)
	listen := m.idx.LookupQualified("test::Listener::listen")[0]
	part := m.idx.LookupQualified("test::Listener")[0]
	start := func(ctx *Context) (*ActionExecutor, error) {
		self, err := ctx.Instantiate(part)
		if err != nil {
			return nil, err
		}
		return ctx.CreateActionExecutorFor(listen, self)
	}
	for _, opts := range []CheckOptions{reduced(), unreduced()} {
		report, err := CheckAction(context.Background(), m.fresh, start, CheckBudget{}, opts, nil)
		if err != nil {
			t.Fatal(err)
		}
		if report.Verdict != CheckViolation || len(report.Violations) != 2 {
			t.Fatalf("reduce=%v: %s with %d violations, want the routing error on both schedules", opts.Reduce, report.Status(), len(report.Violations))
		}
		for _, v := range report.Violations {
			if v.Kind != ViolationFailure || !errors.Is(v.Err, ErrDivisionByZero) {
				t.Fatalf("reduce=%v: violation %s (%v), want the port's %v", opts.Reduce, v.Kind, v.Err, ErrDivisionByZero)
			}
			if errors.Is(v.Err, ErrAcceptDeadlock) || errors.Is(v.Err, ErrActionDeadlock) {
				t.Fatalf("reduce=%v: the routing failure is reported as a deadlock: %v", opts.Reduce, v.Err)
			}
			r := replayWitness(t, m, start, v.Witness, "accept via in")
			if !errors.Is(r.Err, ErrDivisionByZero) {
				t.Fatalf("reduce=%v: the replay ends with %v, want %v", opts.Reduce, r.Err, ErrDivisionByZero)
			}
		}
	}
}

// A loop through a merge stops at the depth bound, named, without claiming exhaustiveness.
func TestCheckMergeLoopHitsTheDepthBound(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		action spin {
			attribute n : Integer = 0;
			first start;
			merge m;
			action a { assign n := n + 1; }
			succession first start then m;
			succession first m then a;
			succession first a then m;
		}
	}`)
	report := checkModel(t, m, "spin", CheckBudget{Depth: 12}, reduced())
	if report.Verdict != CheckWithinBounds {
		t.Fatalf("verdict %s, want within bounds", report.Status())
	}
	if !slices.Contains(report.BoundsHit, "depth") {
		t.Fatalf("bounds hit %v, want depth", report.BoundsHit)
	}
	if report.MaxDepth != 12 {
		t.Fatalf("max depth %d, want 12", report.MaxDepth)
	}
	bounded := checkModel(t, m, "spin", CheckBudget{States: 5}, reduced())
	if bounded.Verdict != CheckWithinBounds || !slices.Contains(bounded.BoundsHit, "states") || bounded.States != 5 {
		t.Fatalf("%s, want the states bound hit at 5 states", bounded.Status())
	}
	// The executor's own action-step budget is a bound named as that budget, its limit kept.
	limits := DefaultBudgets()
	limits.MaxActionSteps = 9
	fresh := func() (*Context, error) {
		ctx, err := m.fresh()
		if err != nil {
			return nil, err
		}
		return ctx, ctx.SetBudgets(limits)
	}
	stepped, err := CheckAction(context.Background(), fresh, starterOf(m.action(t, "spin")), CheckBudget{}, reduced(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if stepped.Verdict != CheckWithinBounds || !slices.Equal(stepped.BoundsHit, []string{BoundActionSteps}) || stepped.MaxDepth != 9 {
		t.Fatalf("%s, bounds %v, want only %s hit at depth 9", stepped.Status(), stepped.BoundsHit, BoundActionSteps)
	}
	if got, ok := ExecutorBound(BoundActionSteps, stepped.Limits); !ok || got != 9 {
		t.Fatalf("%s limit %d, want 9", BoundActionSteps, got)
	}
}

// A state the depth bound cut under a long schedule is searched again when a
// shorter one reaches it, so what lies within the shorter one's bound is found.
func TestCheckSearchesAgainWhatTheDepthBoundCutFromAShorterWay(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		action detour {
			attribute n : Integer = 0;
			first start;
			decide pick;
			merge m1;
			merge m2;
			merge m3;
			merge meet;
			action bump { assign n := n + 1; }
			done;
			succession first start then pick;
			succession first pick if n >= 0 then m1;
			succession first pick if n >= 0 then meet;
			succession first m1 then m2;
			succession first m2 then m3;
			succession first m3 then meet;
			succession first meet then bump;
			succession first bump then done;
		}
	}`)
	low := CheckProperty{Name: "low", Holds: func(_ *Context, exec *ActionExecutor) (bool, error) {
		return exec.Data()["n"].Const.Int < 1, nil
	}}
	// The short way makes n = 1 after 4 moves, the long way after 7: with a bound
	// between, the long way (searched first) is cut at meet, the short way is not.
	for _, depth := range []int{4, 5, 6} {
		report := checkModel(t, m, "detour", CheckBudget{Depth: depth}, reduced(), low)
		if report.Verdict != CheckViolation || len(report.Violations) != 1 {
			t.Fatalf("depth %d: %s, want the violation the short way reaches", depth, report.Status())
		}
		if v := report.Violations[0]; v.Kind != ViolationProperty || v.Name != "low" || v.Depth != 4 {
			t.Fatalf("depth %d: violation %s at depth %d, want low at depth 4", depth, v, v.Depth)
		}
		if !slices.Contains(report.BoundsHit, "depth") {
			t.Fatalf("depth %d: bounds hit %v, want depth", depth, report.BoundsHit)
		}
	}
	// With room for both ways nothing is cut, and the search is exhaustive.
	report := checkModel(t, m, "detour", CheckBudget{Depth: 8}, reduced(), low)
	if report.Verdict != CheckViolation || len(report.BoundsHit) != 0 {
		t.Fatalf("depth 8: %s, want a violation with no bound hit", report.Status())
	}
}

// Every executor budget the search can exhaust is a bound of its own, named by
// its budget, its limit that budget's.
func TestCheckNamesEachExecutorBudget(t *testing.T) {
	limits := Budgets{MaxSteps: 1, MaxActionSteps: 2, MaxStateEvents: 3, MaxDoSteps: 4, MaxElements: 5}
	cases := []struct {
		err   error
		name  string
		limit int64
	}{
		{ErrStepLimitExceeded, BoundSteps, 1},
		{ErrActionStepLimitExceeded, BoundActionSteps, 2},
		{ErrStateEventLimitExceeded, BoundEvents, 3},
		{budgetExceeded(ErrStateEventLimitExceeded, "behaviors", ErrBehaviorBudget), BoundBehaviors, 3},
		{ErrDoStepLimitExceeded, BoundDoSteps, 4},
		{ErrElementLimitExceeded, BoundElements, 5},
	}
	if len(cases) != len(ExecutorBounds) {
		t.Fatalf("%d cases over %d bounds %v", len(cases), len(ExecutorBounds), ExecutorBounds)
	}
	for _, c := range cases {
		name, ok := boundOf(fmt.Errorf("wrapped: %w", c.err))
		if !ok || name != c.name || !slices.Contains(ExecutorBounds, name) {
			t.Fatalf("%v names %q, want %s among %v", c.err, name, c.name, ExecutorBounds)
		}
		if limit, ok := ExecutorBound(name, limits); !ok || limit != c.limit {
			t.Fatalf("%s limit %d, want %d", name, limit, c.limit)
		}
	}
	if _, ok := boundOf(ErrActionDeadlock); ok {
		t.Fatal("a deadlock is not a bound")
	}
	if _, ok := ExecutorBound("depth", limits); ok {
		t.Fatal("depth is the search's bound, not the executor's")
	}
}

// Parking the last token on the clock is a move like any other: the search
// settles by advancing the clock to the earliest wait, never by a failure.
func TestCheckSettlesTimedBranchesOnTheClock(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import SI::*;
		private import ScalarValues::*;
		action timers {
			attribute x : Integer = 0;
			first start;
			fork split;
			action slow accept after 2 [s];
			action slowWrite { assign x := 1; }
			action fast accept after 1 [s];
			action fastWrite { assign x := 2; }
			join sync;
			done;
			succession first start then split;
			succession first split then slow;
			succession first split then fast;
			succession first slow then slowWrite;
			succession first fast then fastWrite;
			succession first slowWrite then sync;
			succession first fastWrite then sync;
			succession first sync then done;
		}
	}`)
	report := checkModel(t, m, "timers", CheckBudget{}, reduced())
	if report.Verdict != CheckExhaustive {
		t.Fatalf("verdict %s, want exhaustive", report.Status())
	}
	// The slow branch always writes last: no divergence over `x`.
	if got := divergentValues(report, "x"); got != nil {
		t.Fatalf("x diverges over %v, want the slow write to stand on every schedule", got)
	}
	if len(report.Finals) != 1 || report.Finals[0].Values["x"] != "1" {
		t.Fatalf("finals %v, want one with x = 1", report.Finals)
	}
}

// A loop that reaches states already visited ends: the visited set closes it.
func TestCheckVisitedStatesCloseALoop(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		action toggle {
			attribute on : Boolean = false;
			first start;
			merge m;
			action flip { assign on := not on; }
			succession first start then m;
			succession first m then flip;
			succession first flip then m;
		}
	}`)
	report := checkModel(t, m, "toggle", CheckBudget{}, reduced())
	if report.Verdict != CheckExhaustive {
		t.Fatalf("verdict %s, want exhaustive", report.Status())
	}
	// start, then m and flip with `on` each way, the first pass through flip apart.
	if report.States != 7 {
		t.Fatalf("%d states for a two-valued loop, want 7", report.States)
	}
}
