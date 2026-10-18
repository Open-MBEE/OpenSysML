package runtime

import (
	"testing"
)

// roundStepOrderModel is doStepModel with a signal nothing accepts, so a dispatch
// that would drop its occurrence can head the pool while the do step is due.
const roundStepOrderModel = `package test {
	private import ScalarValues::*;
	attribute def Stop;
	attribute def Noise;
	state def Machine {
		attribute log : String = "";
		entry; then top;
		state top {
			do { assign log := log + "did "; }
		}
		transition first top accept Stop do assign log := log + "stop " then idle;
		state idle;
	}
}`

// TestRuntimeRobustnessRoundStepOrder: a fixed policy's round records the step order it
// takes — the due do move ahead of an acting dispatch — as the choice point the
// unit-at-a-time policies draw, without moving the run; a dispatch that would drop its
// occurrence is no alternative and records nothing; the recorded choices replay the run.
func TestRuntimeRobustnessRoundStepOrder(t *testing.T) {
	t.Run("default_run_records_the_step_order_as_the_do_step", testDefaultRunRecordsTheStepOrderAsTheDoStep)
	t.Run("dropped_dispatch_at_the_head_records_no_step_order", testDroppedDispatchAtTheHeadRecordsNoStepOrder)
	t.Run("recorded_choices_replay_the_default_run", testRecordedChoicesReplayTheDefaultRun)
}

// runRoundStepOrder runs roundStepOrderModel under policy with signals in the pool and
// returns the context and the log the run left.
func runRoundStepOrder(t *testing.T, policy *SchedulePolicy, signals ...string) (*Context, string) {
	t.Helper()
	m := parseExploreModel(t, roundStepOrderModel)
	ctx, err := m.fresh()
	if err != nil {
		t.Fatal(err)
	}
	if policy != nil {
		mustSchedule(t, ctx, *policy)
	}
	exec, err := ctx.CreateStateExecutor(m.state(t, "Machine"))
	if err != nil {
		t.Fatal(err)
	}
	for _, signal := range signals {
		exec.SendSignal(signal, nil)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("RunToCompletion: %v", err)
	}
	return ctx, FormatValue(exec.StateData()["log"])
}

func stepOrderChoices(ctx *Context) []ChoicePoint {
	var out []ChoicePoint
	for _, choice := range ctx.Choices() {
		if choice.Kind == choiceStepOrder {
			out = append(out, choice)
		}
	}
	return out
}

// testDefaultRunRecordsTheStepOrderAsTheDoStep: with Stop at the head of the pool as top
// is entered, the default run performs the do step first and says so.
func testDefaultRunRecordsTheStepOrderAsTheDoStep(t *testing.T) {
	ctx, log := runRoundStepOrder(t, nil, "Stop")
	if log != `"did stop "` {
		t.Fatalf("log = %s, want the do step before the dispatch as before", log)
	}
	choices := stepOrderChoices(ctx)
	if len(choices) != 1 {
		t.Fatalf("step order choices = %v, want one", choices)
	}
	choice := choices[0]
	if choice.Taken != 0 || len(choice.Alternatives) != 2 ||
		choice.Alternatives[0] != "do top" || choice.Alternatives[1] != "dispatch accept Stop" {
		t.Errorf("choice = %+v, want do top taken first of do top, dispatch accept Stop", choice)
	}
	if choice.Where != "at t=0.0" {
		t.Errorf("choice.Where = %q, want at t=0.0", choice.Where)
	}
}

// testDroppedDispatchAtTheHeadRecordsNoStepOrder: Noise heads the pool and nothing accepts
// it, so the dispatch would drop its occurrence: it is not drawn against the do step.
func testDroppedDispatchAtTheHeadRecordsNoStepOrder(t *testing.T) {
	ctx, log := runRoundStepOrder(t, nil, "Noise")
	if log != `"did "` {
		t.Fatalf("log = %s, want the do step alone", log)
	}
	if choices := stepOrderChoices(ctx); len(choices) != 0 {
		t.Errorf("step order choices = %v, want none for a dispatch that drops its occurrence", choices)
	}
}

// testRecordedChoicesReplayTheDefaultRun: the choices the default run recorded are a
// witness `replay` follows to the same log, so the recorded order is the run's.
func testRecordedChoicesReplayTheDefaultRun(t *testing.T) {
	ctx, log := runRoundStepOrder(t, nil, "Stop")
	taken := ctx.choicesTaken()
	if len(taken) == 0 {
		t.Fatalf("the default run recorded no replayable choice")
	}
	replay := ReplayPolicy(taken)
	_, replayed := runRoundStepOrder(t, &replay, "Stop")
	if replayed != log {
		t.Errorf("replayed log = %s, want %s", replayed, log)
	}
}
