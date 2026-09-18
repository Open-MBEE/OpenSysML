package runtime

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
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

// parseLibraryModel is parseExploreModel over the standard libraries, for models using them.
func parseLibraryModel(t *testing.T, text string) *exploreModel {
	t.Helper()
	if err := primeLibraryCache(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "explore.sysml")
	idx := libs.NewModelIndex()
	idx.AddDocument(path, parseAndBuild(t, text))
	idx.ExpandWildcardImports()
	resolver := resolve.New(idx)
	return &exploreModel{idx: idx, model: semantics.NewModel(resolver), resolver: resolver, path: path}
}

func (m *exploreModel) fresh() (*Context, error) {
	return NewContext(NewModel(m.model, m.resolver), 10000), nil
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
	result, err := Explore(context.Background(), policy, m.fresh, func(ctx *Context) (Outcome, error) {
		outputs, err := ctx.ExecuteAction(sym)
		if err != nil {
			return Outcome{}, err
		}
		return ctx.ActionOutcome(outputs), nil
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
	x, err := Explore(context.Background(), policy, m.fresh, func(ctx *Context) (Outcome, error) {
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

// A choice whose branches are enabled only by the incoming transition's effect is
// a choice point resolved as the choice is reached: exploration enumerates both
// branches, each witness naming the choice and the branch taken, and a seed that
// drew a branch draws it again; the run whose effect is not run first — a
// junction's static reading — would find neither branch enabled.
func TestExploreDynamicChoiceBranches(t *testing.T) {
	m := parseExploreModel(t, `package test {
		state def Machine {
			attribute level : Integer = 0;
			entry; then idle;
			state idle;
			choice pick;
			state left;
			state right;
			transition first idle accept go do assign level := 8 then pick;
			transition first pick if level > 5 then left;
			transition first pick if level > 7 then right;
		}
	}`)
	policy, err := ParseSchedulePolicy("explore")
	if err != nil {
		t.Fatal(err)
	}
	sym := m.state(t, "Machine")
	run := func(ctx *Context) (Outcome, error) {
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
	}
	x, err := Explore(context.Background(), policy, m.fresh, run)
	if err != nil {
		t.Fatal(err)
	}
	if !x.Complete() || x.Runs != 2 {
		t.Fatalf("status %q, want complete (2 runs)", x.Status())
	}
	want := []string{"finalState left; visits idle, left; level = 8", "finalState right; visits idle, right; level = 8"}
	if got := outcomeTexts(x); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("outcomes %v, want %v", got, want)
	}
	w := x.Outcomes[1].Witness
	if len(w) != 1 || w[0].Kind != ChoiceTransition || w[0].Where != "choice pick" || !strings.HasSuffix(w[0].Took, "->right") {
		t.Fatalf("witness of right %v, want the branch of choice pick into right", w)
	}

	under := func(spelling string) (Outcome, ChoicePoint) {
		t.Helper()
		fixed, err := ParseSchedulePolicy(spelling)
		if err != nil {
			t.Fatal(err)
		}
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		if err := ctx.SetSchedule(fixed); err != nil {
			t.Fatal(err)
		}
		outcome, err := run(ctx)
		if err != nil {
			t.Fatalf("%s: %v", spelling, err)
		}
		notes := ctx.Notes()
		if len(notes) != 1 {
			t.Fatalf("%s: notes %v, want the one choice at pick", spelling, notes)
		}
		choice, ok := notes[0].(ChoicePoint)
		if !ok || choice.Kind != ChoiceTransition || choice.Where != "choice pick" {
			t.Fatalf("%s: note %v, want the branch choice at pick", spelling, notes[0])
		}
		return outcome, choice
	}
	reached := make(map[int]bool)
	for _, spelling := range []string{"seed:1", "seed:2", "seed:3", "seed:6"} {
		outcome, choice := under(spelling)
		if got := outcome.String(); got != want[choice.Taken] {
			t.Fatalf("%s: outcome %q after taking %s, want %q", spelling, got, choice.Alternatives[choice.Taken], want[choice.Taken])
		}
		if again, _ := under(spelling); again.String() != outcome.String() {
			t.Fatalf("%s: outcome %q, then %q; want the seed to replay its run", spelling, outcome, again)
		}
		reached[choice.Taken] = true
	}
	if len(reached) != 2 {
		t.Fatalf("seeds reached only %v, want both branches of pick", reached)
	}
}

// A junction with several branches enabled when the incoming transition is
// selected is a choice point read statically, before the incoming effect: the
// branch guards see the data as it stood before the effect, exploration
// enumerates both branches with a witness at the junction, and a seed replays
// its draw. A guard the effect would enable is not a branch.
func TestExploreStaticJunctionBranches(t *testing.T) {
	m := parseExploreModel(t, `package test {
		state def Machine {
			attribute level : Integer = 0;
			attribute route : Integer = 0;
			entry; then idle;
			state idle;
			junction split;
			state left;
			state right;
			state late;
			transition first idle accept go do assign level := 8 then split;
			transition first split if level == 0 do assign route := 1 then left;
			transition first split if level < 5 do assign route := 2 then right;
			transition first split if level > 7 do assign route := 3 then late;
		}
	}`)
	policy, err := ParseSchedulePolicy("explore")
	if err != nil {
		t.Fatal(err)
	}
	sym := m.state(t, "Machine")
	run := func(ctx *Context) (Outcome, error) {
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
	}
	x, err := Explore(context.Background(), policy, m.fresh, run)
	if err != nil {
		t.Fatal(err)
	}
	if !x.Complete() || x.Runs != 2 {
		t.Fatalf("status %q, want complete (2 runs)", x.Status())
	}
	want := []string{
		"finalState left; visits idle, left; level = 8; route = 1",
		"finalState right; visits idle, right; level = 8; route = 2",
	}
	if got := outcomeTexts(x); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("outcomes %v, want %v", got, want)
	}
	w := x.Outcomes[1].Witness
	if len(w) != 1 || w[0].Kind != ChoiceTransition || w[0].Where != "junction split" || !strings.HasSuffix(w[0].Took, "->right") {
		t.Fatalf("witness of right %v, want the branch of junction split into right", w)
	}
	if alts := w[0].Alternatives; alts != 2 {
		t.Fatalf("alternatives %v, want the two branches enabled before the effect", alts)
	}

	for _, spelling := range []string{"seed:1", "seed:2", "seed:3"} {
		fixed, err := ParseSchedulePolicy(spelling)
		if err != nil {
			t.Fatal(err)
		}
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		if err := ctx.SetSchedule(fixed); err != nil {
			t.Fatal(err)
		}
		outcome, err := run(ctx)
		if err != nil {
			t.Fatalf("%s: %v", spelling, err)
		}
		notes := ctx.Notes()
		if len(notes) != 1 {
			t.Fatalf("%s: notes %v, want the one choice at split", spelling, notes)
		}
		choice, ok := notes[0].(ChoicePoint)
		if !ok || choice.Kind != ChoiceTransition || choice.Where != "junction split" {
			t.Fatalf("%s: note %v, want the branch choice at split", spelling, notes[0])
		}
		if got := outcome.String(); got != want[choice.Taken] {
			t.Fatalf("%s: outcome %q after taking %s, want %q", spelling, got, choice.Alternatives[choice.Taken], want[choice.Taken])
		}
	}
}

// A junction with two branches enabled, one of them on to a junction none of
// whose guards holds: the draw is still made, and the dead end is the outcome of
// the run that draws it alone, not a failure of the transition into the first.
func TestExploreJunctionBranchBeyondWhichNoGuardHolds(t *testing.T) {
	m := parseExploreModel(t, `package test {
		state def Machine {
			attribute level : Integer = 0;
			entry; then idle;
			state idle;
			junction split;
			junction stuck;
			state ready;
			state never;
			transition first idle accept go then split;
			transition first split then ready;
			transition first split then stuck;
			transition first stuck if level > 0 then never;
		}
	}`)
	sym := m.state(t, "Machine")
	run := func(ctx *Context) (Outcome, error) {
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
	}
	declared, err := ParseSchedulePolicy("declared")
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	if err := ctx.SetSchedule(declared); err != nil {
		t.Fatal(err)
	}
	outcome, err := run(ctx)
	if err != nil {
		t.Fatalf("declared: %v, want the branch into ready taken", err)
	}
	if got := outcome.String(); got != "finalState ready; visits idle, ready; level = 0" {
		t.Fatalf("declared: outcome %q, want ready reached", got)
	}

	policy, err := ParseSchedulePolicy("explore")
	if err != nil {
		t.Fatal(err)
	}
	x, err := Explore(context.Background(), policy, m.fresh, run)
	if err != nil {
		t.Fatal(err)
	}
	if !x.Complete() || x.Runs != 2 {
		t.Fatalf("status %q, want complete (2 runs)", x.Status())
	}
	got := outcomeTexts(x)
	if len(got) != 2 || got[1] != "finalState ready; visits idle, ready; level = 0" {
		t.Fatalf("outcomes %v, want the dead end at stuck and ready", got)
	}
	if !strings.HasPrefix(got[0], "error: ") || !strings.HasSuffix(got[0], "fire transition out of idle: evaluate pseudostate: junction stuck: no guard evaluated to true") {
		t.Fatalf("outcomes %v, want the first the dead end at stuck, met as the transition fires", got)
	}
	w := x.Outcomes[0].Witness
	if len(w) != 1 || w[0].Kind != ChoiceTransition || w[0].Where != "junction split" || !strings.HasSuffix(w[0].Took, "->stuck") {
		t.Fatalf("witness of the dead end %v, want the branch of junction split into stuck", w)
	}
}

// The branch drawn noted a guard it could not read at a junction on its way
// before it dead-ended: the run fails with the dead end and keeps the note.
func TestJunctionBranchDeadEndKeepsItsNotes(t *testing.T) {
	m := parseExploreModel(t, `package test {
		state def Machine {
			attribute d : Integer = 0;
			entry; then idle;
			state idle;
			junction split;
			junction nested;
			junction stuck;
			state ready;
			state other;
			state never;
			transition first idle accept go then split;
			transition first split then nested;
			transition first split then ready;
			transition first nested if d == 0 then stuck;
			transition first nested if 1 / d > 0 then other;
			transition first stuck if d > 0 then never;
		}
	}`)
	declared, err := ParseSchedulePolicy("declared")
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	if err := ctx.SetSchedule(declared); err != nil {
		t.Fatal(err)
	}
	exec, err := newStateExecutor(ctx, m.state(t, "Machine"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := exec.initialize(); err != nil {
		t.Fatal(err)
	}
	exec.SendSignal("go", nil)
	err = exec.RunToCompletion()
	if err == nil || !strings.Contains(err.Error(), "junction stuck: no guard evaluated to true") {
		t.Fatalf("RunToCompletion: %v, want the dead end at stuck", err)
	}
	var drew, noted bool
	for _, n := range ctx.Notes() {
		switch n := n.(type) {
		case ChoicePoint:
			drew = drew || n.Where == "junction split"
		case UnevaluableGuard:
			noted = noted || n.Where == "junction nested" && strings.Contains(n.Reason, "division by zero")
		}
	}
	if !drew || !noted {
		t.Fatalf("notes %v, want the draw at split and the guard at nested that could not be read", ctx.Notes())
	}
}

// A junction with two branches enabled in one region, whose incoming guard the
// other region's effect disarms: the branch is drawn only as the transition
// fires, after the region order, so a witness lists the order first and the run
// in which the disarming region fires first draws nothing at the junction.
// The regions' entry order is drawn ahead of both, doubling the runs. Every
// witness, and the choices every seed takes, replay to the same run.
func TestExploreJunctionDrawnAsTransitionFires(t *testing.T) {
	m := parseExploreModel(t, `package test {
		state def Machine {
			attribute armed : Boolean = true;
			attribute route : Integer = 0;
			entry; then work;
			state work parallel {
				state a {
					entry; then a1;
					state a1;
					state a2;
					transition first a1 accept go do assign armed := false then a2;
				}
				state b {
					entry; then b1;
					state b1;
					junction split;
					state left;
					state right;
					transition first b1 accept go if armed then split;
					transition first split do assign route := 1 then left;
					transition first split do assign route := 2 then right;
				}
			}
		}
	}`)
	policy, err := ParseSchedulePolicy("explore")
	if err != nil {
		t.Fatal(err)
	}
	sym := m.state(t, "Machine")
	run := func(ctx *Context) (Outcome, error) {
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
	}
	x, err := Explore(context.Background(), policy, m.fresh, run)
	if err != nil {
		t.Fatal(err)
	}
	if !x.Complete() || x.Runs != 6 {
		t.Fatalf("status %q, want complete (6 runs)", x.Status())
	}
	const enterA, enterB = "entering work: a1(entry) first of a1(entry), b1(entry); ", "entering work: b1(entry) first of a1(entry), b1(entry); "
	want := map[string]string{
		"finalState a2+b1; visits work, a1, b1, a2; armed = false; route = 0":           enterA + "on accept go: a1 first of a1, b1",
		"finalState a2+left; visits work, a1, b1, left, a2; armed = false; route = 1":   enterA + "on accept go: b1 first of a1, b1; junction split -> 1->left",
		"finalState a2+right; visits work, a1, b1, right, a2; armed = false; route = 2": enterA + "on accept go: b1 first of a1, b1; junction split -> 2->right",
		"finalState a2+b1; visits work, b1, a1, a2; armed = false; route = 0":           enterB + "on accept go: a1 first of a1, b1",
		"finalState a2+left; visits work, b1, a1, left, a2; armed = false; route = 1":   enterB + "on accept go: b1 first of a1, b1; junction split -> 1->left",
		"finalState a2+right; visits work, b1, a1, right, a2; armed = false; route = 2": enterB + "on accept go: b1 first of a1, b1; junction split -> 2->right",
	}
	for _, o := range x.Outcomes {
		witness, ok := want[o.Outcome.String()]
		if !ok {
			t.Fatalf("outcome %s, want one of %v", o.Outcome, slices.Sorted(maps.Keys(want)))
		}
		if got := FormatChoices(o.Witness); got != witness {
			t.Errorf("%s: witness %q, want %q", o.Outcome, got, witness)
		}
	}
	if len(x.Outcomes) != len(want) {
		t.Fatalf("%d outcomes, want %d", len(x.Outcomes), len(want))
	}
	assertWitnessesReplay(t, x, m.fresh, run)

	for _, spelling := range []string{"declared", "reverse", "seed:1", "seed:2", "seed:3", "seed:4"} {
		fixed, err := ParseSchedulePolicy(spelling)
		if err != nil {
			t.Fatal(err)
		}
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		mustSchedule(t, ctx, fixed)
		outcome, err := run(ctx)
		if err != nil {
			t.Fatalf("%s: %v", spelling, err)
		}
		var taken []ChoiceTaken
		for _, c := range ctx.Choices() {
			taken = append(taken, c.Choice())
		}
		// A policy keeping declaration order notes no entry draw.
		expected := want[outcome.String()]
		if spelling == "declared" || spelling == "reverse" {
			expected = strings.TrimPrefix(expected, enterA)
		}
		if got := FormatChoices(taken); got != expected {
			t.Errorf("%s: %s made the choices %q, want %q", spelling, outcome, got, expected)
		}
		if again, _, err := replayed(t, m.fresh, run, taken); err != nil {
			t.Errorf("%s: replaying its choices %q: %v", spelling, FormatChoices(taken), err)
		} else if again.String() != outcome.String() {
			t.Errorf("%s: replaying its choices reached %s, want %s", spelling, again, outcome)
		}
	}
}

// A history with no record whose default transition ends at a junction with two
// branches enabled: the draw is recorded as a choice point at the junction, as a
// junction reached from a transition is, so exploration enumerates both entries
// and a seed's run replays from its choices.
func TestExploreHistoryDefaultThroughJunction(t *testing.T) {
	m := parseExploreModel(t, `package test {
		state def Machine {
			attribute route : Integer = 0;
			entry; then idle;
			state idle;
			state work {
				entry; then w1;
				state w1;
				state w2;
				history resume;
				junction split;
				transition first resume then split;
				transition first split do assign route := 1 then w1;
				transition first split do assign route := 2 then w2;
			}
			transition first idle accept go then resume;
		}
	}`)
	policy, err := ParseSchedulePolicy("explore")
	if err != nil {
		t.Fatal(err)
	}
	sym := m.state(t, "Machine")
	run := func(ctx *Context) (Outcome, error) {
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
	}
	x, err := Explore(context.Background(), policy, m.fresh, run)
	if err != nil {
		t.Fatal(err)
	}
	if !x.Complete() || x.Runs != 2 {
		t.Fatalf("status %q, want complete (2 runs)", x.Status())
	}
	want := []string{
		"finalState w1; visits idle, work, w1; route = 1",
		"finalState w2; visits idle, work, w2; route = 2",
	}
	if got := outcomeTexts(x); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("outcomes %v, want %v", got, want)
	}
	for i, o := range x.Outcomes {
		if got, wantW := FormatChoices(o.Witness), fmt.Sprintf("junction split -> %d->w%d", i+1, i+1); got != wantW {
			t.Errorf("%s: witness %q, want %q", o.Outcome, got, wantW)
		}
	}
	assertWitnessesReplay(t, x, m.fresh, run)

	for _, spelling := range []string{"declared", "seed:1", "seed:2", "seed:3"} {
		fixed, err := ParseSchedulePolicy(spelling)
		if err != nil {
			t.Fatal(err)
		}
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		mustSchedule(t, ctx, fixed)
		outcome, err := run(ctx)
		if err != nil {
			t.Fatalf("%s: %v", spelling, err)
		}
		notes := ctx.Notes()
		if len(notes) != 1 {
			t.Fatalf("%s: notes %v, want the one choice at split", spelling, notes)
		}
		choice, ok := notes[0].(ChoicePoint)
		if !ok || choice.Kind != ChoiceTransition || choice.Where != "junction split" {
			t.Fatalf("%s: note %v, want the branch choice at split", spelling, notes[0])
		}
		if got := outcome.String(); got != want[choice.Taken] {
			t.Fatalf("%s: outcome %q after taking %s, want %q", spelling, got, choice.Alternatives[choice.Taken], want[choice.Taken])
		}
		if len(ctx.Choices()) != 1 {
			t.Fatalf("%s: choices %v, want the one at split", spelling, ctx.Choices())
		}
		if again, _, err := replayed(t, m.fresh, run, []ChoiceTaken{choice.Choice()}); err != nil {
			t.Errorf("%s: replaying its choice: %v", spelling, err)
		} else if again.String() != outcome.String() {
			t.Errorf("%s: replaying its choice reached %s, want %s", spelling, again, outcome)
		}
	}
}

// One event enabling a transition in each of two regions: the library orders
// neither first, so exploration fires them in both orders; every policy reports
// the order it took as one region-order choice point, `reverse` and `declared`
// taking region declaration order and a seed reaching the other order as well.
// The regions' entry order is drawn before, so exploration reaches each firing
// order from both entry orders. A change occurrence raising both regions'
// conditions at once is dispatched the same way.
func TestExploreSiblingRegionOrder(t *testing.T) {
	t.Run("event", func(t *testing.T) {
		m := parseExploreModel(t, `package test {
			private import ScalarValues::*;
			state def Machine {
				attribute last : Integer = 0;
				entry; then work;
				state work parallel {
					state a { entry; then a1; state a1; state a2; transition first a1 accept go do assign last := 1 then a2; }
					state b { entry; then b1; state b1; state b2; transition first b1 accept go do assign last := 2 then b2; }
				}
			}
		}`)
		checkSiblingRegionOrder(t, m, "go", "on accept go", []string{
			"finalState a2+b2; visits work, a1, b1, a2, b2; last = 2",
			"finalState a2+b2; visits work, a1, b1, b2, a2; last = 1",
			"finalState a2+b2; visits work, b1, a1, a2, b2; last = 2",
			"finalState a2+b2; visits work, b1, a1, b2, a2; last = 1",
		})
	})
	t.Run("change", func(t *testing.T) {
		m := parseExploreModel(t, `package test {
			private import ScalarValues::*;
			state def Machine {
				attribute temp : Integer = 0;
				attribute last : Integer = 0;
				entry; then start;
				state start;
				state work parallel {
					state a { entry; then a1; state a1; state a2; transition first a1 accept when temp > 20 do assign last := 1 then a2; }
					state b { entry; then b1; state b1; state b2; transition first b1 accept when temp > 20 do assign last := 2 then b2; }
				}
				transition first start do assign temp := 30 then work;
			}
		}`)
		checkSiblingRegionOrder(t, m, "", "on change", []string{
			"finalState a2+b2; visits start, work, a1, b1, a2, b2; last = 2; temp = 30",
			"finalState a2+b2; visits start, work, a1, b1, b2, a2; last = 1; temp = 30",
			"finalState a2+b2; visits start, work, b1, a1, a2, b2; last = 2; temp = 30",
			"finalState a2+b2; visits start, work, b1, a1, b2, a2; last = 1; temp = 30",
		})
	})
}

// checkSiblingRegionOrder runs Machine, sending signal if named, and checks that
// exploration reaches the four outcomes — a fired first then b first, under a
// entered first then b first — that the fixed policies take the first and report
// the firing order alone, and that seeds draw both firing orders; where spells
// the choice's trigger.
func checkSiblingRegionOrder(t *testing.T, m *exploreModel, signal, where string, want []string) {
	sym := m.state(t, "Machine")
	run := func(ctx *Context) (Outcome, error) {
		exec, err := newStateExecutor(ctx, sym, nil)
		if err != nil {
			return Outcome{}, err
		}
		if err := exec.initialize(); err != nil {
			return Outcome{}, err
		}
		if signal != "" {
			exec.SendSignal(signal, nil)
		}
		if err := exec.RunToCompletion(); err != nil {
			return Outcome{}, err
		}
		return exec.Outcome(), nil
	}
	policy, err := ParseSchedulePolicy("explore")
	if err != nil {
		t.Fatal(err)
	}
	x, err := Explore(context.Background(), policy, m.fresh, run)
	if err != nil {
		t.Fatal(err)
	}
	if !x.Complete() || x.Runs != 4 {
		t.Fatalf("status %q, want complete (4 runs)", x.Status())
	}
	if got := outcomeTexts(x); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("outcomes %v, want %v", got, want)
	}
	if got := FormatChoices(x.Outcomes[1].Witness); got != "entering work: a1(entry) first of a1(entry), b1(entry); "+where+": b1 first of a1, b1" {
		t.Fatalf("witness of last = 1 %q, want b1 chosen first", got)
	}
	if got := FormatChoices(x.Outcomes[2].Witness); got != "entering work: b1(entry) first of a1(entry), b1(entry); "+where+": a1 first of a1, b1" {
		t.Fatalf("witness of b entered first %q, want b1 entered first", got)
	}
	under := func(spelling string) (Outcome, ChoicePoint) {
		t.Helper()
		fixed, err := ParseSchedulePolicy(spelling)
		if err != nil {
			t.Fatal(err)
		}
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		if err := ctx.SetSchedule(fixed); err != nil {
			t.Fatal(err)
		}
		outcome, err := run(ctx)
		if err != nil {
			t.Fatalf("%s: %v", spelling, err)
		}
		var notes []ChoicePoint
		for _, note := range ctx.Notes() {
			if choice, ok := note.(ChoicePoint); ok && choice.Kind != ChoiceEntryOrder {
				notes = append(notes, choice)
			}
		}
		if len(notes) != 1 {
			t.Fatalf("%s: notes %v, want the one region-order choice beside the entry order", spelling, notes)
		}
		choice := notes[0]
		if choice.Kind != ChoiceRegionOrder || strings.Join(choice.Alternatives, ", ") != "a1, b1" {
			t.Fatalf("%s: note %v, want a region-order choice among a1, b1", spelling, notes[0])
		}
		return outcome, choice
	}
	for _, spelling := range []string{"reverse", "declared"} {
		outcome, choice := under(spelling)
		if got := outcome.String(); got != want[0] || choice.Taken != 0 {
			t.Fatalf("%s: outcome %q taking %d, want %q taking a1 first", spelling, got, choice.Taken, want[0])
		}
	}
	reached := make(map[int]string)
	for _, spelling := range []string{"seed:6", "seed:8"} {
		outcome, choice := under(spelling)
		if got := outcome.String(); got != want[choice.Taken] && got != want[2+choice.Taken] {
			t.Fatalf("%s: outcome %q after taking %s first, want %q or %q", spelling, got, choice.Alternatives[choice.Taken], want[choice.Taken], want[2+choice.Taken])
		}
		if again, _ := under(spelling); again.String() != outcome.String() {
			t.Fatalf("%s: outcome %q, then %q; want the seed to replay its run", spelling, outcome, again)
		}
		reached[choice.Taken] = spelling
	}
	if len(reached) != 2 {
		t.Fatalf("seeds reached only %v, want both region orders", reached)
	}
}

// Executors due at one instant are a due-order choice the exploration
// enumerates: three machines waking at t=5 come to their 6 orders, each
// witness naming the choice by kind, instant, alternatives and the one taken.
func TestExploreDueOrder(t *testing.T) {
	file := parseAndBuild(t, `
		package test {
			private import SI::*;
			private import ScalarValues::*;
			part def Cell { attribute mark : Integer = 0; }
			part cell : Cell;
			state def Ticker {
				attribute seen : Integer = -1;
				entry; then waiting;
				state waiting;
				accept after 5 [s] then took;
				state took {
					entry action take { assign seen := cell.mark; assign cell.mark := cell.mark + 1; }
				}
			}
			state a : Ticker;
			state b : Ticker;
			state c : Ticker;
		}
	`)
	idx, model, _ := buildRuntimeWithLibraries(t, "<test>", file)
	root := idx.DocumentRoot("<test>")
	resolver := resolve.New(idx)
	names := []string{"a", "b", "c"}
	syms := make([]*symbols.Symbol, len(names))
	for i, name := range names {
		syms[i] = namedOrFoundSymbol(t, idx, "test::"+name, root, ast.DefState, ast.UsageState)
	}
	fresh := func() (*Context, error) { return NewContext(NewModel(model, resolver), 10000), nil }
	run := func(ctx *Context) (Outcome, error) {
		execs := make([]*StateExecutor, len(syms))
		for i, sym := range syms {
			exec, err := ctx.CreateStateExecutor(sym)
			if err != nil {
				return Outcome{}, err
			}
			execs[i] = exec
		}
		if _, err := ctx.Advance(5); err != nil {
			return Outcome{}, err
		}
		outputs := make(map[string]Value, len(execs))
		for i, exec := range execs {
			outputs[names[i]] = exec.StateData()["seen"]
		}
		return ctx.ActionOutcome(outputs), nil
	}
	x, err := Explore(context.Background(), mustPolicy(t, "explore"), fresh, run)
	if err != nil {
		t.Fatal(err)
	}
	if !x.Complete() || x.Runs != 6 {
		t.Fatalf("status %q, want complete after the 6 orders of three machines", x.Status())
	}
	want := []string{
		"a = 0; b = 1; c = 2", "a = 0; b = 2; c = 1", "a = 1; b = 0; c = 2",
		"a = 1; b = 2; c = 0", "a = 2; b = 0; c = 1", "a = 2; b = 1; c = 0",
	}
	if got := outcomeTexts(x); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("outcomes %v, want %v", got, want)
	}
	for _, o := range x.Outcomes {
		if o.Linearizations != 1 {
			t.Errorf("%s reached by %d linearizations, want 1", o.Outcome, o.Linearizations)
		}
		w := o.Witness
		if len(w) != 2 || w[0].Kind != ChoiceDueOrder || w[0].Alternatives != 3 || w[1].Kind != ChoiceDueOrder || w[1].Alternatives != 2 {
			t.Errorf("%s witness %v, want a due-order choice among 3 then one among the 2 left", o.Outcome, w)
		}
	}
	const wantWitness = "t=5.0: state machine c first of state machine a, state machine b, state machine c; t=5.0: state machine b first of state machine a, state machine b"
	if got := FormatChoices(x.Outcomes[5].Witness); got != wantWitness {
		t.Fatalf("witness of a = 2; b = 1; c = 0:\n%s\nwant\n%s", got, wantWitness)
	}
	for _, spelling := range []string{"reverse", "declared", "seed:1"} {
		ctx, _ := fresh()
		if err := ctx.SetSchedule(mustPolicy(t, spelling)); err != nil {
			t.Fatal(err)
		}
		outcome, err := run(ctx)
		if err != nil {
			t.Fatalf("%s: %v", spelling, err)
		}
		if !slices.Contains(want, outcome.String()) {
			t.Errorf("%s: outcome %q is not one the exploration reached", spelling, outcome.String())
		}
	}
}

// conformanceMachine loads a state fixture of the conformance suite for
// exploring: fresh builds a context over it and run drives Machine, sending each
// of signals once it is started.
func conformanceMachine(t *testing.T, name string, signals ...string) (fresh func() (*Context, error), run func(*Context) (Outcome, error)) {
	t.Helper()
	path := filepath.Join("testdata", "conformance", name+".sysml")
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	idx, model, _ := buildRuntimeWithLibraries(t, path, parseAndBuild(t, string(text)))
	resolver := resolve.New(idx)
	sym := namedOrFoundSymbol(t, idx, "", idx.DocumentRoot(path), ast.DefState, ast.UsageState)
	fresh = func() (*Context, error) { return NewContext(NewModel(model, resolver), 10000), nil }
	run = func(ctx *Context) (Outcome, error) {
		exec, err := ctx.CreateStateExecutor(sym)
		if err != nil {
			return Outcome{}, err
		}
		if err := exec.RunToCompletion(); err != nil {
			return Outcome{}, err
		}
		for _, signal := range signals {
			exec.SendSignal(signal, nil)
			if err := exec.RunToCompletion(); err != nil {
				return Outcome{}, err
			}
		}
		return exec.Outcome(), nil
	}
	return fresh, run
}

// Entering a parallel state draws the order of its regions' units: two regions
// of two logged entries each interleave six ways, each reached by exactly one
// run, and no run draws more than the front's bound of Σuᵢ − u_last = 3 times.
// Every witness replays to its outcome, a second exploration is the same, and
// the fixed policies keep declaration order, `reverse` and `declared` without a
// choice, `seed:1` reporting the one it drew.
func TestExploreEntryOrder(t *testing.T) {
	fresh, run := conformanceMachine(t, "state_region_entry_order", "Go")
	x, err := Explore(context.Background(), mustPolicy(t, "explore"), fresh, run)
	if err != nil {
		t.Fatal(err)
	}
	if !x.Complete() || x.Runs != 6 || len(x.Outcomes) != 6 {
		t.Fatalf("status %q with %d outcomes, want complete (6 runs) reaching 6", x.Status(), len(x.Outcomes))
	}
	logs := make([]string, 0, len(x.Outcomes))
	for _, o := range x.Outcomes {
		if o.Linearizations != 1 {
			t.Errorf("%s reached by %d linearizations, want 1", o.Outcome, o.Linearizations)
		}
		if draws := len(o.Witness); draws < 2 || draws > 3 {
			t.Errorf("%s drew %d times, want 2 or 3: two two-unit queues draw at most Σuᵢ − u_last", o.Outcome, draws)
		}
		for _, c := range o.Witness {
			if c.Kind != ChoiceEntryOrder || c.Where != "entering work" {
				t.Errorf("%s drew %s, want every draw an entry order entering work", o.Outcome, c)
			}
		}
		logs = append(logs, o.Outcome.String())
	}
	if len(slices.Compact(slices.Clone(logs))) != 6 {
		t.Fatalf("outcomes %v, want six distinct interleavings", logs)
	}
	assertWitnessesReplay(t, x, fresh, run)
	again, err := Explore(context.Background(), mustPolicy(t, "explore"), fresh, run)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(outcomeTexts(again), "|") != strings.Join(logs, "|") || again.Runs != x.Runs {
		t.Fatalf("a second exploration found %v in %d runs, want the same %v in %d", outcomeTexts(again), again.Runs, logs, x.Runs)
	}
	const declaredOrder = "outerL(entry) innerL(entry) outerR(entry) innerR(entry) "
	for _, spelling := range []string{"reverse", "declared", "seed:1"} {
		ctx, _ := fresh()
		mustSchedule(t, ctx, mustPolicy(t, spelling))
		outcome, err := run(ctx)
		if err != nil {
			t.Fatalf("%s: %v", spelling, err)
		}
		var drawn []ChoiceKind
		for _, c := range ctx.Choices() {
			drawn = append(drawn, c.Choice().Kind)
		}
		switch spelling {
		case "seed:1":
			if !slices.Contains(logs, outcome.String()) || !slices.Contains(drawn, ChoiceEntryOrder) {
				t.Errorf("seed:1 reached %s drawing %v, want an explored outcome with the entry order reported", outcome, drawn)
			}
		default:
			if !strings.Contains(outcome.String(), declaredOrder) || slices.Contains(drawn, ChoiceEntryOrder) {
				t.Errorf("%s reached %s drawing %v, want declaration order without an entry-order choice", spelling, outcome, drawn)
			}
		}
	}
}

// A fork's branches are drawn the same way, the shared owner entered once: each
// branch's effect precedes the owner's entry, which the first branch to reach it
// enters, and the branch targets follow in the order drawn. Ten linearizations
// reach eight outcomes: both branches at the owner's entry is one unit, not a
// draw, and a target entered before or after the other branch's effect leaves
// no mark on the log.
func TestExploreForkBranchOrder(t *testing.T) {
	fresh, run := conformanceMachine(t, "state_fork_branch_order")
	x, err := Explore(context.Background(), mustPolicy(t, "explore"), fresh, run)
	if err != nil {
		t.Fatal(err)
	}
	if !x.Complete() || x.Runs != 10 || len(x.Outcomes) != 8 {
		t.Fatalf("status %q with %d outcomes, want complete (10 runs) reaching 8", x.Status(), len(x.Outcomes))
	}
	for _, o := range x.Outcomes {
		for _, c := range o.Witness {
			if c.Kind != ChoiceEntryOrder || c.Where != "fork split" {
				t.Errorf("%s drew %s, want every draw an entry order at fork split", o.Outcome, c)
			}
		}
	}
	assertWitnessesReplay(t, x, fresh, run)
}

// A body paused on the clock whose wait is over is a token able to act, so it is
// an alternative to a sibling parked accept due at the same instant, not work
// swept up after the sibling has acted.
func TestExplorePausedBodyDueIsAMove(t *testing.T) {
	path := filepath.Join("testdata", "conformance", "action_explore_performed_and_accept_due_together.sysml")
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	idx, model, _ := buildRuntimeWithLibraries(t, path, parseAndBuild(t, string(text)))
	resolver := resolve.New(idx)
	sym := namedOrFoundSymbol(t, idx, "test::wake", idx.DocumentRoot(path), ast.DefAction, ast.UsageAction)
	fresh := func() (*Context, error) { return NewContext(NewModel(model, resolver), 10000), nil }
	run := func(ctx *Context) (Outcome, error) {
		outputs, err := ctx.ExecuteAction(sym)
		if err != nil {
			return Outcome{}, err
		}
		return ctx.ActionOutcome(outputs), nil
	}
	x, err := Explore(context.Background(), mustPolicy(t, "explore"), fresh, run)
	if err != nil {
		t.Fatal(err)
	}
	if !x.Complete() || x.Runs != 6 {
		t.Fatalf("status %q, want complete after the 6 interleavings of two chains of two moves", x.Status())
	}
	want := []string{"x = 1", "x = 2"}
	if got := outcomeTexts(x); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("outcomes %v, want %v", got, want)
	}
	for _, o := range x.Outcomes {
		if o.Linearizations != 3 {
			t.Errorf("%s reached by %d linearizations, want 3", o.Outcome, o.Linearizations)
		}
	}
	const wantWitness = "step 3: 2@performed first of 2@performed, 3@direct; step 4: 2@writeOne first of 2@writeOne, 3@direct"
	if got := FormatChoices(x.Outcomes[1].Witness); got != wantWitness {
		t.Fatalf("witness of x = 2:\n%s\nwant\n%s", got, wantWitness)
	}
	// The fixed sweeps resume the paused body after the sibling has acted; the
	// writes then run in the policy's order, so the one it steps last stands.
	fixed := map[string]string{"reverse": "x = 1", "declared": "x = 2", "seed:1": "x = 1"}
	for _, spelling := range []string{"reverse", "declared", "seed:1"} {
		ctx, _ := fresh()
		if err := ctx.SetSchedule(mustPolicy(t, spelling)); err != nil {
			t.Fatal(err)
		}
		outcome, err := run(ctx)
		if err != nil {
			t.Fatalf("%s: %v", spelling, err)
		}
		if got := outcome.String(); got != fixed[spelling] {
			t.Errorf("%s: outcome %q, want %s", spelling, got, fixed[spelling])
		}
	}
}

func TestExploreRejectsOtherPolicies(t *testing.T) {
	m := parseExploreModel(t, threeWritersModel)
	_, err := Explore(context.Background(), DefaultSchedulePolicy, m.fresh, func(*Context) (Outcome, error) { return Outcome{}, nil })
	if !errors.Is(err, ErrNotExploring) {
		t.Fatalf("Explore under %s: %v, want ErrNotExploring", DefaultSchedulePolicy, err)
	}
	ctx, _ := m.fresh()
	policy, _ := ParseSchedulePolicy("explore")
	if err := ctx.SetSchedule(policy); !errors.Is(err, ErrExploreUndriven) {
		t.Fatalf("SetSchedule(explore): %v, want ErrExploreUndriven", err)
	}
}

// A caller that goes away between runs ends the exploration with its error
// before the next context is built; no partial outcome set is reported.
func TestExploreStopsWhenTheCallerGoesAway(t *testing.T) {
	m := parseExploreModel(t, threeWritersModel)
	sym := m.action(t, "race")
	stop, cancel := context.WithCancel(context.Background())
	defer cancel()
	runs, built := 0, 0
	fresh := func() (*Context, error) {
		built++
		return m.fresh()
	}
	x, err := Explore(stop, mustPolicy(t, "explore"), fresh, func(ctx *Context) (Outcome, error) {
		runs++
		if runs == 2 {
			cancel()
		}
		outputs, err := ctx.ExecuteAction(sym)
		if err != nil {
			return Outcome{}, err
		}
		return ctx.ActionOutcome(outputs), nil
	})
	if !errors.Is(err, context.Canceled) || x != nil {
		t.Fatalf("Explore after cancel: %v, %v; want context.Canceled and no exploration", x, err)
	}
	if runs != 2 || built != 2 {
		t.Fatalf("%d runs of %d contexts, want 2 of 2: no third context built", runs, built)
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
	x, err := Explore(context.Background(), policy, func() (*Context, error) {
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
		return fleetOutcome(ctx, self, "lead", "pings")
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
	if got := x.Outcomes[0].Outcome.String(); got != "lead = test::Rover#1{}; pings = 1" {
		t.Fatalf("outcome %q, want lead bound to the scout and pings = 1", got)
	}
}

// fleetOutcome is the outcome of a run on a Fleet: the values self holds under names.
func fleetOutcome(ctx *Context, self *Instance, names ...string) (Outcome, error) {
	outputs := make(map[string]Value, len(names))
	for _, name := range names {
		fv, err := self.GetFeatureValue(ctx, name)
		if err != nil {
			return Outcome{}, err
		}
		outputs[name] = fv.Value
	}
	return ctx.ActionOutcome(outputs), nil
}

// fleetModel is a Fleet whose machine takes one of two transitions on `go`, each
// binding the fleet's lead and backup to its vehicles in the order it states.
func fleetModel(t *testing.T, transitions string) *exploreModel {
	t.Helper()
	return parseExploreModel(t, `package test {
		private import ScalarValues::*;
		part def Vehicle;
		part def Scout :> Vehicle { attribute id : Integer = 1; }
		part def Rover :> Vehicle { attribute id : Integer = 2; }
		part def Fleet {
			part scout : Scout;
			part rover : Rover;
			ref part lead : Vehicle;
			ref part backup : Vehicle;
			attribute picks : Integer = 0;
			exhibit state run {
				entry; then idle;
				state idle;
				state done;
				`+transitions+`
			}
		}
	}`)
}

// exploreFleet drives the fleet's machine through `go` once per linearization,
// reporting the id lead held in each run.
func exploreFleet(t *testing.T, m *exploreModel) (*Exploration, []int64) {
	t.Helper()
	policy, err := ParseSchedulePolicy("explore")
	if err != nil {
		t.Fatal(err)
	}
	fleet := oneSymbol(t, m.idx, "test::Fleet")
	stateSym := oneSymbol(t, m.idx, "test::Fleet::run")
	var leadIDs []int64
	x, err := Explore(context.Background(), policy, m.fresh, func(ctx *Context) (Outcome, error) {
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
		if err := exec.RunToCompletion(); err != nil {
			return Outcome{}, err
		}
		outcome, err := fleetOutcome(ctx, self, "lead", "backup", "picks")
		if err == nil {
			leadIDs = append(leadIDs, outcome.Outputs["lead"].Instance)
		}
		return outcome, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if !x.Complete() || x.Runs != 2 {
		t.Fatalf("status %q, want complete (2 runs)", x.Status())
	}
	return x, leadIDs
}

// Two runs binding lead to objects of two types are two outcomes even though each
// run gives its object the same id, which is not the outcome's to compare.
func TestExploreTellsObjectsApartByWhatTheyAre(t *testing.T) {
	m := fleetModel(t, `
		transition idle_scout first idle accept go do { assign lead := scout; assign backup := scout; assign picks := picks + 1; } then done;
		transition idle_rover first idle accept go do { assign lead := rover; assign backup := scout; assign picks := picks + 1; } then done;`)
	x, leadIDs := exploreFleet(t, m)
	if leadIDs[0] != leadIDs[1] {
		t.Fatalf("lead ids %v, want the same id in both runs (each run's first object)", leadIDs)
	}
	want := []string{
		"backup = test::Scout#1{id = 1}; lead = #1; picks = 1",
		"backup = test::Scout#1{id = 1}; lead = test::Rover#2{id = 2}; picks = 1",
	}
	if got := outcomeTexts(x); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("outcomes %v, want %v", got, want)
	}
	for _, o := range x.Outcomes {
		if o.Linearizations != 1 {
			t.Errorf("%s reached by %d linearizations, want 1", o.Outcome, o.Linearizations)
		}
	}
}

// Two runs binding lead and backup to the same two objects in either order give
// those objects different ids, which does not make them two outcomes.
func TestExploreEquatesObjectsByWhatTheyAre(t *testing.T) {
	m := fleetModel(t, `
		transition lead_first first idle accept go do { assign lead := scout; assign backup := rover; assign picks := picks + 1; } then done;
		transition backup_first first idle accept go do { assign backup := rover; assign lead := scout; assign picks := picks + 1; } then done;`)
	x, leadIDs := exploreFleet(t, m)
	if leadIDs[0] == leadIDs[1] {
		t.Fatalf("lead ids %v, want different ids (the scout is the first object of one run and the second of the other)", leadIDs)
	}
	want := []string{"backup = test::Rover#1{id = 2}; lead = test::Scout#2{id = 1}; picks = 1"}
	if got := outcomeTexts(x); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("outcomes %v, want %v", got, want)
	}
	if x.Outcomes[0].Linearizations != 2 {
		t.Fatalf("reached by %d linearizations, want 2", x.Outcomes[0].Linearizations)
	}
}

// A name spelling the rendering's delimiters can make two outcomes render alike;
// their identities still tell them apart.
func TestOutcomeIdentityQuotesNames(t *testing.T) {
	one := Outcome{Outputs: map[string]Value{`a = "1"; b`: NewStringValue("2")}}
	two := Outcome{Outputs: map[string]Value{"a": NewStringValue("1"), "b": NewStringValue("2")}}
	if one.String() != two.String() {
		t.Fatalf("renderings %q and %q, want alike", one, two)
	}
	if one.identity() == two.identity() {
		t.Fatalf("identity %q shared, want two", one.identity())
	}
	same := Outcome{Outputs: map[string]Value{"b": NewStringValue("2"), "a": NewStringValue("1")}}
	if same.identity() != two.identity() {
		t.Fatalf("identities %q and %q, want one", same.identity(), two.identity())
	}
}

// Two typed regions' states share a name, so the entry-order and region-order
// choices spell each alternative with the region it sits in.
func TestExploreSiblingRegionOrderNamesTypedRegions(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		state def Region { entry; then r1; state r1; state r2; transition first r1 accept go then r2; }
		state def Machine {
			entry; then work;
			state work parallel {
				state a : Region;
				state b : Region;
			}
		}
	}`)
	sym := m.state(t, "Machine")
	policy, err := ParseSchedulePolicy("explore")
	if err != nil {
		t.Fatal(err)
	}
	x, err := Explore(context.Background(), policy, m.fresh, func(ctx *Context) (Outcome, error) {
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
	if !x.Complete() || x.Runs != 4 || len(x.Outcomes) != 1 || x.Outcomes[0].Linearizations != 4 {
		t.Fatalf("status %q with %d outcomes, want complete (4 runs) reaching one outcome four times", x.Status(), len(x.Outcomes))
	}
	if got := FormatChoices(x.Outcomes[0].Witness); got != "entering work: a.r1(entry) first of a.r1(entry), b.r1(entry); on accept go: a.r1 first of a.r1, b.r1" {
		t.Fatalf("witness %q, want the regions naming the alternatives", got)
	}
}

// Two active regions of one name are spelled in declaration order, so the final
// state is the same on every run.
func TestFinalStateNameOrdersRegionsOfOneNameByDeclaration(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		state def Machine {
			entry; then work;
			state work parallel {
				state left {
					entry; then inner;
					state inner parallel {
						state r { entry; then l1; state l1; }
						state s { entry; then l2; state l2; }
					}
				}
				state right {
					entry; then inner;
					state inner parallel {
						state r { entry; then r1; state r1; }
						state s { entry; then r2; state r2; }
					}
				}
			}
		}
	}`)
	sym := m.state(t, "Machine")
	for run := 0; run < 20; run++ {
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		exec, err := newStateExecutor(ctx, sym, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := exec.initialize(); err != nil {
			t.Fatal(err)
		}
		if err := exec.RunToCompletion(); err != nil {
			t.Fatal(err)
		}
		if got := exec.FinalStateName(); got != "inner+l1+r1+inner+l2+r2" {
			t.Fatalf("run %d: final state %q, want inner+l1+r1+inner+l2+r2", run, got)
		}
	}
}

// linkedNodes instantiates n nodes of one type chained by `next`, the last
// pointing back at the first, and returns the head as an outcome.
func linkedNodes(t *testing.T, m *exploreModel, ids ...int64) Outcome {
	t.Helper()
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	node := oneSymbol(t, m.idx, "test::Node")
	nodes := make([]*Instance, len(ids))
	for i, id := range ids {
		if nodes[i], err = ctx.Instantiate(node); err != nil {
			t.Fatal(err)
		}
		value := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: id}}
		if err := nodes[i].SetFeatureValue(ctx, "id", value); err != nil {
			t.Fatal(err)
		}
	}
	for i, inst := range nodes {
		next := nodes[(i+1)%len(nodes)]
		if err := inst.SetFeatureValue(ctx, "next", Value{Kind: ValInstance, Instance: next.ID}); err != nil {
			t.Fatal(err)
		}
	}
	return ctx.ActionOutcome(map[string]Value{"head": {Kind: ValInstance, Instance: nodes[0].ID}})
}

// A cycle of objects is spelled once around, the object closing it named by its
// number; nodes of one type nested past the rendering's depth still count toward
// the identity, so two rings rendered alike are two outcomes when they differ.
func TestOutcomeIdentityOpensEveryObject(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		part def Node { attribute id : Integer = 0; ref part next : Node; }
	}`)
	ring := linkedNodes(t, m, 1, 2)
	if got, want := ring.String(), "head = test::Node#1{id = 1, next = test::Node#2{id = 2, next = #1}}"; got != want {
		t.Fatalf("ring %q, want %q", got, want)
	}
	if got, want := ring.identity(), `finalState ""; "head" = "test::Node"#1{"id" = 1, "next" = "test::Node"#2{"id" = 2, "next" = #1}}`; got != want {
		t.Fatalf("ring identity %q, want %q", got, want)
	}
	if again := linkedNodes(t, m, 1, 2); again.identity() != ring.identity() {
		t.Fatalf("identities %q and %q, want one", again.identity(), ring.identity())
	}
	if loop := linkedNodes(t, m, 1, 1); loop.identity() == ring.identity() {
		t.Fatalf("a self-loop and a ring share identity %q", loop.identity())
	}
	long := linkedNodes(t, m, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
	other := linkedNodes(t, m, 1, 2, 3, 4, 5, 6, 7, 8, 9, 11)
	if !strings.HasSuffix(long.String(), "{…}}}}}}}}}") || long.String() != other.String() {
		t.Fatalf("renderings %q and %q, want alike and cut at depth", long, other)
	}
	if long.identity() == other.identity() {
		t.Fatalf("identity %q shared by two rings differing past the rendering's depth", long.identity())
	}
}

// An outcome built without a context spells an object by the id its run gave it.
func TestOutcomeWithoutContextKeepsObjectIDs(t *testing.T) {
	o := Outcome{Outputs: map[string]Value{"lead": {Kind: ValInstance, Instance: 7}}}
	if got := o.String(); got != "lead = instance(7)" {
		t.Fatalf("outcome %q, want lead = instance(7)", got)
	}
}

// mustSchedule sets a policy a context can run under, failing the test otherwise.
func mustSchedule(t *testing.T, ctx *Context, policy SchedulePolicy) {
	t.Helper()
	if err := ctx.SetSchedule(policy); err != nil {
		t.Fatalf("schedule %s: %v", policy, err)
	}
}
