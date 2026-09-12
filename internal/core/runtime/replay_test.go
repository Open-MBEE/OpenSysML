package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
// unreadable line or no file at all is a typed policy error; the policy spells
// its file back and hands out its witness.
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
	for _, spelling := range []string{"replay", "replay:", "replay:" + filepath.Join(dir, "missing.txt"), "replay:" + bad} {
		_, err := ParseSchedulePolicy(spelling)
		var typed *SchedulePolicyError
		if !errors.As(err, &typed) || !errors.Is(err, ErrInvalidSchedulePolicy) || typed.Spelling != spelling {
			t.Errorf("%q: error %T %v, want a SchedulePolicyError naming the spelling", spelling, err, err)
		}
	}
	if _, err := ParseSchedulePolicy("replay:" + bad); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Errorf("an unreadable witness line is not named: %v", err)
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
