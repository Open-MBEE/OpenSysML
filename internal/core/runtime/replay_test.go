package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Every kind of choice a witness lists reads back from the line that spells it.
func TestParseChoiceReadsEveryKind(t *testing.T) {
	cases := []struct {
		line string
		want ChoiceTaken
	}{
		{"step 3: 2@left first of 2@left, 3@right", ChoiceTaken{Kind: ChoiceTokenOrder, Step: 3, Alternatives: 2, Taken: 0, Among: []string{"2@left", "3@right"}, Took: "2@left"}},
		{"step 3: 3@right first of 2@left, 3@right", ChoiceTaken{Kind: ChoiceTokenOrder, Step: 3, Alternatives: 2, Taken: 1, Among: []string{"2@left", "3@right"}, Took: "3@right"}},
		{"step 5: decision select -> 2->alarm", ChoiceTaken{Kind: ChoiceDecisionBranch, Step: 5, Where: "decision select", Took: "2->alarm"}},
		{"state idle on accept go -> 2->right", ChoiceTaken{Kind: ChoiceTransition, Where: "state idle on accept go", Took: "2->right"}},
		{"on accept go: b1 first of a1, b1", ChoiceTaken{Kind: ChoiceRegionOrder, Where: "on accept go", Alternatives: 2, Taken: 1, Among: []string{"a1", "b1"}, Took: "b1"}},
		{"t=5.0: state machine c first of state machine a, state machine b, state machine c", ChoiceTaken{Kind: ChoiceDueOrder, Where: "t=5.0", Alternatives: 3, Taken: 2, Among: []string{"state machine a", "state machine b", "state machine c"}, Took: "state machine c"}},
	}
	for _, c := range cases {
		got, err := ParseChoice(c.line)
		if err != nil {
			t.Errorf("%q: %v", c.line, err)
			continue
		}
		if got.String() != c.line {
			t.Errorf("%q read back as %q", c.line, got)
		}
		if got.Kind != c.want.Kind || got.Step != c.want.Step || got.Where != c.want.Where || got.Alternatives != c.want.Alternatives ||
			got.Taken != c.want.Taken || strings.Join(got.Among, "|") != strings.Join(c.want.Among, "|") || got.Took != c.want.Took {
			t.Errorf("%q: %+v, want %+v", c.line, got, c.want)
		}
	}
}

// A line that spells no choice is a typed error naming the line and why; a
// witness is read a line at a time, or as FormatChoices joins it.
func TestParseChoicesRejectsWhatSpellsNoChoice(t *testing.T) {
	for _, text := range []string{"step 0: a first of a, b", "step x: a first of a, b", "a first of b, c", "c first of a, b", "step 2: c first of a, b", "-> x", "x ->", "nonsense"} {
		_, err := ParseChoices("step 1: a first of a, b\n" + text)
		var typed *ChoiceParseError
		if !errors.As(err, &typed) || !errors.Is(err, ErrInvalidChoice) {
			t.Errorf("%q: error %T %v, want a ChoiceParseError", text, err, err)
			continue
		}
		if typed.Line != 2 || typed.Text != text || !strings.Contains(err.Error(), "line 2") {
			t.Errorf("%q: error %q does not name line 2", text, err)
		}
	}
	joined, err := ParseChoices("step 1: a first of a, b; step 2: decision d -> 1->x\n\nno choice points\n")
	if err != nil {
		t.Fatal(err)
	}
	if FormatChoices(joined) != "step 1: a first of a, b; step 2: decision d -> 1->x" {
		t.Fatalf("read %v", joined)
	}
	none, err := ParseChoices("no choice points\n")
	if err != nil || len(none) != 0 {
		t.Fatalf("no choice points read as %v, %v", none, err)
	}
}

// A witness file may carry the run's trace after its header, separated by one blank
// line: the header is read and the trace, which spells no choice, is ignored.
func TestParseChoicesStopsAtBlankLine(t *testing.T) {
	text := "\n\nstep 1: 2@b first of 1@a, 2@b\nstep 2: decision d -> 1->x\n\n" +
		"[step 1] token 2 at b\n[step 2] decision d took 1->x\nnonsense that is no choice\n"
	choices, err := ParseChoices(text)
	if err != nil {
		t.Fatal(err)
	}
	if FormatChoices(choices) != "step 1: 2@b first of 1@a, 2@b; step 2: decision d -> 1->x" {
		t.Fatalf("read %v", choices)
	}
	if _, err := ParseChoices("step 1: 2@b first of 1@a, 2@b\nnonsense\n"); err == nil {
		t.Fatal("a line spelling no choice inside the header was accepted")
	}
}

// `replay:<file>` reads the file when the policy is parsed: a missing file, an
// unreadable line, a file naming no move or no file at all is a typed policy
// error; the policy spells its file back and hands out its witness.
func TestParseReplayPolicy(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "witness.txt")
	if err := os.WriteFile(file, []byte("step 3: 3@c first of 1@a, 2@b, 3@c\nstep 4: 2@b first of 1@a, 2@b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	policy, err := ParseSchedulePolicy("replay:" + file)
	if err != nil {
		t.Fatal(err)
	}
	if got := policy.String(); got != "replay:"+file {
		t.Errorf("String() = %q", got)
	}
	if policy.IsDefault() {
		t.Error("a replay is not the default policy")
	}
	choices, ok := policy.Replay()
	if !ok || len(choices) != 2 || choices[1].Took != "2@b" {
		t.Fatalf("Replay() = %v, %v", choices, ok)
	}
	if _, ok := DefaultSchedulePolicy.Replay(); ok {
		t.Error("reverse hands out a witness")
	}
	bad := filepath.Join(dir, "bad.txt")
	if err := os.WriteFile(bad, []byte("step 3: 3@c first of 1@a, 2@b, 3@c\nnonsense\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	moveless := filepath.Join(dir, "moveless.txt")
	if err := os.WriteFile(moveless, []byte("no choice points\n\n[trace] step 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	spellings := []string{"replay", "replay:", "replay:" + filepath.Join(dir, "missing.txt"), "replay:" + bad,
		"replay:" + empty, "replay:" + moveless}
	for _, spelling := range spellings {
		_, err := ParseSchedulePolicy(spelling)
		var typed *SchedulePolicyError
		if !errors.As(err, &typed) || !errors.Is(err, ErrInvalidSchedulePolicy) || typed.Spelling != spelling {
			t.Errorf("%q: error %T %v, want a SchedulePolicyError naming the spelling", spelling, err, err)
		}
	}
	if _, err := ParseSchedulePolicy("replay:" + bad); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Errorf("an unreadable witness line is not named: %v", err)
	}
	for _, file := range []string{empty, moveless} {
		if _, err := ParseSchedulePolicy("replay:" + file); err == nil || !strings.Contains(err.Error(), "names no move to follow") {
			t.Errorf("%s: a witness naming no move is not refused as such: %v", file, err)
		}
	}
	if choices, err := ParseChoices("no choice points\n"); err != nil || len(choices) != 0 {
		t.Errorf("ParseChoices(no choice points) = %v, %v; want no choices and no error", choices, err)
	}
	if got := SchedulePolicyNames[len(SchedulePolicyNames)-1]; got != "replay:<file>" {
		t.Errorf("SchedulePolicyNames ends with %q", got)
	}
}

// replayed runs one run of the model under the witness, as the exploration ran it.
func replayed(t *testing.T, fresh func() (*Context, error), run func(*Context) (Outcome, error), witness []ChoiceTaken) (Outcome, []ChoiceTaken, error) {
	t.Helper()
	ctx, err := fresh()
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, ReplayPolicy(witness))
	outcome, err := run(ctx)
	if err == nil {
		err = ctx.Unfollowed()
	}
	var choices []ChoiceTaken
	for _, c := range ctx.Choices() {
		choices = append(choices, c.Choice())
	}
	return outcome, choices, err
}

// choiceKinds lists the kind of each choice, in order.
func choiceKinds(choices []ChoiceTaken) []ChoiceKind {
	kinds := make([]ChoiceKind, len(choices))
	for i, c := range choices {
		kinds[i] = c.Kind
	}
	return kinds
}

// assertWitnessesReplay checks every outcome of an exploration: the run under its
// witness reaches the outcome, and its choices are the witness's lines.
func assertWitnessesReplay(t *testing.T, x *Exploration, fresh func() (*Context, error), run func(*Context) (Outcome, error)) {
	t.Helper()
	for _, o := range x.Outcomes {
		outcome, choices, err := replayed(t, fresh, run, o.Witness)
		if err != nil {
			t.Errorf("%s: replaying %s: %v", o.Outcome, FormatChoices(o.Witness), err)
			continue
		}
		if outcome.String() != o.Outcome.String() {
			t.Errorf("replaying %s reached %s, want %s", FormatChoices(o.Witness), outcome, o.Outcome)
		}
		if got, want := FormatChoices(choices), FormatChoices(o.Witness); got != want {
			t.Errorf("replaying %s made the choices\n%s", want, got)
		}
	}
}

// Every witness an exploration of an action writes replays to its outcome with
// the witness's choices: token orders, and decisions in a loop.
func TestReplayFollowsActionWitnesses(t *testing.T) {
	for name, text := range map[string]string{"race": threeWritersModel, "route": choiceModel, "count": decisionLoopModel} {
		m := parseExploreModel(t, text)
		x := m.exploreAction(t, "explore", name)
		if !x.Complete() {
			t.Fatalf("%s: status %q, want complete", name, x.Status())
		}
		sym := m.action(t, name)
		run := func(ctx *Context) (Outcome, error) {
			outputs, err := ctx.ExecuteAction(sym)
			if err != nil {
				return Outcome{}, err
			}
			return ctx.ActionOutcome(outputs), nil
		}
		assertWitnessesReplay(t, x, m.fresh, run)
	}
}

// decisionLoopModel branches on two overlapping guards inside a merge loop.
const decisionLoopModel = `package test {
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
}`

// A witness written to a file and read back through `replay:<file>` is the same
// run: the trace of the replay is the trace exploration recorded for it.
func TestReplayFileReproducesTheExploredRun(t *testing.T) {
	m := parseExploreModel(t, choiceModel)
	x := m.exploreAction(t, "explore", "route")
	sym := m.action(t, "route")
	run := func(ctx *Context, policy SchedulePolicy) (string, map[string]Value) {
		t.Helper()
		mustSchedule(t, ctx, policy)
		exec, err := ctx.CreateActionExecutor(sym)
		if err != nil {
			t.Fatal(err)
		}
		trace := NewTraceRecorder()
		exec.SetTrace(trace)
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("%s: %v", policy, err)
		}
		if err := ctx.Unfollowed(); err != nil {
			t.Fatalf("%s: %v", policy, err)
		}
		return trace.String(), exec.Results()
	}
	for i, o := range x.Outcomes {
		file := filepath.Join(t.TempDir(), "witness.txt")
		lines := make([]string, len(o.Witness))
		for j, c := range o.Witness {
			lines[j] = c.String()
		}
		if err := os.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		policy := mustPolicy(t, "replay:"+file)
		ctx, _ := m.fresh()
		trace, outputs := run(ctx, policy)
		if got := ctx.ActionOutcome(outputs).String(); got != o.Outcome.String() {
			t.Errorf("outcome %d: replay of %s reached %s, want %s", i, FormatChoices(o.Witness), got, o.Outcome)
		}
		again, _ := m.fresh()
		if second, _ := run(again, policy); second != trace {
			t.Errorf("outcome %d: two replays of one file differ\n%s\n---\n%s", i, trace, second)
		}
		if !strings.Contains(trace, "choice step") {
			t.Errorf("outcome %d: the replay records no choice lines\n%s", i, trace)
		}
	}
}

// State machine witnesses replay too: a transition conflict, sibling regions
// reacting to one event and executors due at one instant.
func TestReplayFollowsStateWitnesses(t *testing.T) {
	t.Run("transition", func(t *testing.T) {
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
		sym := m.state(t, "Machine")
		run := stateRun(sym, "go")
		x, err := Explore(context.Background(), mustPolicy(t, "explore"), m.fresh, run)
		if err != nil || !x.Complete() || x.Runs != 2 {
			t.Fatalf("explore: %v, %v", x, err)
		}
		assertWitnessesReplay(t, x, m.fresh, run)
	})
	// A choice's branches are read after the incoming effect; the witness names
	// the branch taken as the run's choice line does.
	t.Run("dynamic choice", func(t *testing.T) {
		m := parseExploreModel(t, `package test {
			private import ScalarValues::*;
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
		sym := m.state(t, "Machine")
		run := stateRun(sym, "go")
		x, err := Explore(context.Background(), mustPolicy(t, "explore"), m.fresh, run)
		if err != nil || !x.Complete() || x.Runs != 2 {
			t.Fatalf("explore: %v, %v", x, err)
		}
		for _, o := range x.Outcomes {
			if len(o.Witness) != 1 || o.Witness[0].Kind != ChoiceTransition || o.Witness[0].Where != "choice pick" {
				t.Fatalf("witness of %s is %s, want the one branch choice at pick", o.Outcome, FormatChoices(o.Witness))
			}
		}
		assertWitnessesReplay(t, x, m.fresh, run)
	})
	t.Run("regions", func(t *testing.T) {
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
		sym := m.state(t, "Machine")
		run := stateRun(sym, "go")
		x, err := Explore(context.Background(), mustPolicy(t, "explore"), m.fresh, run)
		if err != nil || !x.Complete() || x.Runs != 2 {
			t.Fatalf("explore: %v, %v", x, err)
		}
		assertWitnessesReplay(t, x, m.fresh, run)
	})
	// A dispatch draws every region's transition before the order they fire in, and
	// notes each with its firing; the witness lists the draws, the run the firings.
	t.Run("regions with a conflict", func(t *testing.T) {
		m := parseExploreModel(t, `package test {
			private import ScalarValues::*;
			state def Machine {
				attribute last : Integer = 0;
				entry; then work;
				state work parallel {
					state a { entry; then a1; state a1; state a2; state a3;
						transition first a1 accept go do assign last := 1 then a2;
						transition first a1 accept go do assign last := 2 then a3; }
					state b { entry; then b1; state b1; state b2; transition first b1 accept go do assign last := 3 then b2; }
				}
			}
		}`)
		sym := m.state(t, "Machine")
		run := stateRun(sym, "go")
		x, err := Explore(context.Background(), mustPolicy(t, "explore"), m.fresh, run)
		if err != nil || !x.Complete() || x.Runs != 4 {
			t.Fatalf("explore: %v, %v", x, err)
		}
		for _, o := range x.Outcomes {
			if kinds := choiceKinds(o.Witness); !reflect.DeepEqual(kinds, []ChoiceKind{ChoiceTransition, ChoiceRegionOrder}) {
				t.Fatalf("witness %s draws %v, want the transition then the region order", FormatChoices(o.Witness), kinds)
			}
			outcome, choices, err := replayed(t, m.fresh, run, o.Witness)
			if err != nil {
				t.Errorf("%s: replaying %s: %v", o.Outcome, FormatChoices(o.Witness), err)
				continue
			}
			if outcome.String() != o.Outcome.String() {
				t.Errorf("replaying %s reached %s, want %s", FormatChoices(o.Witness), outcome, o.Outcome)
			}
			if kinds := choiceKinds(choices); !reflect.DeepEqual(kinds, []ChoiceKind{ChoiceRegionOrder, ChoiceTransition}) {
				t.Errorf("replaying %s noted %v, want the region order then the transition", FormatChoices(o.Witness), kinds)
			}
			got, want := strings.Split(FormatChoices(choices), "; "), strings.Split(FormatChoices(o.Witness), "; ")
			slices.Sort(got)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("replaying %s made the choices\n%s", FormatChoices(o.Witness), FormatChoices(choices))
			}
		}
	})
	t.Run("due", func(t *testing.T) {
		fresh, run := dueOrderModel(t)
		x, err := Explore(context.Background(), mustPolicy(t, "explore"), fresh, run)
		if err != nil || !x.Complete() || x.Runs != 6 {
			t.Fatalf("explore: %v, %v", x, err)
		}
		assertWitnessesReplay(t, x, fresh, run)
	})
}

// dueOrderModel is three tickers waking at one instant, each taking the next mark
// of a shared cell, so the order they run in is the outcome.
func dueOrderModel(t *testing.T) (func() (*Context, error), func(*Context) (Outcome, error)) {
	t.Helper()
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
	return fresh, run
}

// stateRun initializes the machine, sends it signal and runs it to completion.
func stateRun(sym *symbols.Symbol, signal string) func(*Context) (Outcome, error) {
	return func(ctx *Context) (Outcome, error) {
		exec, err := ctx.CreateStateExecutor(sym)
		if err != nil {
			return Outcome{}, err
		}
		exec.SendSignal(signal, nil)
		if err := exec.RunToCompletion(); err != nil {
			return Outcome{}, err
		}
		return exec.Outcome(), nil
	}
}

// A witness move the run cannot make is refused with a typed error naming the
// move: a token not able to act, a branch not holding, a move at a step the run
// is past, a move where the run has none, and one left over when the run ends.
func TestReplayRefusesAMoveNotEnabled(t *testing.T) {
	m := parseExploreModel(t, choiceModel)
	sym := m.action(t, "route")
	run := func(ctx *Context) (Outcome, error) {
		outputs, err := ctx.ExecuteAction(sym)
		if err != nil {
			return Outcome{}, err
		}
		return ctx.ActionOutcome(outputs), nil
	}
	x := m.exploreAction(t, "explore", "route")
	good := x.Outcomes[0].Witness
	const wantGood = "step 3: 2@a first of 2@a, 3@b, 4@c; step 4: 3@b first of 3@b, 4@c; step 7: decision select -> 1->warn"
	if FormatChoices(good) != wantGood {
		t.Fatalf("witness %s, want %s", FormatChoices(good), wantGood)
	}
	orders := good[0].String() + "\n" + good[1].String() + "\n"
	cases := []struct {
		name  string
		lines string
		move  int
		faced string
	}{
		{"token not able", "step 3: 9@zzz first of 2@a, 9@zzz", 1, "9@zzz is not able to act (able to act: 2@a, 3@b, 4@c)"},
		{"alternative not able", "step 3: 2@a first of 2@a, 9@zzz", 1, "9@zzz is not able to act"},
		{"token order where one token acts", "step 1: 1@a first of 1@a, 2@b", 1, "is not able to act"},
		{"step already past", "step 1: decision select -> 1->warn", 1, "step 1 had no such move"},
		{"branch not holding", orders + "step 7: decision select -> 3->nowhere", 3, "3->nowhere is not enabled (enabled: 1->warn, 2->alarm)"},
		{"branch at the wrong place", orders + "step 7: decision elsewhere -> 1->warn", 3, "the run faced"},
		{"branch at the wrong step", orders + "step 6: decision select -> 1->warn", 3, "step 6 had no such move"},
		{"branch where a token order is faced", "step 3: decision select -> 1->warn", 1, "must pick a token (able to act: 2@a, 3@b, 4@c)"},
		{"move left over", wantGood + "; step 99: 1@a first of 1@a, 2@b", 4, "the run ended"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			witness, err := ParseChoices(strings.ReplaceAll(c.lines, "; ", "\n"))
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = replayed(t, m.fresh, run, witness)
			var refused *ReplayError
			if !errors.As(err, &refused) || !errors.Is(err, ErrReplayRefused) {
				t.Fatalf("error %T %v, want a ReplayError", err, err)
			}
			if refused.Move != c.move || refused.Choice.String() != witness[c.move-1].String() {
				t.Errorf("refused move %d (%s), want move %d (%s)", refused.Move, refused.Choice, c.move, witness[c.move-1])
			}
			if !strings.Contains(err.Error(), c.faced) || !strings.Contains(err.Error(), witness[c.move-1].String()) {
				t.Errorf("error %q does not say %q and name the move", err, c.faced)
			}
		})
	}
	if _, _, err := replayed(t, m.fresh, run, good); err != nil {
		t.Fatalf("the witness itself is refused: %v", err)
	}
}

// A run that outlives its witness goes on as `reverse` does: the witness's first
// move is taken, and from there the choices are the default policy's.
func TestReplayFallsBackToReverse(t *testing.T) {
	m := parseExploreModel(t, choiceModel)
	sym := m.action(t, "route")
	run := func(ctx *Context) (Outcome, error) {
		outputs, err := ctx.ExecuteAction(sym)
		if err != nil {
			return Outcome{}, err
		}
		return ctx.ActionOutcome(outputs), nil
	}
	x := m.exploreAction(t, "explore", "route")
	// The exploration's first run takes `1@a` first; reverse takes `3@c` first,
	// so a witness of that one move steers the run off the default path.
	first := x.Outcomes[0].Witness[:1]
	outcome, choices, err := replayed(t, m.fresh, run, first)
	if err != nil {
		t.Fatal(err)
	}
	if len(choices) < 2 || choices[0].String() != first[0].String() {
		t.Fatalf("choices %s, want %s then the rest", FormatChoices(choices), first[0])
	}
	ctx, _ := m.fresh()
	mustSchedule(t, ctx, DefaultSchedulePolicy)
	def, err := run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var defChoices []ChoiceTaken
	for _, c := range ctx.Choices() {
		defChoices = append(defChoices, c.Choice())
	}
	if defChoices[0].String() == first[0].String() {
		t.Fatalf("reverse takes %s too; the witness steers nothing", first[0])
	}
	// After the first token order the choices left (write orders, the decision)
	// are the ones reverse makes, being the default's.
	for i := 1; i < len(choices) && i < len(defChoices); i++ {
		if choices[i].Kind == ChoiceDecisionBranch && defChoices[i].Kind == ChoiceDecisionBranch && choices[i].Took != defChoices[i].Took {
			t.Errorf("past the witness the decision took %s, reverse takes %s", choices[i].Took, defChoices[i].Took)
		}
	}
	if outcome.String() == def.String() {
		t.Logf("witness %s and reverse reach one outcome %s", first[0], outcome)
	}
}

// forkDecisionModel decides while a sibling token is able to act, so one step
// holds both a token order and a decision.
const forkDecisionModel = `package test {
	action mix {
		attribute level : Integer = 75;
		attribute handler : Integer = 0;
		attribute x : Integer = 0;
		first start;
		fork split;
		action a { assign x := 1; }
		decide select;
		action warn { assign handler := 1; }
		action alarm { assign handler := 2; }
		merge either;
		join sync;
		done;
		succession first start then split;
		succession first split then a;
		succession first split then select;
		succession first select if level > 50 then warn;
		succession first select if level > 70 then alarm;
		succession first a then sync;
		succession first warn then either;
		succession first alarm then either;
		succession first either then sync;
		succession first sync then done;
	}
}`

// A run notes a step's decision before the token order that led to it; a witness
// written the other way round, order first as a checker states its moves,
// replays the same.
func TestReplayReadsAStepsOrderInEitherPlace(t *testing.T) {
	m := parseExploreModel(t, forkDecisionModel)
	sym := m.action(t, "mix")
	run := func(ctx *Context) (Outcome, error) {
		outputs, err := ctx.ExecuteAction(sym)
		if err != nil {
			return Outcome{}, err
		}
		return ctx.ActionOutcome(outputs), nil
	}
	const recorded = "step 3: decision select -> 2->alarm; step 3: 3@select first of 2@a, 3@select"
	const stated = "step 3: 3@select first of 2@a, 3@select; step 3: decision select -> 2->alarm"
	var traces []string
	for _, text := range []string{recorded, stated} {
		w, err := ParseChoices(text)
		if err != nil {
			t.Fatal(err)
		}
		outcome, choices, err := replayed(t, m.fresh, run, w)
		if err != nil {
			t.Fatalf("%s: %v", text, err)
		}
		if got := outcome.String(); got != "handler = 2; level = 75; x = 1" {
			t.Errorf("%s reached %s", text, got)
		}
		traces = append(traces, FormatChoices(choices))
	}
	if traces[0] != traces[1] || !strings.HasPrefix(traces[0], recorded) {
		t.Errorf("the two spellings made different choices:\n%s\n%s", traces[0], traces[1])
	}
}

// The policies that exist keep their behaviour: their traces are unchanged by the
// replay policy existing, and a probe under replay does not move the witness.
func TestReplayProbeLeavesTheWitnessInPlace(t *testing.T) {
	idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
		state Dispatcher {
			attribute level : Integer = 8;
			entry; then idle;
			state idle;
			state low;
			state high;
			transition first idle accept Go if level > 5 then low;
			transition first idle accept Go if level > 7 then high;
		}
	}`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Dispatcher", ast.DefState)
	if sym == nil {
		t.Fatal("state machine not found")
	}
	witness, err := ParseChoices("state idle on accept Go -> 2->high\n")
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, ReplayPolicy(witness))
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatal(err)
	}
	// Deciding previews the transition without making it, so the witness's move is
	// still there for the run.
	for i := 0; i < 2; i++ {
		decision, err := exec.Decide(Message{SignalType: "Go"})
		if err != nil || len(decision.Fires) != 1 || !strings.HasSuffix(decision.Fires[0], "-> high") {
			t.Fatalf("Decide %d: %+v, %v; want the witness's transition into high", i, decision, err)
		}
	}
	exec.SendSignal("Go", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatal(err)
	}
	if err := ctx.Unfollowed(); err != nil {
		t.Fatal(err)
	}
	if got := activeLeaf(exec); got != "high" {
		t.Fatalf("ended in %s, want high", got)
	}
}

// A behavior run whole by the context — an action or a state machine — ends
// refused when the witness has moves left over, with no call to Unfollowed needed.
func TestReplayRefusesMovesLeftOverByARun(t *testing.T) {
	t.Run("action", func(t *testing.T) {
		m := parseExploreModel(t, choiceModel)
		sym := m.action(t, "route")
		good := m.exploreAction(t, "explore", "route").Outcomes[0].Witness
		extra := ChoiceTaken{Kind: ChoiceTokenOrder, Step: 99, Among: []string{"1@a", "2@b"}, Took: "1@a", Alternatives: 2}
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		mustSchedule(t, ctx, ReplayPolicy(append(slices.Clone(good), extra)))
		// Before any run there is nothing unfollowed, and asking begins no run.
		if err := ctx.Unfollowed(); err != nil || ctx.run.scheduler != nil {
			t.Fatalf("before a run: Unfollowed() = %v, scheduler begun %v", err, ctx.run.scheduler != nil)
		}
		_, err = ctx.ExecuteAction(sym)
		assertRefusedLeftOver(t, err, len(good)+1, extra)
	})
	t.Run("state", func(t *testing.T) {
		idx, _, ctx := buildRuntime(t, "<test>", parseAndBuild(t, `package test {
			state Dispatcher {
				entry; then idle;
				state idle;
				state low;
				state high;
				transition first idle accept Go then low;
				transition first idle accept Go then high;
			}
		}`))
		sym := findSymbolByName(idx.DocumentRoot("<test>"), "Dispatcher", ast.DefState)
		if sym == nil {
			t.Fatal("state machine not found")
		}
		witness, err := ParseChoices("state idle on accept Go -> 2->high\nstate high on accept Go -> 1->idle\n")
		if err != nil {
			t.Fatal(err)
		}
		mustSchedule(t, ctx, ReplayPolicy(witness))
		_, _, err = ctx.ExecuteStateWithEvents(sym, []string{"Go"})
		assertRefusedLeftOver(t, err, 2, witness[1])
	})
}

// assertRefusedLeftOver checks that err is the refusal of the witness's move
// left over when the run ended.
func assertRefusedLeftOver(t *testing.T, err error, move int, choice ChoiceTaken) {
	t.Helper()
	var refused *ReplayError
	if !errors.As(err, &refused) || !errors.Is(err, ErrReplayRefused) {
		t.Fatalf("error %T %v, want a ReplayError", err, err)
	}
	if refused.Move != move || refused.Choice.String() != choice.String() || refused.Faced != "the run ended" {
		t.Errorf("refused %+v, want move %d (%s) faced the run ended", refused, move, choice)
	}
}

// A do-order move naming a state whose behavior is not due is refused before
// either due behavior acts, so the run stops where the witness stopped fitting.
func TestReplayRefusesADoOrderMoveNotEnabled(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		state def Interleave parallel {
			attribute seq : Integer = 0;
			state left {
				entry; then lstart;
				state lstart;
				state lwork { do { assign seq := seq * 10 + 1; assign seq := seq * 10 + 2; } }
				succession first lstart then lwork;
			}
			state right {
				entry; then rstart;
				state rstart;
				state rwork { do { assign seq := seq * 10 + 4; assign seq := seq * 10 + 5; } }
				succession first rstart then rwork;
			}
		}
	}`)
	sym := m.state(t, "Interleave")
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	witness, err := ParseChoices("do round at t=0.0: zork first of lwork, zork\n")
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, ReplayPolicy(witness))
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatal(err)
	}
	err = exec.RunToCompletion()
	var refused *ReplayError
	if !errors.As(err, &refused) || refused.Move != 1 || !strings.Contains(err.Error(), "zork is not enabled (enabled: lwork, rwork)") {
		t.Fatalf("error %T %v, want the do-order move refused", err, err)
	}
	if seq := FormatValue(exec.StateData()["seq"]); seq != "1" {
		t.Errorf("seq is %v after the refusal, want 1: neither due behavior may act on a refused round", seq)
	}
}

// A transition move refused on a message is refused before the message reaches
// the do behavior parked at an accept for it in a sibling region, so a refused
// replay leaves the machine's data as it found it.
func TestReplayRefusesATransitionMoveBeforeDoBehaviorsTakeTheMessage(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		attribute def Go;
		state def Waiter parallel {
			attribute total : Integer = 0;
			state left {
				entry; then lwork;
				state lwork {
					do action work {
						first start;
						then action reader accept Go;
						then action count assign total := total + 10;
						then done;
					}
				}
			}
			state right {
				entry; then rwait;
				state rwait;
				transition first rwait accept Go then rdone;
				transition first rwait accept Go then rother;
				state rdone { entry assign total := total + 1; }
				state rother { entry assign total := total + 2; }
			}
		}
	}`)
	sym := m.state(t, "Waiter")
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	witness, err := ParseChoices("state rwait on accept Go -> 3->nowhere\n")
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, ReplayPolicy(witness))
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatal(err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to the accept: %v", err)
	}
	exec.SendSignal("Go", nil)
	err = exec.RunToCompletion()
	var refused *ReplayError
	if !errors.As(err, &refused) || !errors.Is(err, ErrReplayRefused) || refused.Move != 1 || !strings.Contains(err.Error(), "3->nowhere") {
		t.Fatalf("error %T %v, want the transition move refused", err, err)
	}
	if total := FormatValue(exec.StateData()["total"]); total != "0" {
		t.Errorf("total is %v after the refusal, want 0: the do behavior may not take a message whose dispatch is refused", total)
	}
}

// A refused move at a choice changes nothing: the compound transition is undone
// whole — the exit made ahead of the choice, the incoming effect its guards were
// read against, the do behavior the exit abandoned — no branch is entered and no
// choice recorded, and the refusal is the run's.
func TestReplayRefusedChoiceMoveChangesNothing(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		attribute def Go;
		attribute def Tick;
		state def Machine {
			attribute level : Integer = 0;
			attribute exited : Integer = 0;
			attribute went : Integer = 0;
			entry; then idle;
			state idle {
				exit assign exited := 1;
				do action work {
					first start;
					then action reader accept Tick;
					then action count assign went := 100;
					then done;
				}
			}
			choice pick;
			state one { entry assign went := 1; }
			state two { entry assign went := 2; }
			state three { entry assign went := 3; }
			transition first idle accept Go do assign level := 8 then pick;
			transition first pick if level > 5 then one;
			transition first pick if level > 7 then two;
			transition first pick if level > 9 then three;
		}
	}`)
	sym := m.state(t, "Machine")
	witness, err := ParseChoices("choice pick -> 3->three\n")
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, ReplayPolicy(witness))
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		t.Fatal(err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to the do behavior's accept: %v", err)
	}
	if len(exec.doActions) != 1 || exec.doActions[0].run == nil {
		t.Fatalf("do actions %v, want idle's do behavior paused at its accept", exec.doActions)
	}
	paused := exec.doActions[0].run
	exec.SendSignal("Go", nil)
	err = exec.RunToCompletion()
	var refused *ReplayError
	if !errors.As(err, &refused) || !errors.Is(err, ErrReplayRefused) || refused.Move != 1 || !strings.Contains(err.Error(), "3->three is not enabled (enabled: 1->one, 2->two)") {
		t.Fatalf("error %T %v, want the choice move refused as not enabled", err, err)
	}
	data := exec.StateData()
	for name, want := range map[string]string{"level": "0", "exited": "0", "went": "0"} {
		if got := FormatValue(data[name]); got != want {
			t.Errorf("%s is %s after the refusal, want %s: the move is undone whole", name, got, want)
		}
	}
	if state, ok := exec.CurrentState().(*ast.StateNode); !ok || state.Name != "idle" {
		t.Errorf("the machine is in %v after the refusal, want idle", exec.CurrentState())
	}
	if len(exec.doActions) != 1 || exec.doActions[0].run != paused {
		t.Errorf("do actions %v after the refusal, want idle's do behavior paused as it was", exec.doActions)
	}
	if choices := ctx.Choices(); len(choices) != 0 {
		t.Errorf("the run recorded %v, want no choice: a refused move is not one made", choices)
	}
	if ctx.Unfollowed() == nil {
		t.Error("the refusal is not reported for the run")
	}
}

// A choice move naming a branch the choice's guards do not enable is refused
// where the choice is resolved, so the run neither takes a branch nor exits 0.
func TestReplayRefusesAChoiceBranchNotEnabled(t *testing.T) {
	m := parseExploreModel(t, `package test {
		private import ScalarValues::*;
		attribute def Go;
		state def Machine {
			attribute level : Integer = 0;
			entry; then idle;
			state idle;
			choice pick;
			state left;
			state right;
			state never;
			transition first idle accept Go do assign level := 8 then pick;
			transition first pick if level > 5 then left;
			transition first pick if level > 7 then right;
			transition first pick if level < 0 then never;
		}
	}`)
	sym := m.state(t, "Machine")
	for _, tc := range []struct{ witness, refused string }{
		{"choice pick -> 3->never\n", "3->never is not enabled (enabled: 1->left, 2->right)"},
		{"choice pick -> 9->nowhere\n", "9->nowhere is not enabled (enabled: 1->left, 2->right)"},
	} {
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		witness, err := ParseChoices(tc.witness)
		if err != nil {
			t.Fatal(err)
		}
		mustSchedule(t, ctx, ReplayPolicy(witness))
		exec, err := ctx.CreateStateExecutor(sym)
		if err != nil {
			t.Fatal(err)
		}
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("run to idle: %v", err)
		}
		exec.SendSignal("Go", nil)
		err = exec.RunToCompletion()
		var refused *ReplayError
		if !errors.As(err, &refused) || !errors.Is(err, ErrReplayRefused) || refused.Move != 1 || !strings.Contains(err.Error(), tc.refused) {
			t.Fatalf("%q: error %T %v, want the choice move refused with %q", tc.witness, err, err, tc.refused)
		}
		if visits := exec.GetStateVisits(); slices.Contains(visits, "left") || slices.Contains(visits, "right") || slices.Contains(visits, "never") {
			t.Errorf("%q: visits %v after the refusal, want no branch taken", tc.witness, visits)
		}
	}
}

// A refused decision move leaves the token at the decision: no branch is taken,
// so neither branch's action ran.
func TestReplayRefusedDecisionTakesNoBranch(t *testing.T) {
	m := parseExploreModel(t, choiceModel)
	sym := m.action(t, "route")
	good := m.exploreAction(t, "explore", "route").Outcomes[0].Witness
	witness, err := ParseChoices(good[0].String() + "\n" + good[1].String() + "\nstep 7: decision select -> 3->nowhere\n")
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, ReplayPolicy(witness))
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		t.Fatal(err)
	}
	for err == nil && exec.State() == StateRunning {
		err = exec.Step()
	}
	var refused *ReplayError
	if !errors.As(err, &refused) || refused.Move != 3 || !strings.Contains(refused.Faced, "3->nowhere is not enabled") {
		t.Fatalf("error %v, want move 3 refused as not enabled", err)
	}
	tokens := exec.Tokens()
	if len(tokens) != 1 {
		t.Fatalf("%d tokens after the refusal, want the one at the decision", len(tokens))
	}
	if decision, ok := tokens[0].Location.(*ast.DecisionNode); !ok || decision.Name != "select" {
		t.Errorf("token at %T, want decision select", tokens[0].Location)
	}
	if got := FormatTraceValue(exec.Data()["handler"]); got != "0" {
		t.Errorf("handler = %s, want 0: no branch ran", got)
	}
	if ctx.Unfollowed() == nil {
		t.Error("the refusal is not reported for the run")
	}
}

// An executor driven call by call — as the REPL drives it — refuses the witness
// moves left over when it completes, as a run under ExecuteAction does; a
// witness the run uses up is followed whole.
func TestReplayRefusesMovesLeftOverByADrivenExecutor(t *testing.T) {
	extra := ChoiceTaken{Kind: ChoiceTokenOrder, Step: 99, Among: []string{"1@a", "2@b"}, Took: "1@a", Alternatives: 2}
	t.Run("action run to completion", func(t *testing.T) {
		m := parseExploreModel(t, choiceModel)
		good := m.exploreAction(t, "explore", "route").Outcomes[0].Witness
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		mustSchedule(t, ctx, ReplayPolicy(append(slices.Clone(good), extra)))
		exec, err := ctx.CreateActionExecutor(m.action(t, "route"))
		if err != nil {
			t.Fatal(err)
		}
		assertRefusedLeftOver(t, exec.RunToCompletion(), len(good)+1, extra)
		if exec.State() != StateCompleted {
			t.Errorf("state %v, want completed: the run ended before the move was refused", exec.State())
		}
	})
	t.Run("action stepped", func(t *testing.T) {
		m := parseExploreModel(t, choiceModel)
		good := m.exploreAction(t, "explore", "route").Outcomes[0].Witness
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		mustSchedule(t, ctx, ReplayPolicy(append(slices.Clone(good), extra)))
		exec, err := ctx.CreateActionExecutor(m.action(t, "route"))
		if err != nil {
			t.Fatal(err)
		}
		steps := 0
		for err == nil && exec.State() == StateRunning {
			err = exec.Step()
			steps++
		}
		assertRefusedLeftOver(t, err, len(good)+1, extra)
		if exec.State() != StateCompleted || steps < 7 {
			t.Errorf("state %v after %d steps, want completed at the last step", exec.State(), steps)
		}
	})
	t.Run("action followed whole", func(t *testing.T) {
		m := parseExploreModel(t, choiceModel)
		good := m.exploreAction(t, "explore", "route").Outcomes[0].Witness
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		mustSchedule(t, ctx, ReplayPolicy(good))
		exec, err := ctx.CreateActionExecutor(m.action(t, "route"))
		if err != nil {
			t.Fatal(err)
		}
		if err := exec.RunToCompletion(); err != nil {
			t.Fatalf("run: %v", err)
		}
		if err := ctx.Unfollowed(); err != nil {
			t.Errorf("unfollowed: %v", err)
		}
	})
	const dispatcher = `package test {
		state def Dispatcher {
			entry; then idle;
			state idle;
			state low;
			transition first idle accept Go then low;
			transition first idle accept Go then done;
		}
	}`
	stateWitness := func(t *testing.T, lines string) []ChoiceTaken {
		t.Helper()
		witness, err := ParseChoices(lines)
		if err != nil {
			t.Fatal(err)
		}
		return witness
	}
	t.Run("state run to completion", func(t *testing.T) {
		m := parseExploreModel(t, dispatcher)
		witness := stateWitness(t, "state idle on accept Go -> 2->done\nstate low on accept Go -> 1->idle\n")
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		mustSchedule(t, ctx, ReplayPolicy(witness))
		exec, err := ctx.CreateStateExecutor(m.state(t, "Dispatcher"))
		if err != nil {
			t.Fatal(err)
		}
		exec.SendSignal("Go", nil)
		assertRefusedLeftOver(t, exec.RunToCompletion(), 2, witness[1])
		if exec.State() != StateCompleted {
			t.Errorf("state %v, want completed", exec.State())
		}
	})
	t.Run("state stepped", func(t *testing.T) {
		m := parseExploreModel(t, dispatcher)
		witness := stateWitness(t, "state idle on accept Go -> 2->done\nstate low on accept Go -> 1->idle\n")
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		mustSchedule(t, ctx, ReplayPolicy(witness))
		exec, err := ctx.CreateStateExecutor(m.state(t, "Dispatcher"))
		if err != nil {
			t.Fatal(err)
		}
		exec.SendSignal("Go", nil)
		assertRefusedLeftOver(t, exec.ProcessNextEvent(), 2, witness[1])
		if exec.State() != StateCompleted {
			t.Errorf("state %v, want completed", exec.State())
		}
	})
	t.Run("state not yet complete", func(t *testing.T) {
		m := parseExploreModel(t, dispatcher)
		witness := stateWitness(t, "state idle on accept Go -> 1->low\nstate low on accept Go -> 1->idle\n")
		ctx, err := m.fresh()
		if err != nil {
			t.Fatal(err)
		}
		mustSchedule(t, ctx, ReplayPolicy(witness))
		exec, err := ctx.CreateStateExecutor(m.state(t, "Dispatcher"))
		if err != nil {
			t.Fatal(err)
		}
		exec.SendSignal("Go", nil)
		if err := exec.ProcessNextEvent(); err != nil {
			t.Fatalf("a move left for a machine still running is not left over: %v", err)
		}
		if exec.State() == StateCompleted {
			t.Fatal("the machine completed in low")
		}
	})
}

// Every choice reads back from the line that spells it, whatever punctuation the
// names it carries share with the line; a witness of such lines reads back whole.
func TestChoiceLinesRoundTripPunctuatedNames(t *testing.T) {
	names := []string{"a, b", "x -> y", "p; q", "k: v", "it's", `back\slash`, "first of all", "step 3", " padded ", "tab\there", "line\nbreak", "plain"}
	var choices []ChoiceTaken
	for i, name := range names {
		others := []string{name, names[(i+1)%len(names)], names[(i+2)%len(names)]}
		choices = append(choices,
			ChoiceTaken{Kind: ChoiceTokenOrder, Step: i + 1, Alternatives: 3, Taken: 0, Among: others, Took: name},
			ChoiceTaken{Kind: ChoiceDecisionBranch, Step: i + 1, Where: "decision " + name, Took: "1->" + name},
			ChoiceTaken{Kind: ChoiceTransition, Where: "state " + name + " on accept " + name, Took: "2->" + name},
			ChoiceTaken{Kind: ChoiceRegionOrder, Where: "on accept " + name, Alternatives: 3, Taken: 1, Among: []string{others[1], name, others[2]}, Took: name},
			ChoiceTaken{Kind: ChoiceDueOrder, Where: "t=5.0", Alternatives: 3, Taken: 2, Among: []string{others[1], others[2], name}, Took: name},
		)
	}
	var lines []string
	for _, want := range choices {
		line := want.String()
		got, err := ParseChoice(line)
		if err != nil {
			t.Errorf("%s: %v", line, err)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s reads back as %+v, want %+v", line, got, want)
		}
		lines = append(lines, line)
	}
	for _, line := range lines {
		if strings.ContainsAny(line, "\n\r") {
			t.Errorf("%q spans lines", line)
		}
	}
	text := strings.Join(lines[:5], "; ") + "\n" + strings.Join(lines[5:], "\n") + "\n\nstep 1: tokens 1@'a, b'\nanything -> at all; after the blank line\n"
	got, err := ParseChoices(text)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, choices) {
		t.Errorf("witness reads back as\n%s\nwant\n%s", FormatChoices(got), FormatChoices(choices))
	}
	plain := ChoiceTaken{Kind: ChoiceTokenOrder, Step: 3, Alternatives: 2, Among: []string{"2@a", "3@b"}, Took: "2@a"}
	if got := plain.String(); got != "step 3: 2@a first of 2@a, 3@b" {
		t.Errorf("a plain name is quoted: %s", got)
	}
}

// A quoted name left open, or closed where the line's punctuation does not follow,
// is a parse error naming the quote.
func TestParseChoiceRejectsAnUnclosedQuote(t *testing.T) {
	for _, line := range []string{
		"step 3: 'a, b first of 'a, b', 3@b",
		"step 3: 'a, b'x first of 'a, b', 3@b",
		"'state idle -> 2->right",
		"on accept go: 'b1 first of a1, 'b1'",
	} {
		_, err := ParseChoice(line)
		var parse *ChoiceParseError
		if !errors.As(err, &parse) || parse.Reason != unclosedQuote {
			t.Errorf("%s: %v, want the unclosed quote refused", line, err)
		}
	}
}
