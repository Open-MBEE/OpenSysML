package analysis

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// checkedAction is the fixture's action as a check starts it, with every executor
// it started by the context it lives in, so a property can read the action's features.
type checkedAction struct {
	sym   *symbols.Symbol
	execs sync.Map
}

func (f *fixture) checked(t *testing.T, name string) *checkedAction {
	t.Helper()
	return &checkedAction{sym: f.symbol(t, name)}
}

func (a *checkedAction) start(ctx *runtime.Context) (*runtime.ActionExecutor, error) {
	exec, err := ctx.CreateActionExecutor(a.sym)
	if err != nil {
		return nil, err
	}
	a.execs.Store(ctx, exec)
	return exec, nil
}

// x is the property that the action's x is at most limit.
func (a *checkedAction) x(limit int64) runtime.CheckProperty {
	return runtime.CheckProperty{Name: "x", Holds: func(ctx *runtime.Context) (bool, error) {
		exec, ok := a.execs.Load(ctx)
		if !ok {
			return false, errors.New("no executor started in this context")
		}
		x, ok := exec.(*runtime.ActionExecutor).Results()["x"]
		if !ok || x.Kind != runtime.ValConst {
			return false, errors.New("x holds no value")
		}
		return x.Const.Int <= limit, nil
	}}
}

// checkQuestion asks kind about the racing action through the check engine.
func checkQuestion(t *testing.T, f *fixture, kind Kind, ask *CheckAsk) Question {
	t.Helper()
	return Question{Kind: kind, Subject: "test::race", Schedule: policy(t, "explore"), Free: FreeSchedule, Check: ask}
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
	for _, name := range []string{"depth", "states", "steps", "elements"} {
		found := false
		for _, b := range d.Bounds {
			found = found || b == name
		}
		if !found {
			t.Fatalf("bounds %v, want %s", d.Bounds, name)
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
	race := f.checked(t, "race")
	plan := answered(t, Default(), f.building(), checkQuestion(t, f, Outcomes, &CheckAsk{Start: race.start}), Budget{})
	if !sameNames(stepNames(plan), []string{ExploreEngineName, CheckEngineName}) {
		t.Fatalf("steps %v, want explore refusing then check", stepNames(plan))
	}
	result := plan.Result
	if result.Engine != CheckEngineName || result.Claim != ClaimOutcomes || result.Strength != Bounded {
		t.Fatalf("result %+v, want check's bounded outcomes", result)
	}
	c := result.Check()
	if c == nil || c.Report.Verdict != runtime.CheckDivergent || len(c.Report.Finals) != 3 || len(c.Report.Divergent) != 1 {
		t.Fatalf("checked %+v, want a complete search of 3 finals with x divergent", c)
	}
	if result.Bounds.Reached() {
		t.Fatalf("bounds %s, want none reached", result.Bounds)
	}
	if c.Violations != nil || c.Divergent != nil {
		t.Fatalf("witness paths %v %v, want none written without a directory", c.Violations, c.Divergent)
	}
	if s := result.Standing(); !strings.Contains(s, "bounded") || !strings.Contains(s, "states") || strings.Contains(s, "proved") {
		t.Fatalf("standing %q, want bounded with the states searched", s)
	}
}

// A property true at every state holds bounded; one a schedule falsifies is violated,
// witnessed after replay, the witness being the schedule that reached it.
func TestCheckHoldsOrWitnessesAProperty(t *testing.T) {
	f := parseFixture(t)
	race := f.checked(t, "race")
	holds := answered(t, Default(), f.building(), checkQuestion(t, f, Holds, &CheckAsk{Start: race.start, Properties: []runtime.CheckProperty{race.x(3)}}), Budget{}).Result
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
		if seen[path] || !strings.HasPrefix(filepath.Base(path), "test.race.") {
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
			if step.Result.Strength != Bounded {
				t.Fatalf("check %+v, want bounded", step.Result)
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
	slow := runtime.CheckProperty{Name: "slow", Holds: func(*runtime.Context) (bool, error) {
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
		if results[i].Strength != Bounded || len(results[i].Check().Report.Divergent) != 1 || len(results[i].Check().Report.Divergent[0].Values) != 3 {
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
