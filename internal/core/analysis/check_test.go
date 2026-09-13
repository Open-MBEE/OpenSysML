package analysis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// checkedAction is the fixture's action as a check starts it.
type checkedAction struct {
	sym *symbols.Symbol
}

func (f *fixture) checked(t *testing.T, name string) *checkedAction {
	t.Helper()
	return &checkedAction{sym: f.symbol(t, name)}
}

func (a *checkedAction) start(ctx *runtime.Context) (*runtime.ActionExecutor, error) {
	return ctx.CreateActionExecutor(a.sym)
}

// x is the property that the action's x is at most limit.
func (a *checkedAction) x(limit int64) runtime.CheckProperty { return a.atMost("x", limit) }

// y is the property that the action's y is at most limit.
func (a *checkedAction) y(limit int64) runtime.CheckProperty { return a.atMost("y", limit) }

// atMost is the property that the action's named integer is at most limit.
func (a *checkedAction) atMost(name string, limit int64) runtime.CheckProperty {
	return runtime.CheckProperty{Name: name, Holds: func(_ *runtime.Context, exec *runtime.ActionExecutor) (bool, error) {
		v, ok := exec.Results()[name]
		if !ok || v.Kind != runtime.ValConst {
			return false, errors.New(name + " holds no value")
		}
		return v.Const.Int <= limit, nil
	}}
}

// checkQuestion asks kind about the racing action through the check engine.
func checkQuestion(t *testing.T, f *fixture, kind Kind, ask *CheckAsk) Question {
	t.Helper()
	return questionOf(t, "test::race", kind, ask)
}

// questionOf asks kind about the named action through the check engine.
func questionOf(t *testing.T, subject string, kind Kind, ask *CheckAsk) Question {
	t.Helper()
	return Question{Kind: kind, Subject: subject, Schedule: policy(t, "explore"), Free: FreeSchedule, Check: ask}
}

// finals is the outcome set a check reached, sorted.
func finals(c *Checked) []string {
	var out []string
	for _, f := range c.Report.Finals {
		out = append(out, f.Outcome)
	}
	sort.Strings(out)
	return out
}

// explored is the outcome set an exploration reached, sorted.
func explored(x *runtime.Exploration) []string {
	var out []string
	for _, o := range x.Outcomes {
		out = append(out, o.Outcome.String())
	}
	sort.Strings(out)
	return out
}

// check describes itself as the framework's bounded, replaying engine over outcomes and holds.
func TestCheckDescribesItself(t *testing.T) {
	d := NewCheck().Describe()
	if NewCheck().Name() != CheckEngineName || d.Authority != Bounded || !d.Replays {
		t.Fatalf("description %+v, want check, bounded, replaying", d)
	}
	if len(d.Questions) != 2 || d.Questions[0] != Outcomes || d.Questions[1] != Holds {
		t.Fatalf("questions %v, want outcomes and holds", d.Questions)
	}
	for _, name := range append([]string{"depth", "states", "deadline"}, runtime.ExecutorBounds...) {
		found := false
		for _, b := range d.Bounds {
			found = found || b == name
		}
		if !found {
			t.Fatalf("bounds %v, want %s", d.Bounds, name)
		}
	}
}

// The bounds a check result carries are the search's and each executor budget by its
// own name and limit: an exhausted budget is reached under that name, no other.
func TestCheckBoundsNameEachBudgetHit(t *testing.T) {
	limits := runtime.Budgets{MaxSteps: 11, MaxActionSteps: 12, MaxStateEvents: 13, MaxDoSteps: 14, MaxElements: 15}
	report := &runtime.CheckReport{Limits: limits, BoundsHit: []string{runtime.BoundActionSteps, runtime.BoundBehaviors}}
	bounds := checkBounds(report, Budget{Depth: 7, Runs: 8})
	want := map[string]Bound{
		"depth":                  {Name: "depth", Limit: 7},
		"states":                 {Name: "states", Limit: 8},
		runtime.BoundSteps:       {Name: runtime.BoundSteps, Limit: 11},
		runtime.BoundActionSteps: {Name: runtime.BoundActionSteps, Limit: 12, Reached: true},
		runtime.BoundEvents:      {Name: runtime.BoundEvents, Limit: 13},
		runtime.BoundBehaviors:   {Name: runtime.BoundBehaviors, Limit: 13, Reached: true},
		runtime.BoundDoSteps:     {Name: runtime.BoundDoSteps, Limit: 14},
		runtime.BoundElements:    {Name: runtime.BoundElements, Limit: 15},
	}
	if len(bounds) != len(want) {
		t.Fatalf("bounds %s, want %d of them", bounds, len(want))
	}
	for _, b := range bounds {
		if b != want[b.Name] {
			t.Fatalf("bound %+v, want %+v", b, want[b.Name])
		}
	}
	for _, name := range NewCheck().Describe().Bounds {
		if _, ok := bounds.Limit(name); !ok && name != "deadline" {
			t.Fatalf("the engine describes %s but the result carries no such bound", name)
		}
	}
}

// check refuses what it does not answer with typed reasons: another kind, free inputs, a
// fixed schedule, and a question that starts no action; a question without a Check is refused,
// so no surface that never sets one sees the engine.
func TestCheckCoversOutcomesAndHoldsOverAnActionsSchedules(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	e := NewCheck()
	ask := &CheckAsk{Start: race.start}
	for _, tc := range []struct {
		name string
		q    Question
		want error
	}{
		{"evaluate", Question{Kind: Evaluate, Free: FreeSchedule, Check: ask}, ErrNotAsked},
		{"sweep", Question{Kind: Sweep, Free: FreeSchedule, Check: ask}, ErrNotAsked},
		{"free inputs", Question{Kind: Holds, Free: FreeSchedule | FreeInputs, Check: ask}, ErrFreedom},
		{"fixed schedule", Question{Kind: Outcomes, Check: ask}, ErrConstruct},
		{"no check", Question{Kind: Outcomes, Free: FreeSchedule, Linearize: raceRun(t, f)}, ErrMalformedQuestion},
		{"no start", Question{Kind: Holds, Free: FreeSchedule, Check: &CheckAsk{}}, ErrMalformedQuestion},
	} {
		c := e.Covers(f.building(), tc.q)
		if c.Covered || !errors.Is(c.Refusal, tc.want) {
			t.Errorf("%s: coverage %+v, want refused with %v", tc.name, c, tc.want)
		}
	}
	for _, kind := range []Kind{Outcomes, Holds} {
		if c := e.Covers(f.building(), Question{Kind: kind, Free: FreeSchedule, Check: ask}); !c.Covered {
			t.Errorf("%s: refused %v, want covered", kind, c.Refusal)
		}
	}
}

// An exhaustive search answers outcomes bounded, never proved, with every final the
// schedules reach and no bound reached; the standing names the states searched.
func TestCheckBoundsAnOutcomeSet(t *testing.T) {
	f := parseFixture(t)
	steady := f.checked(t, "steady")
	plan := answered(t, Default(), f.building(), questionOf(t, "test::steady", Outcomes, &CheckAsk{Start: steady.start}), Budget{})
	if !sameNames(stepNames(plan), []string{ExploreEngineName, CheckEngineName}) {
		t.Fatalf("steps %v, want explore refusing then check", stepNames(plan))
	}
	result := plan.Result
	if result.Engine != CheckEngineName || result.Claim != ClaimOutcomes || result.Strength != Bounded {
		t.Fatalf("result %+v, want check's bounded outcomes", result)
	}
	c := result.Check()
	if c == nil || c.Report.Verdict != runtime.CheckExhaustive || len(c.Report.Finals) != 1 || len(c.Report.Divergent) != 0 {
		t.Fatalf("checked %+v, want a complete search of 1 final with nothing divergent", c)
	}
	if result.Bounds.Reached() {
		t.Fatalf("bounds %s, want none reached", result.Bounds)
	}
	if s := result.Standing(); !strings.Contains(s, "bounded") || !strings.Contains(s, "states") || strings.Contains(s, "proved") {
		t.Fatalf("standing %q, want bounded with the states searched", s)
	}
}

// A feature the schedule decides is a sensitivity, witnessed by the schedule reaching
// its first value once every value's witness has replayed; the outcome set is kept.
func TestCheckWitnessesADivergence(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	for _, kind := range []Kind{Outcomes, Holds} {
		result := answered(t, Default(), f.building(), checkQuestion(t, f, kind, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{race.x(3)}}), Budget{}).Result
		if result.Engine != CheckEngineName || result.Claim != ClaimSensitive || result.Strength != Witnessed || result.Witness == nil {
			t.Fatalf("%s: result %+v, want check's witnessed sensitivity", kind, result)
		}
		c := result.Check()
		if c == nil || c.Report.Verdict != runtime.CheckDivergent || len(c.Report.Finals) != 3 || len(c.Report.Divergent) != 1 || len(c.Report.Violations) != 0 {
			t.Fatalf("%s: checked %+v, want a complete search of 3 finals with x divergent", kind, c)
		}
		if c.Violations != nil || c.Divergent != nil {
			t.Fatalf("witness paths %v %v, want none written without a directory", c.Violations, c.Divergent)
		}
		if result.Reason != "x ends as 1 or 2 or 3" {
			t.Fatalf("%s: reason %q, want the divergence spelt", kind, result.Reason)
		}
		first := c.Report.Divergent[0].Values[0].Witness.Choices
		if replay, ok := result.Witness.Schedule.Replay(); !ok || len(replay) != len(first) {
			t.Fatalf("%s: witness %s, want a replay of the first value's schedule", kind, result.Witness.Schedule)
		}
	}
}

// A property true at every state holds bounded; one a schedule falsifies is violated,
// witnessed after replay, the witness being the schedule that reached it.
func TestCheckHoldsOrWitnessesAProperty(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	steady := f.checked(t, "steady")
	holds := answered(t, Default(), f.building(), questionOf(t, "test::steady", Holds, &CheckAsk{Start: steady.start, Properties: []runtime.CheckProperty{steady.y(1)}}), Budget{}).Result
	if holds.Engine != CheckEngineName || holds.Claim != ClaimHolds || holds.Strength != Bounded || holds.Witness != nil {
		t.Fatalf("result %+v, want check holding bounded with no witness", holds)
	}
	violated := answered(t, Default(), f.building(), checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{race.x(2)}}), Budget{}).Result
	if violated.Claim != ClaimViolated || violated.Strength != Witnessed || violated.Witness == nil {
		t.Fatalf("result %+v, want violated and witnessed", violated)
	}
	c := violated.Check()
	if c == nil || c.Report.Verdict != runtime.CheckViolation || len(c.Report.Violations) == 0 || c.Report.Violations[0].Kind != runtime.ViolationProperty {
		t.Fatalf("checked %+v, want a property violation", c)
	}
	if replay, ok := violated.Witness.Schedule.Replay(); !ok || len(replay) != len(c.Report.Violations[0].Witness.Choices) {
		t.Fatalf("witness schedule %s, want a replay of the violation's choices", violated.Witness.Schedule)
	}
	if !strings.Contains(violated.Reason, "x is false") {
		t.Fatalf("reason %q, want the violation", violated.Reason)
	}
}

// With a directory named, every violation and divergent value is written as a witness file
// the shared parser reads back to the choices the check took.
func TestCheckWritesEveryWitness(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	dir := filepath.Join(t.TempDir(), "witnesses")
	ask := &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{race.x(1)}, Diverge: []string{"x"}, WitnessDir: dir}
	result := answered(t, Default(), f.building(), checkQuestion(t, f, Holds, ask), Budget{}).Result
	c := result.Check()
	if c == nil || len(c.Violations) != len(c.Report.Violations) || len(c.Divergent) != 1 || len(c.Divergent[0]) != 3 {
		t.Fatalf("checked %+v, want a path per violation and per divergent value of x", c)
	}
	paths := append([]string{}, c.Violations...)
	paths = append(paths, c.Divergent[0]...)
	seen := make(map[string]bool)
	for i, path := range paths {
		if seen[path] || !strings.HasPrefix(filepath.Base(path), "test.race") {
			t.Fatalf("path %d %q, want distinct under the subject's name", i, path)
		}
		seen[path] = true
		text, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		w, err := runtime.ParseWitness(string(text))
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if w.Trace == "" {
			t.Fatalf("%s carries no trace", path)
		}
	}
	for _, path := range c.Violations {
		if !strings.Contains(path, "violation-") {
			t.Fatalf("violation path %q, want named as one", path)
		}
	}
}

// A link planted where a witness will be written is replaced by the witness, a
// regular file, and what the link pointed at is left as it was.
func TestCheckWitnessReplacesAPlantedLink(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "target")
	const kept = "not a witness\n"
	if err := os.WriteFile(target, []byte(kept), 0o600); err != nil {
		t.Fatal(err)
	}
	planted := filepath.Join(dir, "test.race-x-1.witness")
	if err := os.Symlink(target, planted); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	ask := &CheckAsk{Start: race.start, Diverge: []string{"x"}, WitnessDir: dir}
	result := answered(t, Default(), f.building(), checkQuestion(t, f, Outcomes, ask), Budget{}).Result
	c := result.Check()
	if c == nil || len(c.Divergent) != 1 || c.Divergent[0][0] != planted {
		t.Fatalf("checked %+v, want x's first witness at %s", c, planted)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != kept {
		t.Fatalf("the link's target reads %q, %v; want it untouched", got, err)
	}
	info, err := os.Lstat(planted)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("witness path is %v, %v; want a regular file", info, err)
	}
	text, err := os.ReadFile(planted)
	if err != nil {
		t.Fatal(err)
	}
	if w, err := runtime.ParseWitness(string(text)); err != nil || w.String() != c.Report.Divergent[0].Values[0].Witness.String() {
		t.Fatalf("witness at %s: %v, want x's first witness", planted, err)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".*")); len(left) != 0 {
		t.Fatalf("temporary files left: %v", left)
	}
}

// Two features spelled apart, `a/b` and `a?b`, are written to two files each, the
// characters no file name keeps spelled `%XX`; each file replays to the value it names.
func TestCheckWitnessFilesTellFeaturesApart(t *testing.T) {
	const model = `package test {
	action race {
		attribute 'a/b' : Integer = 0;
		attribute 'a?b' : Integer = 0;
		first start;
		fork split;
		action p { assign 'a/b' := 1; assign 'a?b' := 1; }
		action q { assign 'a/b' := 2; assign 'a?b' := 2; }
		join sync;
		done;
		succession first start then split;
		succession first split then p;
		succession first split then q;
		succession first p then sync;
		succession first q then sync;
		succession first sync then done;
	}
}`
	f := parseModel(t, model)
	race := f.checked(t, "race")
	dir := t.TempDir()
	ask := &CheckAsk{Start: race.start, Diverge: []string{"a/b", "a?b"}, WitnessDir: dir}
	result := answered(t, Default(), f.building(), checkQuestion(t, f, Outcomes, ask), Budget{}).Result
	c := result.Check()
	if result.Claim != ClaimSensitive || result.Strength != Witnessed || c == nil || len(c.Report.Divergent) != 2 {
		t.Fatalf("%s %s %+v, want both features divergent and witnessed", result.Claim, result.Strength, c)
	}
	want := map[string][]string{
		"a/b": {"test.race-a%2Fb-1.witness", "test.race-a%2Fb-2.witness"},
		"a?b": {"test.race-a%3Fb-1.witness", "test.race-a%3Fb-2.witness"},
	}
	for i, d := range c.Report.Divergent {
		paths := c.Divergent[i]
		if len(paths) != len(want[d.Feature]) || len(d.Values) != len(paths) {
			t.Fatalf("%s: paths %v, want %v", d.Feature, paths, want[d.Feature])
		}
		for j, path := range paths {
			if path != filepath.Join(dir, want[d.Feature][j]) {
				t.Errorf("%s value %d at %s, want %s", d.Feature, j+1, path, want[d.Feature][j])
			}
			text, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			w, err := runtime.ParseWitness(string(text))
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			if w.String() != d.Values[j].Witness.String() {
				t.Errorf("%s holds another witness than %s = %s", path, d.Feature, d.Values[j].Value)
			}
			replayed, err := runtime.ReplayAction(context.Background(), func() (*runtime.Context, error) { return f.context(t), nil }, race.start, w, nil)
			if err != nil {
				t.Fatalf("%s: replay: %v", path, err)
			}
			if got := replayed.Exec.Results()[d.Feature]; got.Kind != runtime.ValConst || fmt.Sprint(got.Const.Int) != d.Values[j].Value {
				t.Errorf("%s replays to %s = %v, want %s", path, d.Feature, got, d.Values[j].Value)
			}
		}
	}
}

// A witness whose replay leaves another state is not covered with the disagreement as its
// reason — never violated — and under all it yields to explore's complete table.
func TestCheckWitnessThatFailsReplayIsNotCovered(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	other := f.symbol(t, "Double")
	var starts sync.Mutex
	started := 0
	// The search starts race; every replay after it starts a calc instead, so no witness replays.
	start := func(ctx *runtime.Context) (*runtime.ActionExecutor, error) {
		starts.Lock()
		n := started
		started++
		starts.Unlock()
		if n == 0 {
			return race.start(ctx)
		}
		return ctx.CreateActionExecutor(other)
	}
	q := checkQuestion(t, f, Holds, &CheckAsk{Start: start, Properties: []runtime.CheckProperty{race.x(2)}})
	result := answered(t, Default(), f.building(), q, Budget{}).Result
	if result.Covered() || result.Claim != ClaimNone || !strings.Contains(result.Reason, "replay") {
		t.Fatalf("result %+v, want not covered with the replay's disagreement", result)
	}
	if c := result.Check(); c == nil || c.Report.Verdict != runtime.CheckViolation {
		t.Fatalf("checked %+v, want the report kept beside the refusal", c)
	}
	started = 0
	q.Kind = Outcomes
	q.Linearize = raceRun(t, f)
	plan, err := Default().AnswerWith(context.Background(), f.building(), q, Budget{}, All())
	if err != nil {
		t.Fatal(err)
	}
	if plan.Result.Engine != ExploreEngineName || plan.Result.Strength != Proved || len(plan.Disagreements) != 0 {
		t.Fatalf("result %+v (%d disagreements), want explore's proof undisputed", plan.Result, len(plan.Disagreements))
	}
}

// Under all, check and explore answer the same outcomes question: explore proves the complete
// table, check bounds the same set, and neither disputes the other.
func TestExploreRefereesCheck(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	q := checkQuestion(t, f, Outcomes, &CheckAsk{Start: race.start})
	q.Linearize = raceRun(t, f)
	plan, err := Default().AnswerWith(context.Background(), f.building(), q, Budget{}, All())
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Disagreements) != 0 || plan.Result.Engine != ExploreEngineName || plan.Result.Strength != Proved {
		t.Fatalf("plan %+v, want explore's proof standing with no disagreement", plan.Result)
	}
	var checked *Checked
	var exploration *runtime.Exploration
	for _, step := range plan.Steps {
		if step.Result == nil {
			continue
		}
		switch step.Engine {
		case CheckEngineName:
			checked = step.Result.Check()
			if step.Result.Claim != ClaimSensitive || step.Result.Strength != Witnessed {
				t.Fatalf("check %+v, want the race's sensitivity witnessed beside the proof", step.Result)
			}
		case ExploreEngineName:
			exploration = step.Result.Exploration()
		}
	}
	if checked == nil || exploration == nil || !exploration.Complete() {
		t.Fatalf("steps %v, want check's report and explore's complete table", stepNames(plan))
	}
	if got, want := finals(checked), explored(exploration); strings.Join(got, ";") != strings.Join(want, ";") {
		t.Fatalf("check finals %v, explore outcomes %v, want equal", got, want)
	}
	auto := answered(t, Default(), f.building(), q, Budget{})
	if auto.Result.Engine != ExploreEngineName || len(auto.Steps) != 1 {
		t.Fatalf("auto %+v (steps %v), want explore alone as the stronger authority", auto.Result, stepNames(auto))
	}
}

// The budget's depth and runs bound the search as its depth and states; each reached is
// named and the claim stays bounded.
func TestCheckTakesTheBudgetsDepthAndRuns(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	q := checkQuestion(t, f, Outcomes, &CheckAsk{Start: race.start})
	byStates := answered(t, Default(), f.building(), q, Budget{Runs: 2}).Result
	if byStates.Strength != Bounded || !byStates.Bounds.Reached() {
		t.Fatalf("result %+v, want bounded with a bound reached", byStates)
	}
	if limit, ok := byStates.Bounds.Limit("states"); !ok || limit != 2 || byStates.Check().Report.Verdict != runtime.CheckWithinBounds {
		t.Fatalf("bounds %s (verdict %v), want states=2 reached within bounds", byStates.Bounds, byStates.Check().Report.Verdict)
	}
	byDepth := answered(t, Default(), f.building(), q, Budget{Depth: 2}).Result
	for _, b := range byDepth.Bounds {
		if b.Name == "depth" && (b.Limit != 2 || !b.Reached) {
			t.Fatalf("bounds %s, want depth=2 reached", byDepth.Bounds)
		}
		if b.Name == "states" && b.Reached {
			t.Fatalf("bounds %s, want states unreached", byDepth.Bounds)
		}
	}
	if !strings.Contains(byDepth.Standing(), "depth=2 (reached)") {
		t.Fatalf("standing %q, want the depth bound named", byDepth.Standing())
	}
}

// The plan's deadline stops the search: the step fails with the deadline, no result is
// composed, and nothing claims within bounds.
func TestCheckStopsAtThePlansDeadline(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	slow := runtime.CheckProperty{Name: "slow", Holds: func(*runtime.Context, *runtime.ActionExecutor) (bool, error) {
		time.Sleep(20 * time.Millisecond)
		return true, nil
	}}
	q := checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{slow}})
	plan, err := Default().Answer(context.Background(), f.building(), q, Budget{Deadline: time.Now().Add(50 * time.Millisecond)})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("answer: %v, want the deadline", err)
	}
	var stopped *runtime.CheckStopped
	if !errors.As(err, &stopped) {
		t.Fatalf("answer: %v, want the search's stop with its counts", err)
	}
	if plan.Result.Covered() {
		t.Fatalf("result %+v, want nothing claimed", plan.Result)
	}
	for _, step := range plan.Steps {
		if step.Engine == CheckEngineName && step.Err == nil {
			t.Fatalf("step %+v, want the deadline as its error", step)
		}
	}
}

// Two check plans on one model, on two goroutines, each build workers of their own; run
// under -race, this is the isolation test.
func TestChecksOnOneModelHaveWorkersOfTheirOwn(t *testing.T) {
	f := parseFixture(t)
	w := &workers{}
	model := f.recording(w)
	race := f.checked(t, "race")
	q := checkQuestion(t, f, Outcomes, &CheckAsk{Start: race.start, Diverge: []string{"x"}})
	results := make([]Result, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			plan, err := Default().AnswerWith(context.Background(), model, q, Budget{Jobs: 2}, Only(CheckEngineName))
			results[i], errs[i] = plan.Result, err
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("plan %d: %v", i, err)
		}
		if results[i].Claim != ClaimSensitive || len(results[i].Check().Report.Divergent) != 1 || len(results[i].Check().Report.Divergent[0].Values) != 3 {
			t.Fatalf("plan %d: %+v, want x divergent over 3 values", i, results[i])
		}
	}
	if resolvers, models := w.distinct(); resolvers < 2 || models < 2 {
		t.Fatalf("%d resolvers, %d models across two plans, want each plan's own", resolvers, models)
	}
	if w.asked < 2 {
		t.Fatalf("semantics asked %d times, want once per plan at least", w.asked)
	}
}

// A property that fails to evaluate is a violation witnessed once its replay raises the same
// failure; a property whose replay evaluates otherwise leaves the report not covered.
func TestCheckWitnessesAPropertyThatFailsToEvaluate(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	offline := errors.New("x is offline")
	failing := runtime.CheckProperty{Name: "sensor", Holds: func(_ *runtime.Context, exec *runtime.ActionExecutor) (bool, error) {
		if exec.State() == runtime.StateCompleted && exec.Results()["x"].Const.Int == 2 {
			return false, offline
		}
		return true, nil
	}}
	result := answered(t, Default(), f.building(), checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{failing}}), Budget{}).Result
	if result.Claim != ClaimViolated || result.Strength != Witnessed || result.Witness == nil {
		t.Fatalf("result %+v, want the failure violated and witnessed", result)
	}
	c := result.Check()
	if c == nil || len(c.Report.Violations) != 1 || c.Report.Violations[0].Kind != runtime.ViolationFailure || c.Report.Violations[0].Witness.Property != "sensor" {
		t.Fatalf("checked %+v, want the sensor's failure as the one violation", c)
	}
	var evaluations atomic.Int32
	flaky := runtime.CheckProperty{Name: "sensor", Holds: func(_ *runtime.Context, exec *runtime.ActionExecutor) (bool, error) {
		if exec.State() == runtime.StateCompleted && exec.Results()["x"].Const.Int == 2 && evaluations.Add(1) == 1 {
			return false, offline
		}
		return true, nil
	}}
	result = answered(t, Default(), f.building(), checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{flaky}}), Budget{}).Result
	if result.Covered() || result.Claim != ClaimNone || !strings.Contains(result.Reason, "sensor") {
		t.Fatalf("result %+v, want not covered by the sensor's disagreement", result)
	}
}
