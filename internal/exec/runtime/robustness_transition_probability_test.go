package runtime

import (
	"context"
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// weightedMachine is a state def body importing ScalarValues and Stochastic,
// with the machine starting in a state the transitions under test leave.
func weightedMachine(body string) string {
	return `package test {
	private import ScalarValues::*;
	private import Stochastic::*;
	state def Machine {
		entry; then a;
		state a;
		state b;
		state c;
` + body + `
	}
}`
}

// weightedStateRun lowers, creates and runs the machine of weightedMachine,
// sending it signal first, and answers the first error it reports.
func weightedStateRun(t *testing.T, body, signal string) error {
	t.Helper()
	m := parseLibraryModel(t, weightedMachine(body))
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	exec, err := ctx.CreateStateExecutor(m.state(t, "Machine"))
	if err != nil {
		return err
	}
	if signal != "" {
		exec.SendSignal(signal, nil)
	}
	return exec.RunToCompletion()
}

// TestRuntimeRobustnessTransitionProbability exercises the failure modes of
// @Probability on state transitions: the checks lowering makes of constant
// weights — the range and the sum to 1 over the transitions competing for one
// event — and the checks made of the values read at dispatch, each a typed
// error naming the transition, never a silent renormalization.
func TestRuntimeRobustnessTransitionProbability(t *testing.T) {
	t.Run("negative_weight", testTransitionNegativeWeight)
	t.Run("weight_above_one", testTransitionWeightAboveOne)
	t.Run("weights_not_summing_to_one", testTransitionWeightsNotSummingToOne)
	t.Run("single_weighted_transition_not_weighing_one", testSingleWeightedTransitionNotWeighingOne)
	t.Run("mixed_weighted_and_unweighted_siblings", testTransitionMixedWeightedAndUnweighted)
	t.Run("non_constant_weights_not_summing_at_dispatch", testTransitionWeightsNotSummingAtDispatch)
	t.Run("non_numeric_weight", testTransitionWeightOfNonNumericType)
	t.Run("holding_total_zero_at_dispatch", testTransitionHoldingTotalZero)
	t.Run("lone_enabled_zero_weight", testLoneEnabledZeroWeight)
	t.Run("lone_enabled_weight_above_one", testLoneEnabledWeightAboveOne)
	t.Run("trigger_argument_weight_out_of_range", testTriggerArgumentWeightOutOfRange)
}

// testTransitionNegativeWeight: a probability is in [0, 1], so a constant
// weight below 0 is refused where it is written, naming the transition.
func testTransitionNegativeWeight(t *testing.T) {
	err := libraryStateExecutorError(t, weightedMachine(`
		transition first a accept go then b { @Probability { p = -0.2; } }
		transition first a accept go then c { @Probability { p = 1.2; } }`), "Machine")
	if !errors.Is(err, lower.ErrProbability) || !strings.Contains(err.Error(), "outside 0.0..1.0") {
		t.Fatalf("error = %v, want the out-of-range weight refused", err)
	}
	if !strings.Contains(err.Error(), "1->b") || !strings.Contains(err.Error(), "state a") {
		t.Fatalf("error = %v, want it to name the transition and its state", err)
	}
}

// testTransitionWeightAboveOne: a constant weight above 1 is refused the same way.
func testTransitionWeightAboveOne(t *testing.T) {
	err := libraryStateExecutorError(t, weightedMachine(`
		transition first a accept go then b { @Probability { p = 1.2; } }
		transition first a accept go then c { @Probability { p = -0.2; } }`), "Machine")
	if !errors.Is(err, lower.ErrProbability) || !strings.Contains(err.Error(), "outside 0.0..1.0") {
		t.Fatalf("error = %v, want the out-of-range weight refused", err)
	}
}

// testTransitionWeightsNotSummingToOne: the constant weights of the transitions
// competing for one event must sum to 1; 0.3 and 0.3 are refused at lowering.
func testTransitionWeightsNotSummingToOne(t *testing.T) {
	err := libraryStateExecutorError(t, weightedMachine(`
		transition first a accept go then b { @Probability { p = 0.3; } }
		transition first a accept go then c { @Probability { p = 0.3; } }`), "Machine")
	if !errors.Is(err, lower.ErrProbability) || !strings.Contains(err.Error(), "sum to 0.6, not 1.0") {
		t.Fatalf("error = %v, want the weights' sum refused", err)
	}
	if !strings.Contains(err.Error(), "state a") || !strings.Contains(err.Error(), "accept go") {
		t.Fatalf("error = %v, want it to name the state and the event", err)
	}
}

// testSingleWeightedTransitionNotWeighingOne: the sum rule applies to a group
// of one, so a lone weighted transition whose constant weight is not 1 is refused.
func testSingleWeightedTransitionNotWeighingOne(t *testing.T) {
	err := libraryStateExecutorError(t, weightedMachine(`
		transition first a accept go then b { @Probability { p = 0.5; } }`), "Machine")
	if !errors.Is(err, lower.ErrProbability) || !strings.Contains(err.Error(), "sum to 0.5, not 1.0") {
		t.Fatalf("error = %v, want the lone weight's sum refused", err)
	}
}

// testTransitionMixedWeightedAndUnweighted: every transition competing for one
// event is weighted or none is; a mix has no reading and is refused, naming the
// unweighted one.
func testTransitionMixedWeightedAndUnweighted(t *testing.T) {
	err := libraryStateExecutorError(t, weightedMachine(`
		transition first a accept go then b { @Probability { p = 1.0; } }
		transition first a accept go then c;`), "Machine")
	if !errors.Is(err, lower.ErrProbability) ||
		!strings.Contains(err.Error(), "state a weights 1 of its 2 transitions on accept go") {
		t.Fatalf("error = %v, want the mixed group refused", err)
	}
	if !strings.Contains(err.Error(), "2->c") {
		t.Fatalf("error = %v, want it to name the unweighted transition", err)
	}
}

// testTransitionWeightsNotSummingAtDispatch: weights computed from features are
// read where the guard is, and the group's sum is checked when the dispatch
// draws: pA + pB = 0.5 is a typed ErrBranchWeights, not a renormalization.
func testTransitionWeightsNotSummingAtDispatch(t *testing.T) {
	err := weightedStateRun(t, `
		attribute pA : Real = 0.3;
		attribute pB : Real = 0.2;
		transition first a accept go then b { @Probability { p = pA; } }
		transition first a accept go then c { @Probability { p = pB; } }`, "go")
	if !errors.Is(err, ErrBranchWeights) ||
		!strings.Contains(err.Error(), "state a on accept go: the weights of its transitions sum to 0.5, not 1.0") {
		t.Fatalf("error = %v, want ErrBranchWeights naming the sum 0.5", err)
	}
}

// testTransitionWeightOfNonNumericType: a p bound to a String or Boolean
// feature is a type error of the model, reported where it is written, as it is
// for a decision's successions.
func testTransitionWeightOfNonNumericType(t *testing.T) {
	for _, tc := range []struct{ typ, value, want string }{
		{"String", `"often"`, "cannot bind String value to a feature typed by Real"},
		{"Boolean", "true", "cannot bind Boolean value to a feature typed by Real"},
	} {
		src := weightedMachine(`
			attribute w : ` + tc.typ + ` = ` + tc.value + `;
			transition first a accept go then b { @Probability { p = w; } }
			transition first a accept go then c { @Probability { p = 0.5; } }`)
		file := parseAndBuild(t, src)
		idx := libs.NewModelIndex()
		idx.AddDocument("<test>", file)
		idx.ExpandWildcardImports()
		var refusals []string
		for _, d := range passes.Analyze("<test>", file, nil, idx) {
			if d.Severity == diag.SeverityError {
				refusals = append(refusals, d.Message)
			}
		}
		if len(refusals) != 1 || !strings.Contains(refusals[0], tc.want) {
			t.Errorf("%s weight: errors %v, want one saying %q", tc.typ, refusals, tc.want)
		}
	}
}

// testTransitionHoldingTotalZero: a distribution is checked whole, but the draw
// is over the enabled transitions' weights: two enabled transitions each
// weighing 0 leave nothing to draw, which is a typed error rather than a pick
// of the guarded-out sibling's 1.
func testTransitionHoldingTotalZero(t *testing.T) {
	err := weightedStateRun(t, `
		attribute ready : Boolean = false;
		transition first a accept go then b { @Probability { p = 0.0; } }
		transition first a accept go then c { @Probability { p = 0.0; } }
		transition first a accept go if ready then b { @Probability { p = 1.0; } }`, "go")
	if !errors.Is(err, ErrBranchWeights) ||
		!strings.Contains(err.Error(), "no holding branch has a positive weight") {
		t.Fatalf("error = %v, want ErrBranchWeights for the zero total", err)
	}
}

// An explore of a weighted dispatch enumerates both weighted alternatives, and
// each witness replays to the state the draw picked.
func TestExploreEnumeratesWeightedTransitions(t *testing.T) {
	m := parseLibraryModel(t, weightedMachine(`
		transition first a accept go then b { @Probability { p = 0.4; } }
		transition first a accept go then c { @Probability { p = 0.6; } }`))
	sym := m.state(t, "Machine")
	run := stateRun(sym, "go")
	x, err := Explore(context.Background(), mustPolicy(t, "explore"), m.fresh, run)
	if err != nil || !x.Complete() || x.Runs != 2 {
		t.Fatalf("explore: %v, %v", x, err)
	}
	finals := map[string]bool{}
	for _, o := range x.Outcomes {
		finals[o.Outcome.FinalState] = true
		if len(o.Witness) != 1 || o.Witness[0].Kind != ChoiceTransition || !o.Witness[0].Weighted() {
			t.Fatalf("witness of %s is %s, want one weighted transition draw", o.Outcome, FormatChoices(o.Witness))
		}
	}
	if !finals["b"] || !finals["c"] {
		t.Fatalf("outcomes reach %v, want both weighted alternatives", finals)
	}
	assertWitnessesReplay(t, x, m.fresh, run)
}

// An explore of a nested weighted dispatch enumerates both alternatives of the
// innermost state's weighted pair — the enclosing state's transition on the
// same event is outranked, not weighed against — and each witness replays to
// the configuration the draw picked.
func TestExploreEnumeratesNestedWeightedTransitions(t *testing.T) {
	m := parseLibraryModel(t, `package test {
	private import ScalarValues::*;
	private import Stochastic::*;
	state def Machine {
		entry; then work;
		state work parallel {
			state left {
				entry; then l1;
				state l1;
				state l2;
				state l3;
				transition first l1 accept go then l2 { @Probability { p = 0.4; } }
				transition first l1 accept go then l3 { @Probability { p = 0.6; } }
			}
			state right {
				entry; then r1;
				state r1;
				state r2;
				transition first r1 accept go then r2;
			}
		}
		state escaped;
		transition first work accept go then escaped;
	}
}`)
	sym := m.state(t, "Machine")
	run := stateRun(sym, "go")
	x, err := Explore(context.Background(), mustPolicy(t, "explore"), m.fresh, run)
	if err != nil || !x.Complete() {
		t.Fatalf("explore: %v, %v", x, err)
	}
	finals := map[string]bool{}
	for _, o := range x.Outcomes {
		finals[o.Outcome.FinalState] = true
	}
	if !finals["l2+r2"] || !finals["l3+r2"] || finals["escaped"] {
		t.Fatalf("outcomes reach %v, want l2+r2 and l3+r2 and never escaped", finals)
	}
	// The notes a nested firing records list the region's exit order ahead of
	// the transition it fired, weighted or not; what a replay must follow is
	// the witness's weighted pick, which reaching its outcome proves.
	for _, o := range x.Outcomes {
		outcome, choices, err := replayed(t, m.fresh, run, o.Witness)
		if err != nil {
			t.Fatalf("%s: replaying %s: %v", o.Outcome, FormatChoices(o.Witness), err)
		}
		if outcome.String() != o.Outcome.String() {
			t.Fatalf("replaying %s reached %s, want %s", FormatChoices(o.Witness), outcome, o.Outcome)
		}
		followed := slices.ContainsFunc(choices, func(c ChoiceTaken) bool {
			return c.Kind == ChoiceTransition && c.Weighted()
		})
		if !followed {
			t.Fatalf("replaying %s made no weighted transition pick: %s", FormatChoices(o.Witness), FormatChoices(choices))
		}
	}
}

// Explore still enumerates every weighted alternative and each outcome carries
// the product of its run's pick shares: the stated weight for a weighted pick.
func TestExploreReportsTransitionProbabilities(t *testing.T) {
	m := parseLibraryModel(t, weightedMachine(`
		transition first a accept go then b { @Probability { p = 0.3; } }
		transition first a accept go then c { @Probability { p = 0.7; } }
	`))
	sym := m.state(t, "Machine")
	x, err := Explore(context.Background(), mustPolicy(t, "explore"), m.fresh, stateRun(sym, "go"))
	if err != nil || !x.Complete() || len(x.Outcomes) != 2 {
		t.Fatalf("explore: %v, %v", x, err)
	}
	probs := map[string]float64{}
	for _, o := range x.Outcomes {
		probs[o.Outcome.FinalState] += o.Probability
	}
	if math.Abs(probs["b"]-0.3) > 1e-9 || math.Abs(probs["c"]-0.7) > 1e-9 {
		t.Errorf("probabilities %v, want b=0.3, c=0.7", probs)
	}
	if p := x.Probability(); math.Abs(p-1) > 1e-9 || x.ProbabilitiesBounded() {
		t.Errorf("a complete exploration covers %v, want 1 exact", p)
	}
}

// A check of a machine whose dispatch is a weighted pick charges each violation
// the share of the path that reaches it: 0.3 for the 0.3-weighted transition.
func TestCheckWeightsViolationMass(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		private import Stochastic::*;
		state def Machine {
			entry; then decide;
			state decide;
			state bad;
			state good;
			state later;
			transition first decide then bad { @Probability { p = 0.3; } }
			transition first decide then good { @Probability { p = 0.7; } }
			transition first good then later;
		}
	}`)
	inv := invocationOf(nil, []*symbols.Symbol{m.state(t, "Machine")})
	prop := CheckProperty{Name: "notBad", Holds: func(_ *Context, inv *Invocation) (bool, error) {
		return inv.States[0].FinalStateName() != "bad", nil
	}}

	report, err := Check(context.Background(), m.fresh, inv, CheckBudget{}, CheckOptions{}, []CheckProperty{prop})
	if err != nil || report.Verdict != CheckViolation || len(report.Violations) != 1 {
		t.Fatalf("check: %v, %v", report, err)
	}
	v := report.Violations[0]
	if v.Name != "notBad" || math.Abs(v.Mass-0.3) > 1e-9 {
		t.Errorf("violation %s carries mass %v, want 0.3", v.Name, v.Mass)
	}
	if report.MassBounded {
		t.Errorf("a tree-shaped search reports its masses as lower bounds")
	}

	bounded, err := Check(context.Background(), m.fresh, inv, CheckBudget{Depth: 1}, CheckOptions{}, []CheckProperty{prop})
	if err != nil || bounded.Verdict != CheckViolation || len(bounded.Violations) != 1 {
		t.Fatalf("bounded check: %v, %v", bounded, err)
	}
	if !bounded.MassBounded || len(bounded.BoundsHit) == 0 {
		t.Errorf("a depth cut reports massLowerBound=%v bounds=%v, want the bound hit", bounded.MassBounded, bounded.BoundsHit)
	}
}

// testLoneEnabledZeroWeight: a guard leaving only a zero-weighted transition
// enabled leaves the draw nothing to take — a typed error even though no choice
// point is recorded.
func testLoneEnabledZeroWeight(t *testing.T) {
	err := weightedStateRun(t, `
		attribute w : Real = 1.0;
		transition first a accept go if w < 0.5 then b { @Probability { p = w; } }
		transition first a accept go then c { @Probability { p = 1.0 - w; } }`, "go")
	if !errors.Is(err, ErrBranchWeights) ||
		!strings.Contains(err.Error(), "no holding branch has a positive weight") {
		t.Fatalf("error = %v, want ErrBranchWeights for the lone enabled zero weight", err)
	}
}

// testLoneEnabledWeightAboveOne: a lone enabled transition is validated too —
// a dynamic weight outside [0, 1] is a typed error, not a transition taken at
// face value.
func testLoneEnabledWeightAboveOne(t *testing.T) {
	err := weightedStateRun(t, `
		attribute w : Real = 1.5;
		transition first a accept go if w < 1.0 then b { @Probability { p = 2.0 - w; } }
		transition first a accept go then c { @Probability { p = w; } }`, "go")
	if !errors.Is(err, ErrBranchWeights) ||
		!strings.Contains(err.Error(), "not a probability in [0, 1]") {
		t.Fatalf("error = %v, want ErrBranchWeights for the lone enabled weight 1.5", err)
	}
}

// testTriggerArgumentWeightOutOfRange: a weight reading the call's bound
// argument is judged by the value the invocation carried — 1.5 is a typed
// error, the same as a literal out of range.
func testTriggerArgumentWeightOutOfRange(t *testing.T) {
	m := parseLibraryModel(t, weightedMachine(`
		transition first a accept route(priority) then b { @Probability { p = priority; } }
		transition first a accept route(priority) then c { @Probability { p = 1.0 - priority; } }`))
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	exec, err := ctx.CreateStateExecutor(m.state(t, "Machine"))
	if err != nil {
		t.Fatal(err)
	}
	exec.InvokeOperation("route", map[string]Value{"priority": constReal(1.5)})
	err = exec.RunToCompletion()
	if !errors.Is(err, ErrBranchWeights) ||
		!strings.Contains(err.Error(), "not a probability in [0, 1]") {
		t.Fatalf("error = %v, want ErrBranchWeights for the bound weight 1.5", err)
	}
}

// TestExploreWeighsTransitionsByTriggerArguments: the weight expressions read
// the arguments the triggering call carried — a route at priority 0.25 draws
// the 0.75 branch three times as often, which explore reports back.
func TestExploreWeighsTransitionsByTriggerArguments(t *testing.T) {
	m := parseLibraryModel(t, weightedMachine(`
		transition first a accept route(priority) then b { @Probability { p = priority; } }
		transition first a accept route(priority) then c { @Probability { p = 1.0 - priority; } }`))
	sym := m.state(t, "Machine")
	x, err := Explore(context.Background(), mustPolicy(t, "explore"), m.fresh, func(ctx *Context) (Outcome, error) {
		exec, err := ctx.CreateStateExecutor(sym)
		if err != nil {
			return Outcome{}, err
		}
		exec.InvokeOperation("route", map[string]Value{"priority": constReal(0.25)})
		if err := exec.RunToCompletion(); err != nil {
			return Outcome{}, err
		}
		return exec.Outcome(), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	probs := make(map[string]float64)
	for _, o := range x.Outcomes {
		probs[o.Outcome.FinalState] = o.Probability
	}
	if math.Abs(probs["b"]-0.25) > 1e-9 || math.Abs(probs["c"]-0.75) > 1e-9 {
		t.Errorf("probabilities %v, want b=0.25 c=0.75 from the call's priority", probs)
	}
	if math.Abs(x.Probability()-1) > 1e-9 || x.ProbabilitiesBounded() {
		t.Errorf("a complete exploration's probabilities sum to %v, want 1 unbounded", x.Probability())
	}
}

// Two transitions off one `after` spelling arm one timer: its expiry is the
// single occurrence they compete for, drawn by weight — explore reports the
// 0.9 branch nine times likelier than the 0.1 one.
func TestExploreWeighsTimedTransitionsAsOneOccurrence(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		private import SI::*;
		private import Stochastic::*;
		state def Machine {
			entry; then a;
			state a;
			state b;
			state c;
			transition first a accept after 5 [s] then b { @Probability { p = 0.9; } }
			transition first a accept after 5 [s] then c { @Probability { p = 0.1; } }
		}
	}`)
	sym := m.state(t, "Machine")
	x, err := Explore(context.Background(), mustPolicy(t, "explore"), m.fresh, func(ctx *Context) (Outcome, error) {
		exec, err := ctx.CreateStateExecutor(sym)
		if err != nil {
			return Outcome{}, err
		}
		if err := exec.RunToCompletion(); err != nil {
			return Outcome{}, err
		}
		return exec.Outcome(), nil
	})
	if err != nil || !x.Complete() || len(x.Outcomes) != 2 {
		t.Fatalf("explore: %v, %v", x, err)
	}
	probs := map[string]float64{}
	for _, o := range x.Outcomes {
		probs[o.Outcome.FinalState] += o.Probability
	}
	if math.Abs(probs["b"]-0.9) > 1e-9 || math.Abs(probs["c"]-0.1) > 1e-9 {
		t.Errorf("probabilities %v, want b=0.9, c=0.1", probs)
	}
}

// A check of the grouped timer charges each violation the weight its member
// was drawn at: the 0.1-weighted branch's failure carries mass 0.1.
func TestCheckWeighsTimedTransitionsAsOneOccurrence(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import ScalarValues::*;
		private import SI::*;
		private import Stochastic::*;
		state def Machine {
			entry; then a;
			state a;
			state bad;
			state good;
			transition first a accept after 5 [s] then bad { @Probability { p = 0.1; } }
			transition first a accept after 5 [s] then good { @Probability { p = 0.9; } }
		}
	}`)
	inv := invocationOf(nil, []*symbols.Symbol{m.state(t, "Machine")})
	prop := CheckProperty{Name: "notBad", Holds: func(_ *Context, inv *Invocation) (bool, error) {
		return inv.States[0].FinalStateName() != "bad", nil
	}}
	report, err := Check(context.Background(), m.fresh, inv, CheckBudget{}, CheckOptions{}, []CheckProperty{prop})
	if err != nil || report.Verdict != CheckViolation || len(report.Violations) != 1 {
		t.Fatalf("check: %v, %v", report, err)
	}
	v := report.Violations[0]
	if v.Name != "notBad" || math.Abs(v.Mass-0.1) > 1e-9 {
		t.Errorf("violation %s carries mass %v, want 0.1", v.Name, v.Mass)
	}
}
