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
)

// TestRuntimeRobustnessExploreBodyInterleavings exercises the scheduler boundaries
// inside a leaf body: another performance may run between a body's start shot and
// its assignment, and every surface that enumerates or replays schedules reaches it.
func TestRuntimeRobustnessExploreBodyInterleavings(t *testing.T) {
	t.Run("explore_reaches_every_admitted_outcome", testBodyInterleavingsExplored)
	t.Run("budget_exhaustion_is_reported", testBodyInterleavingsBudgetHit)
	t.Run("recorded_interleaving_replays", testBodyInterleavingsReplay)
	t.Run("seeded_runs_are_reproducible_and_reach_the_lost_update", testBodyInterleavingsSeeded)
	t.Run("declared_and_reverse_keep_their_results", testBodyInterleavingsFixedPolicies)
	t.Run("checker_finds_every_outcome", testBodyInterleavingsChecked)
	t.Run("callee_executors_interleave", testBodyInterleavingsCallees)
	t.Run("callees_touching_only_their_own_features_run_whole", testBodyInterleavingsOwnCallees)
	t.Run("callee_output_writes_are_observed_one_at_a_time", testBodyInterleavingsCalleeOutputs)
}

// caseRun runs the case's one action under ctx and reports its outcome.
func caseRun(t *testing.T, m *exploreModel, name string) func(*Context) (Outcome, error) {
	t.Helper()
	sym := m.action(t, name)
	return func(ctx *Context) (Outcome, error) {
		outputs, err := ctx.ExecuteAction(sym)
		if err != nil {
			return Outcome{}, err
		}
		return ctx.ActionOutcome(outputs), nil
	}
}

// featureValues spells feature in each outcome, in outcome order.
func featureValues(t *testing.T, x *Exploration, feature string) []string {
	t.Helper()
	var values []string
	for _, o := range x.Outcomes {
		values = append(values, outcomeValue(t, o.Outcome, feature))
	}
	return values
}

func outcomeValue(t *testing.T, o Outcome, feature string) string {
	t.Helper()
	if o.Err != nil {
		t.Fatalf("outcome is an error: %v", o.Err)
	}
	v, ok := o.Outputs[feature]
	if !ok {
		t.Fatalf("outcome %s has no %s", o, feature)
	}
	return FormatValue(v)
}

func testBodyInterleavingsExplored(t *testing.T) {
	for _, c := range []struct {
		fixture, action, feature string
		want                     []string
	}{
		{"action_explore_body_lost_update", "Race", "c", []string{"1", "2"}},
		{"action_explore_body_three_way", "Race3", "c", []string{"1", "2", "3"}},
		{"action_explore_body_fork_lost_update", "ForkPlain", "c", []string{"1", "2"}},
		{"action_explore_body_ordered_substeps", "Ordered", "log", []string{`"12b"`, `"1b2"`, `"b12"`}},
		{"action_step_multiplicity_single_assignment", "Single", "c", []string{"3"}},
		{"action_explore_body_guard_branch", "GuardBranch", "c", []string{"1", "2"}},
	} {
		t.Run(c.action, func(t *testing.T) {
			x := conformanceModel(t, c.fixture).exploreAction(t, "explore", c.action)
			if !x.Complete() {
				t.Fatalf("exploration %s, want complete", x.Status())
			}
			got := featureValues(t, x, c.feature)
			slices.Sort(got)
			got = slices.Compact(got)
			if !slices.Equal(got, c.want) {
				t.Fatalf("%s over every schedule = %v, want %v", c.feature, got, c.want)
			}
		})
	}
}

func testBodyInterleavingsBudgetHit(t *testing.T) {
	m := conformanceModel(t, "action_explore_body_three_way")
	x := m.exploreAction(t, "explore:runs=3", "Race3")
	if x.Complete() || !slices.Contains(x.BudgetsHit, "runs") || x.Runs != 3 {
		t.Fatalf("exploration %s after %d runs, want the runs budget of 3 hit", x.Status(), x.Runs)
	}
	if !strings.Contains(x.Status(), "runs budget 3") {
		t.Errorf("status %q does not name the runs budget", x.Status())
	}
	x = m.exploreAction(t, "explore:depth=1", "Race3")
	if x.Complete() || !slices.Contains(x.BudgetsHit, "depth") {
		t.Fatalf("exploration %s, want the depth budget hit", x.Status())
	}
	if !strings.Contains(x.Status(), "depth budget 1") {
		t.Errorf("status %q does not name the depth budget", x.Status())
	}
}

func testBodyInterleavingsReplay(t *testing.T) {
	m := conformanceModel(t, "action_explore_body_lost_update")
	run := caseRun(t, m, "Race")
	x := m.exploreAction(t, "explore", "Race")
	i := slices.IndexFunc(x.Outcomes, func(o ExploredOutcome) bool { return outcomeValue(t, o.Outcome, "c") == "1" })
	if i < 0 {
		t.Fatalf("exploration reached %v, want the lost update c = 1", outcomeTexts(x))
	}
	witness := x.Outcomes[i].Witness
	recorded := FormatChoices(witness)
	parsed, err := ParseChoices(recorded)
	if err != nil {
		t.Fatalf("parse recorded witness %q: %v", recorded, err)
	}
	outcome, taken, err := replayed(t, m.fresh, run, parsed)
	if err != nil {
		t.Fatalf("replay %q: %v", recorded, err)
	}
	if got := outcomeValue(t, outcome, "c"); got != "1" {
		t.Fatalf("replaying %q gave c = %s, want 1", recorded, got)
	}
	if again := FormatChoices(taken); again != recorded {
		t.Errorf("replay took %q, want the witness %q", again, recorded)
	}
}

func testBodyInterleavingsSeeded(t *testing.T) {
	m := conformanceModel(t, "action_explore_body_lost_update")
	run := caseRun(t, m, "Race")
	seen := map[string]bool{}
	for seed := 1; seed <= 32; seed++ {
		spelling := fmt.Sprintf("seed:%d", seed)
		var first string
		for range 2 {
			ctx, err := m.fresh()
			if err != nil {
				t.Fatal(err)
			}
			mustSchedule(t, ctx, mustPolicy(t, spelling))
			outcome, err := run(ctx)
			if err != nil {
				t.Fatalf("%s: %v", spelling, err)
			}
			got := outcomeValue(t, outcome, "c")
			if first == "" {
				first = got
			} else if got != first {
				t.Fatalf("%s gave c = %s, then c = %s; want the seed to replay its run", spelling, first, got)
			}
		}
		seen[first] = true
	}
	if !seen["1"] || !seen["2"] {
		t.Fatalf("seeds 1..32 reached %v, want both c = 1 and c = 2", seen)
	}
}

func testBodyInterleavingsFixedPolicies(t *testing.T) {
	m := conformanceModel(t, "action_explore_body_lost_update")
	run := caseRun(t, m, "Race")
	for _, spelling := range []string{"reverse", "declared"} {
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		mustSchedule(t, ctx, mustPolicy(t, spelling))
		outcome, err := run(ctx)
		if err != nil {
			t.Fatalf("%s: %v", spelling, err)
		}
		if got := outcomeValue(t, outcome, "c"); got != "2" {
			t.Errorf("%s gave c = %s, want 2, each body run whole", spelling, got)
		}
	}
}

func testBodyInterleavingsChecked(t *testing.T) {
	m := conformanceModel(t, "action_explore_body_fork_lost_update")
	for _, opts := range []CheckOptions{reduced(), unreduced()} {
		report, err := Check(context.Background(), m.fresh, starterOf(m.action(t, "ForkPlain")), CheckBudget{}, opts, nil)
		if err != nil {
			t.Fatalf("reduce=%v: %v", opts.Reduce, err)
		}
		if len(report.BoundsHit) != 0 {
			t.Fatalf("reduce=%v: bounds hit %v, want exhaustive", opts.Reduce, report.BoundsHit)
		}
		if got := divergentValues(report, "c"); !slices.Equal(got, []string{"1", "2"}) {
			t.Errorf("reduce=%v: c diverges over %v, want [1 2]", opts.Reduce, got)
		}
	}
}

// testBodyInterleavingsCallees checks that the performances a flow invokes, each
// in an executor of its own, interleave inside their bodies under explore and check,
// and that a fixed policy still runs each whole.
func testBodyInterleavingsCallees(t *testing.T) {
	for _, c := range []struct{ fixture, action string }{
		{"action_explore_body_typed_callees", "TypedCallees"},
		{"action_explore_body_performed_callees", "PerformedCallees"},
	} {
		t.Run(c.action, func(t *testing.T) {
			m := conformanceModel(t, c.fixture)
			x := m.exploreAction(t, "explore", c.action)
			if !x.Complete() {
				t.Fatalf("exploration %s, want complete", x.Status())
			}
			got := featureValues(t, x, "seen")
			slices.Sort(got)
			if got = slices.Compact(got); !slices.Equal(got, []string{"1", "2"}) {
				t.Fatalf("seen over every schedule = %v, want [1 2]", got)
			}
			report, err := Check(context.Background(), m.fresh, starterOf(m.action(t, c.action)), CheckBudget{}, reduced(), nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := divergentValues(report, "seen"); !slices.Equal(got, []string{"1", "2"}) {
				t.Errorf("check: seen diverges over %v, want [1 2]", got)
			}
			run := caseRun(t, m, c.action)
			seeded := map[string]bool{}
			for seed := 1; seed <= 32; seed++ {
				ctx, err := m.fresh()
				if err != nil {
					t.Fatal(err)
				}
				mustSchedule(t, ctx, mustPolicy(t, fmt.Sprintf("seed:%d", seed)))
				outcome, err := run(ctx)
				if err != nil {
					t.Fatalf("seed:%d: %v", seed, err)
				}
				seeded[outcomeValue(t, outcome, "seen")] = true
			}
			if !seeded["1"] || !seeded["2"] || len(seeded) != 2 {
				t.Errorf("seeds 1-32 gave seen in %v, want both 1 and 2", seeded)
			}
			for _, spelling := range []string{"reverse", "declared"} {
				ctx, err := m.fresh()
				if err != nil {
					t.Fatal(err)
				}
				mustSchedule(t, ctx, mustPolicy(t, spelling))
				outcome, err := run(ctx)
				if err != nil {
					t.Fatalf("%s: %v", spelling, err)
				}
				if got := outcomeValue(t, outcome, "seen"); got != "2" {
					t.Errorf("%s gave seen = %s, want 2, each callee run whole", spelling, got)
				}
			}
		})
	}
}

func testBodyInterleavingsOwnCallees(t *testing.T) {
	m := conformanceModel(t, "action_explore_body_own_callees")
	x := m.exploreAction(t, "explore", "OwnCallees")
	if !x.Complete() {
		t.Fatalf("exploration %s, want complete", x.Status())
	}
	got := featureValues(t, x, "sum")
	if got = slices.Compact(got); !slices.Equal(got, []string{"5"}) {
		t.Fatalf("sum over every schedule = %v, want [5]", got)
	}
	if runs := x.Runs; runs > 2 {
		t.Errorf("explore took %d runs, want at most 2: the callees share nothing", runs)
	}
}

func testBodyInterleavingsCalleeOutputs(t *testing.T) {
	text, err := os.ReadFile(filepath.Join("testdata", "robustness", "action_explore_body_callee_outputs.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	m := parseExploreModel(t, string(text))
	x := m.exploreAction(t, "explore", "CalleeOutputs")
	if !x.Complete() {
		t.Fatalf("exploration %s, want complete", x.Status())
	}
	var got []string
	errs := 0
	for _, o := range x.Outcomes {
		if o.Outcome.Err != nil {
			if !errors.Is(o.Outcome.Err, ErrNodeNotPerformed) {
				t.Errorf("outcome error %v, want only %v", o.Outcome.Err, ErrNodeNotPerformed)
			}
			errs++
			continue
		}
		got = append(got, outcomeValue(t, o.Outcome, "seen"))
	}
	slices.Sort(got)
	if !slices.Equal(got, []string{"0", "1", "2"}) || errs != 1 {
		t.Fatalf("seen over every schedule = %v with %d error outcomes, want [0 1 2] and the read before producer is performed", got, errs)
	}
}
