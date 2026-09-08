package runtime

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// exploreModel is a model parsed and lowered once, from which every run of an
// exploration builds its own context.
type exploreModel struct {
	idx      *symbols.Index
	model    *semantics.Model
	resolver *resolve.Resolver
	path     string
}

func parseExploreModel(t *testing.T, text string) *exploreModel {
	t.Helper()
	path := filepath.Join(t.TempDir(), "explore.sysml")
	p := parser.New(source.New(path, []byte(text)))
	file := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse: %v", p.Diagnostics)
	}
	idx := symbols.NewIndex()
	idx.AddDocument(path, file)
	resolver := resolve.New(idx)
	return &exploreModel{idx: idx, model: semantics.NewModel(resolver), resolver: resolver, path: path}
}

func (m *exploreModel) fresh() (*Context, error) {
	return NewContext(m.model, m.resolver, 10000), nil
}

func (m *exploreModel) action(t *testing.T, name string) *symbols.Symbol {
	t.Helper()
	return namedOrFoundSymbol(t, m.idx, "test::"+name, m.idx.DocumentRoot(m.path), ast.DefAction, ast.UsageAction)
}

func (m *exploreModel) state(t *testing.T, name string) *symbols.Symbol {
	t.Helper()
	return namedOrFoundSymbol(t, m.idx, "test::"+name, m.idx.DocumentRoot(m.path), ast.DefState, ast.UsageState)
}

// exploreAction explores the runs of an action performed by no object.
func (m *exploreModel) exploreAction(t *testing.T, spelling, name string) *Exploration {
	t.Helper()
	policy, err := ParseSchedulePolicy(spelling)
	if err != nil {
		t.Fatalf("policy %s: %v", spelling, err)
	}
	sym := m.action(t, name)
	result, err := Explore(policy, m.fresh, func(ctx *Context) (Outcome, error) {
		outputs, err := ctx.ExecuteAction(sym)
		if err != nil {
			return Outcome{}, err
		}
		return ActionOutcome(outputs), nil
	})
	if err != nil {
		t.Fatalf("explore %s: %v", name, err)
	}
	return result
}

func outcomeTexts(x *Exploration) []string {
	texts := make([]string, len(x.Outcomes))
	for i, o := range x.Outcomes {
		texts[i] = o.Outcome.String()
	}
	return texts
}

const threeWritersModel = `package test {
	action race {
		attribute x : Integer = 0;
		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		action c { assign x := 3; }
		join sync;
		done;
		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first split then c;
		succession first a then sync;
		succession first b then sync;
		succession first c then sync;
		succession first sync then done;
	}
}`

func TestExploreThreeWritersReachEveryOutcomeOnce(t *testing.T) {
	m := parseExploreModel(t, threeWritersModel)
	x := m.exploreAction(t, "explore", "race")
	if !x.Complete() || x.Runs != 6 {
		t.Fatalf("status %q, want complete after the 6 orders of three tokens", x.Status())
	}
	want := []string{"x = 1", "x = 2", "x = 3"}
	if got := outcomeTexts(x); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("outcomes %v, want %v", got, want)
	}
	for _, o := range x.Outcomes {
		if o.Linearizations != 2 {
			t.Errorf("%s reached by %d linearizations, want 2 (the writer last, the other two either way)", o.Outcome, o.Linearizations)
		}
		w := o.Witness
		if len(w) != 2 || w[0].Kind != ChoiceTokenOrder || w[0].Alternatives != 3 || w[1].Alternatives != 2 {
			t.Errorf("%s witness %v, want a token-order choice among 3 then one among the 2 left", o.Outcome, w)
		}
	}
}

func TestExploreIsDeterministic(t *testing.T) {
	m := parseExploreModel(t, threeWritersModel)
	first := m.exploreAction(t, "explore", "race")
	second := m.exploreAction(t, "explore", "race")
	if first.Status() != second.Status() {
		t.Fatalf("status %q then %q", first.Status(), second.Status())
	}
	for i := range first.Outcomes {
		a, b := first.Outcomes[i], second.Outcomes[i]
		if a.Outcome.String() != b.Outcome.String() || a.Linearizations != b.Linearizations ||
			FormatChoices(a.Witness) != FormatChoices(b.Witness) || a.WitnessRun != b.WitnessRun {
			t.Errorf("outcome %d differs between explorations: %+v then %+v", i, a, b)
		}
	}
}

func TestExploreNoChoicePointsIsOneRun(t *testing.T) {
	m := parseExploreModel(t, `package test {
		action straight {
			attribute x : Integer = 0;
			first start;
			action a { assign x := x + 1; }
			action b { assign x := x * 10; }
			done;
			succession first start then a;
			succession first a then b;
			succession first b then done;
		}
	}`)
	x := m.exploreAction(t, "explore", "straight")
	if x.Runs != 1 || !x.Complete() {
		t.Fatalf("status %q, want complete (1 runs)", x.Status())
	}
	if got := outcomeTexts(x); len(got) != 1 || got[0] != "x = 10" {
		t.Fatalf("outcomes %v, want [x = 10]", got)
	}
	if w := x.Outcomes[0].Witness; len(w) != 0 || FormatChoices(w) != "no choice points" {
		t.Fatalf("witness %v, want none", w)
	}
}

func TestExploreRunsBudgetIsIncomplete(t *testing.T) {
	m := parseExploreModel(t, threeWritersModel)
	x := m.exploreAction(t, "explore:runs=1", "race")
	if x.Complete() || x.Runs != 1 {
		t.Fatalf("status %q, want incomplete after 1 run", x.Status())
	}
	if want := "incomplete: runs budget 1 hit after 1 runs"; x.Status() != want {
		t.Fatalf("status %q, want %q", x.Status(), want)
	}
	if len(x.Outcomes) != 1 {
		t.Fatalf("outcomes %v, want the one run's", outcomeTexts(x))
	}
}

func TestExploreDepthZeroIsIncomplete(t *testing.T) {
	m := parseExploreModel(t, threeWritersModel)
	x := m.exploreAction(t, "explore:depth=0", "race")
	if x.Complete() || x.Runs != 1 {
		t.Fatalf("status %q, want incomplete after the one run that varied nothing", x.Status())
	}
	if want := "incomplete: depth budget 0 hit after 1 runs"; x.Status() != want {
		t.Fatalf("status %q, want %q", x.Status(), want)
	}
}

func TestExploreErrorIsAnOutcome(t *testing.T) {
	m := parseExploreModel(t, `package test {
		action fragile {
			attribute x : Integer = 0;
			attribute y : Integer = 1;
			first start;
			fork split;
			action a { assign x := 0; }
			action b { assign x := 4; }
			join sync;
			action divide { assign y := 8 / x; }
			done;
			succession first start then split;
			succession first split then a;
			succession first split then b;
			succession first a then sync;
			succession first b then sync;
			succession first sync then divide;
			succession first divide then done;
		}
	}`)
	x := m.exploreAction(t, "explore", "fragile")
	if !x.Complete() || x.Runs != 2 {
		t.Fatalf("status %q, want complete (2 runs)", x.Status())
	}
	var errored, computed int
	for _, o := range x.Outcomes {
		if o.Outcome.Err != nil {
			errored++
			if !strings.HasPrefix(o.Outcome.String(), "error: ") {
				t.Errorf("error outcome renders as %q", o.Outcome)
			}
		} else {
			computed++
		}
	}
	if errored != 1 || computed != 1 {
		t.Fatalf("outcomes %v, want one error outcome and one computed", outcomeTexts(x))
	}
}

func TestExploreDecisionInLoop(t *testing.T) {
	m := parseExploreModel(t, `package test {
		action count {
			attribute n : Integer = 0;
			attribute odd : Integer = 0;
			attribute even : Integer = 0;
			first start;
			decide pick;
			action left { assign odd := odd + 1; assign n := n + 1; }
			action right { assign even := even + 1; assign n := n + 1; }
			merge again;
			done;
			succession first start then pick;
			succession first pick if n < 2 then left;
			succession first pick if n < 3 then right;
			succession first pick if n >= 3 then done;
			succession first left then again;
			succession first right then again;
			succession first again then pick;
		}
	}`)
	x := m.exploreAction(t, "explore", "count")
	if !x.Complete() {
		t.Fatalf("status %q, want complete", x.Status())
	}
	// Two overlapping guards hold while n < 2, so the first two rounds branch and
	// the third takes `right` alone: 4 linearizations, 3 outcomes by even/odd.
	if x.Runs != 4 {
		t.Fatalf("%d runs, want 4", x.Runs)
	}
	want := []string{
		"even = 1; n = 3; odd = 2",
		"even = 2; n = 3; odd = 1",
		"even = 3; n = 3; odd = 0",
	}
	if got := outcomeTexts(x); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("outcomes %v, want %v", got, want)
	}
}

func TestExploreStateTransitionConflict(t *testing.T) {
	m := parseExploreModel(t, `package test {
		state def Machine {
			entry; then idle;
			state idle;
			state left;
			state right;
			transition idle_left first idle accept go then left;
			transition idle_right first idle accept go then right;
		}
	}`)
	policy, err := ParseSchedulePolicy("explore")
	if err != nil {
		t.Fatal(err)
	}
	sym := m.state(t, "Machine")
	x, err := Explore(policy, m.fresh, func(ctx *Context) (Outcome, error) {
		exec, err := newStateExecutor(ctx, sym, nil)
		if err != nil {
			return Outcome{}, err
		}
		if err := exec.initialize(); err != nil {
			return Outcome{}, err
		}
		exec.SendSignal("go", nil)
		if err := exec.RunToCompletion(); err != nil {
			return Outcome{}, err
		}
		return exec.Outcome(), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !x.Complete() || x.Runs != 2 {
		t.Fatalf("status %q, want complete (2 runs)", x.Status())
	}
	want := []string{"finalState left; visits idle, left", "finalState right; visits idle, right"}
	if got := outcomeTexts(x); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("outcomes %v, want %v", got, want)
	}
	if w := x.Outcomes[1].Witness; len(w) != 1 || w[0].Kind != ChoiceTransition || !strings.HasSuffix(w[0].Took, "->right") {
		t.Fatalf("witness of right %v, want the transition choice into right", w)
	}
}

func TestExploreRejectsOtherPolicies(t *testing.T) {
	m := parseExploreModel(t, threeWritersModel)
	_, err := Explore(DefaultSchedulePolicy, m.fresh, func(*Context) (Outcome, error) { return Outcome{}, nil })
	if !errors.Is(err, ErrNotExploring) {
		t.Fatalf("Explore under %s: %v, want ErrNotExploring", DefaultSchedulePolicy, err)
	}
	ctx, _ := m.fresh()
	policy, _ := ParseSchedulePolicy("explore")
	if err := ctx.SetSchedule(policy); !errors.Is(err, ErrExploreUndriven) {
		t.Fatalf("SetSchedule(explore): %v, want ErrExploreUndriven", err)
	}
}

func TestParseExplorePolicy(t *testing.T) {
	for _, spelling := range []string{"explore", "explore:runs=10", "explore:depth=20", "explore:runs=10,depth=20", "explore:depth=20,runs=10"} {
		policy, err := ParseSchedulePolicy(spelling)
		if err != nil {
			t.Errorf("%s: %v", spelling, err)
			continue
		}
		budget, ok := policy.Exploration()
		if !ok {
			t.Errorf("%s does not explore", spelling)
		}
		if strings.Contains(spelling, "runs=10") && budget.Runs != 10 {
			t.Errorf("%s: runs %d", spelling, budget.Runs)
		}
		if strings.Contains(spelling, "depth=20") && budget.Depth != 20 {
			t.Errorf("%s: depth %d", spelling, budget.Depth)
		}
		if !strings.Contains(spelling, "runs=") && budget.Runs != DefaultExploreBudget.Runs {
			t.Errorf("%s: runs %d, want the default %d", spelling, budget.Runs, DefaultExploreBudget.Runs)
		}
	}
	for _, spelling := range []string{"explore:", "explore:runs", "explore:runs=", "explore:runs=x", "explore:runs=-1", "explore:runs=0x10", "explore:width=3", "explore:runs=1,runs=2", "explore:runs=1,", "explore:1"} {
		_, err := ParseSchedulePolicy(spelling)
		var typed *SchedulePolicyError
		if !errors.Is(err, ErrInvalidSchedulePolicy) || !errors.As(err, &typed) {
			t.Errorf("%s: %v, want a SchedulePolicyError", spelling, err)
		}
	}
}

// Every run gets a context of its own, so identities, messages and notes made
// by one run are not seen by the next.
func TestExploreRunsShareNoState(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		part def Rover;
		part def Fleet {
			part scout : Rover;
			ref part lead : Rover;
			attribute pings : Integer = 0;
			exhibit state run {
				entry; then idle;
				state idle;
				state a { entry action seta { assign lead := scout; assign pings := pings + 1; } }
				state b { entry action setb { assign lead := scout; assign pings := pings + 1; } }
				transition idle_a first idle accept go then a;
				transition idle_b first idle accept go then b;
				transition a_done first a accept ping then done;
				transition b_done first b accept ping then done;
				state done;
			}
		}
	}`)
	policy, err := ParseSchedulePolicy("explore")
	if err != nil {
		t.Fatal(err)
	}
	fleet := oneSymbol(t, m.idx, "test::Fleet")
	stateSym := oneSymbol(t, m.idx, "test::Fleet::run")
	var contexts int
	x, err := Explore(policy, func() (*Context, error) {
		contexts++
		return m.fresh()
	}, func(ctx *Context) (Outcome, error) {
		self, err := ctx.Instantiate(fleet)
		if err != nil {
			return Outcome{}, err
		}
		exec, err := newStateExecutor(ctx, stateSym, self)
		if err != nil {
			return Outcome{}, err
		}
		if err := exec.initialize(); err != nil {
			return Outcome{}, err
		}
		exec.SendSignal("go", nil)
		exec.SendSignal("ping", nil)
		if err := exec.RunToCompletion(); err != nil {
			return Outcome{}, err
		}
		outcome := Outcome{Outputs: make(map[string]Value)}
		for _, name := range []string{"lead", "pings"} {
			fv, err := self.GetFeatureValue(ctx, name)
			if err != nil {
				return Outcome{}, err
			}
			outcome.Outputs[name] = fv.Value
		}
		return outcome, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !x.Complete() || x.Runs != 2 || contexts != 2 {
		t.Fatalf("status %q with %d contexts, want complete (2 runs) in 2 contexts", x.Status(), contexts)
	}
	if got := outcomeTexts(x); len(got) != 1 || x.Outcomes[0].Linearizations != 2 {
		t.Fatalf("outcomes %v, want one reached by both orders", got)
	}
	if got := x.Outcomes[0].Outcome.String(); !strings.Contains(got, "lead = instance(") || !strings.Contains(got, "pings = 1") {
		t.Fatalf("outcome %q, want lead bound to one instance and pings = 1", got)
	}
}

// mustSchedule sets a policy a context can run under, failing the test otherwise.
func mustSchedule(t *testing.T, ctx *Context, policy SchedulePolicy) {
	t.Helper()
	if err := ctx.SetSchedule(policy); err != nil {
		t.Fatalf("schedule %s: %v", policy, err)
	}
}
