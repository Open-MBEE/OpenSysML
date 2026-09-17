package runtime

import (
	"errors"
	"strings"
	"testing"
)

// inlinePerformanceModel performs `sub`, whose two branches race, inline in `outer`:
// the inner order is a step of the same number as the performer's token move.
const inlinePerformanceModel = `package test {
	action sub {
		attribute y : Integer = 0;
		first start;
		then fork split;
		succession first split then a;
		succession first split then b;
		action a { assign y := y + 1; }
		action b { assign y := y + 10; }
		join sync;
		succession first a then sync;
		succession first b then sync;
		then done;
	}
	action outer {
		attribute x : Integer = 0;
		first start;
		then action one assign x := 1;
		then perform action nested : sub;
		then done;
	}
}`

// TestRuntimeRobustnessReplay exercises the witness lines a replay cannot follow
// at a step several flows share: each is a typed refusal naming the move, never a
// move made against the wrong flow, a silent skip or a hang.
func TestRuntimeRobustnessReplay(t *testing.T) {
	t.Run("inner_order_naming_a_token_of_no_flow", testInnerOrderNamingATokenOfNoFlow)
	t.Run("inner_order_mixing_two_flows", testInnerOrderMixingTwoFlows)
	t.Run("witness_ending_before_the_run_does", testWitnessEndingBeforeTheRunDoes)
}

func refusedInlineReplay(t *testing.T, lines string) error {
	t.Helper()
	m := parseExploreModel(t, inlinePerformanceModel)
	sym := m.action(t, "outer")
	run := func(ctx *Context) (Outcome, error) {
		outputs, err := ctx.ExecuteAction(sym)
		if err != nil {
			return Outcome{}, err
		}
		return ctx.ActionOutcome(outputs), nil
	}
	witness, err := ParseChoices(lines)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = replayed(t, m.fresh, run, witness)
	var refused *ReplayError
	if !errors.As(err, &refused) || !errors.Is(err, ErrReplayRefused) {
		t.Fatalf("error %T %v, want a ReplayError", err, err)
	}
	if refused.Move != 1 || refused.Choice.String() != witness[0].String() {
		t.Errorf("refused move %d (%s), want move 1 (%s)", refused.Move, refused.Choice, witness[0])
	}
	return err
}

// testInnerOrderNamingATokenOfNoFlow: an order over a token no flow holds is
// refused at the inner step, where the run must pick, naming the token absent.
func testInnerOrderNamingATokenOfNoFlow(t *testing.T) {
	err := refusedInlineReplay(t, "step 3: 2@a first of 2@a, 9@zzz")
	if !strings.Contains(err.Error(), "step 3: 9@zzz is not able to act (able to act: 2@a, 3@b)") {
		t.Errorf("error %q does not name 9@zzz as absent at step 3", err)
	}
}

// testInnerOrderMixingTwoFlows: an order naming the performer's token beside an
// inner branch is no flow's step, and is refused where the branches race.
func testInnerOrderMixingTwoFlows(t *testing.T) {
	err := refusedInlineReplay(t, "step 3: 2@a first of 1@nested, 2@a")
	if !strings.Contains(err.Error(), "step 3: 1@nested is not able to act (able to act: 2@a, 3@b)") {
		t.Errorf("error %q does not name 1@nested as unable at step 3", err)
	}
}

// testWitnessEndingBeforeTheRunDoes: a witness cut short leaves the run to end on
// its own, one move a step, without a refusal or a move left over.
func testWitnessEndingBeforeTheRunDoes(t *testing.T) {
	m := loopingDoModel(t, false)
	sym := m.state(t, "Machine")
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	mustSchedule(t, ctx, ReplayPolicy(nil))
	run, err := beginInvocation(ctx, stateStarterOf(sym, HorizonAt(5)))
	if err != nil {
		t.Fatal(err)
	}
	defer run.inv.Release()
	for {
		if err := run.stabilize(); err != nil {
			t.Fatalf("run: %v", err)
		}
		if run.terminal() {
			break
		}
		if err := run.step(owners(run.enabledMoves())); err != nil {
			t.Fatalf("step: %v", err)
		}
	}
	if err := ctx.Unfollowed(); err != nil {
		t.Fatalf("unfollowed: %v", err)
	}
	outcome := run.inv.Outcome()
	for _, out := range outcome.RenderedOutputs() {
		if out.Text != "1" {
			t.Fatalf("outcome %s, want %s at 1", outcome, out.Name)
		}
	}
	if outcome.FinalState != "heard+finished" || len(outcome.RenderedOutputs()) != 3 {
		t.Fatalf("outcome %s, want heard+finished with left, right and late", outcome)
	}
}
