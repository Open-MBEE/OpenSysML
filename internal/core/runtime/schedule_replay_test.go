package runtime

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// clashSchedules is the two complete schedules of `clash`, keyed by the final x each leaves.
func clashSchedules(t *testing.T) (*exploreModel, Starter, map[string][]ChoiceTaken) {
	t.Helper()
	m := conformanceModel(t, "action_fork_branches_write_one_feature")
	report := checkModel(t, m, "clash", CheckBudget{}, unreduced())
	schedules := map[string][]ChoiceTaken{}
	for _, final := range report.Finals {
		schedules[final.Values["x"]] = final.Witness.Choices
	}
	if len(schedules["1"]) != 1 || len(schedules["2"]) != 1 {
		t.Fatalf("finals %v, want one schedule each ending x = 1 and x = 2", report.Finals)
	}
	return m, starterOf(m.action(t, "clash")), schedules
}

// xIsNot is the property that x is not the value given.
func xIsNot(value string) CheckProperty {
	return CheckProperty{Name: "x != " + value, Holds: func(ctx *Context, inv *Invocation) (bool, error) {
		r := &Replayed{Ctx: ctx, Inv: inv}
		x, err := r.FinalValue("x")
		return x != value, err
	}}
}

func scheduleDisagreement(t *testing.T, err error, want string) {
	t.Helper()
	var dis *ReplayDisagreement
	if !errors.As(err, &dis) || !errors.Is(err, ErrReplayDisagrees) {
		t.Fatalf("replay = %v, want a ReplayDisagreement about %q", err, want)
	}
	if !strings.Contains(dis.Reason, want) {
		t.Fatalf("disagreement %q, want it about %q", dis.Reason, want)
	}
}

// The state after the move named is the one evaluated: on the schedule that runs `right`
// first, x is 2 after its one move and 1 once `left` has repaired it at the end.
func TestReplayScheduleEvaluatesAtTheMoveNamed(t *testing.T) {
	m, start, schedules := clashSchedules(t)
	rightFirst := schedules["1"]
	r, err := ReplaySchedule(context.Background(), m.fresh, start, Witness{Choices: rightFirst}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if x, _ := r.FinalValue("x"); x != "2" || r.Inv.Completed() {
		t.Fatalf("after move 1 x = %s, complete %v; want 2 with the run going on", x, r.Inv.Completed())
	}
	if holds, err := r.Evaluate(xIsNot("2")); holds || err != nil {
		t.Fatalf("x != 2 at move 1: %v, %v; want false", holds, err)
	}
	end, err := ReplaySchedule(context.Background(), m.fresh, start, Witness{Choices: rightFirst}, ScheduleEnd)
	if err != nil {
		t.Fatal(err)
	}
	if x, _ := end.FinalValue("x"); x != "1" || !end.Inv.Completed() || end.Err != nil {
		t.Fatalf("at the end x = %s, complete %v, %v; want 1, complete", x, end.Inv.Completed(), end.Err)
	}
	if holds, err := end.Evaluate(xIsNot("2")); !holds || err != nil {
		t.Fatalf("x != 2 at the end: %v, %v; want true, the later move having repaired it", holds, err)
	}
	if !strings.Contains(end.Outcome(), "x = 1") {
		t.Fatalf("outcome %q, want x = 1", end.Outcome())
	}
	before, err := ReplaySchedule(context.Background(), m.fresh, start, Witness{Choices: rightFirst}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if x, _ := before.FinalValue("x"); x != "0" {
		t.Fatalf("before any move x = %s, want 0", x)
	}
}

// A whole execution is visited at every settled state: the schedule that runs `right` first
// passes through x = 2 after its one move, a property false there fails the visit, and the
// visit's own error ends the replay.
func TestReplayExecutionVisitsEverySettledState(t *testing.T) {
	m, start, schedules := clashSchedules(t)
	var seen []string
	r, err := ReplayExecution(context.Background(), m.fresh, start, Witness{Choices: schedules["1"]}, func(r *Replayed, moves int) error {
		x, err := r.FinalValue("x")
		if err != nil {
			return err
		}
		seen = append(seen, fmt.Sprintf("%d:%s", moves, x))
		return nil
	})
	if err != nil || r.Err != nil || !r.Inv.Completed() {
		t.Fatalf("replay: %v, %v, complete %v; want a complete run", err, r.Err, r.Inv.Completed())
	}
	if len(seen) < 3 || seen[0] != "0:0" || seen[len(seen)-1] != "1:1" || !slices.Contains(seen, "1:2") {
		t.Fatalf("visited %v, want x = 0 before any move, 2 after the move, 1 at the end", seen)
	}
	stopped := errors.New("x = 2 seen")
	_, err = ReplayExecution(context.Background(), m.fresh, start, Witness{Choices: schedules["1"]}, func(r *Replayed, moves int) error {
		if holds, err := r.Evaluate(xIsNot("2")); err != nil || !holds {
			return stopped
		}
		return nil
	})
	if !errors.Is(err, stopped) {
		t.Fatalf("replay = %v, want the visit's error", err)
	}
	_, err = ReplayExecution(context.Background(), m.fresh, start, Witness{Choices: schedules["1"]}, func(r *Replayed, moves int) error {
		holds, err := r.Evaluate(xIsNot("1"))
		if err != nil {
			return err
		}
		if !holds && moves == 0 {
			return errors.New("x = 1 before any move")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("replay = %v, want x != 1 to hold before the last move only", err)
	}
}

// A move the schedule does not have is a disagreement before any run.
func TestReplayScheduleRefusesAMoveBeyondTheSchedule(t *testing.T) {
	m, start, schedules := clashSchedules(t)
	_, err := ReplaySchedule(context.Background(), m.fresh, start, Witness{Choices: schedules["1"]}, 2)
	scheduleDisagreement(t, err, "move 2 of a schedule of 1 moves")
}

// A choice the run cannot follow is the replay's refusal, reported as a disagreement.
func TestReplayScheduleReportsARefusedChoice(t *testing.T) {
	m, start, schedules := clashSchedules(t)
	tampered := append([]ChoiceTaken(nil), schedules["1"]...)
	tampered[0].Took = "9@nowhere"
	tampered[0].Among = []string{"9@nowhere", tampered[0].Among[1]}
	_, err := ReplaySchedule(context.Background(), m.fresh, start, Witness{Choices: tampered}, ScheduleEnd)
	scheduleDisagreement(t, err, ErrReplayRefused.Error())
}

// A schedule with choices the run never reaches disagrees, to the end or to the move left over.
func TestReplayScheduleReportsChoicesLeftOver(t *testing.T) {
	m, start, schedules := clashSchedules(t)
	long := append(append([]ChoiceTaken(nil), schedules["1"]...), schedules["2"]...)
	for _, at := range []int{ScheduleEnd, 2} {
		_, err := ReplaySchedule(context.Background(), m.fresh, start, Witness{Choices: long}, at)
		scheduleDisagreement(t, err, "move 2 (step 3: 2@left first of 2@left, 3@right): the run ended")
	}
}

// A caller that goes away takes the replay with it: the error is the caller's.
func TestReplayScheduleStopsWhenCancelled(t *testing.T) {
	m, start, schedules := clashSchedules(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ReplaySchedule(ctx, m.fresh, start, Witness{Choices: schedules["1"]}, ScheduleEnd)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("replay = %v, want the caller's cancellation", err)
	}
}

// A run deadlocking before the end is the Replayed's Err, no disagreement: the schedule
// was followed whole, and what it reaches is the caller's to judge.
func TestReplayScheduleReportsADeadlockAsTheRunsError(t *testing.T) {
	m := parseExploreModel(t, `package test {
		action starve {
			first start;
			action stranded;
			join sync;
			done;
			succession first start then sync;
			succession first stranded then sync;
			succession first sync then done;
		}
	}`)
	r, err := ReplaySchedule(context.Background(), m.fresh, starterOf(m.action(t, "starve")), Witness{}, ScheduleEnd)
	if err != nil {
		t.Fatalf("replay: %v, want the deadlock as the run's error", err)
	}
	if !errors.Is(r.Err, ErrActionDeadlock) {
		t.Fatalf("run ended with %v, want a deadlock", r.Err)
	}
}

// A witness's inputs are fixed before the replay's first move, so the state evaluated is
// the one those inputs lead to; an input the action lacks is the witness's disagreement,
// not a failure of the run for the caller to judge.
func TestReplayScheduleFixesTheWitnessInputs(t *testing.T) {
	m := parseExploreModel(t, inputModel)
	start := starterOf(m.action(t, "gate"))
	r, err := ReplaySchedule(context.Background(), m.fresh, start,
		Witness{Inputs: []InputTaken{InputOf("n", intOf(5)), {Feature: "mode", Written: "Mode::Fast"}}}, ScheduleEnd)
	if err != nil || r.Err != nil {
		t.Fatalf("replay: %v, %v", err, r.Err)
	}
	over, err := r.FinalValue("over")
	if err != nil || over != "true" {
		t.Fatalf("over = %q, %v; want true under n = 5", over, err)
	}
	_, err = ReplaySchedule(context.Background(), m.fresh, start,
		Witness{Inputs: []InputTaken{InputOf("nn", intOf(5))}}, ScheduleEnd)
	scheduleDisagreement(t, err, "declares no such feature")
}

// Timed waits are settled on the clock as a check settles them: the slow branch writes
// last, so a replay to the end leaves x = 1.
func TestReplayScheduleAdvancesTheClock(t *testing.T) {
	m := parseLibraryModel(t, `package test {
		private import SI::*;
		private import ScalarValues::*;
		action timers {
			attribute x : Integer = 0;
			first start;
			fork split;
			action slow accept after 2 [s];
			action slowWrite { assign x := 1; }
			action fast accept after 1 [s];
			action fastWrite { assign x := 2; }
			join sync;
			done;
			succession first start then split;
			succession first split then slow;
			succession first split then fast;
			succession first slow then slowWrite;
			succession first fast then fastWrite;
			succession first slowWrite then sync;
			succession first fastWrite then sync;
			succession first sync then done;
		}
	}`)
	report := checkModel(t, m, "timers", CheckBudget{}, reduced())
	if len(report.Finals) != 1 {
		t.Fatalf("finals %v, want one", report.Finals)
	}
	start := starterOf(m.action(t, "timers"))
	r, err := ReplaySchedule(context.Background(), m.fresh, start, Witness{Choices: report.Finals[0].Witness.Choices}, ScheduleEnd)
	if err != nil {
		t.Fatal(err)
	}
	if x, _ := r.FinalValue("x"); x != "1" || !r.Inv.Completed() || r.Err != nil {
		t.Fatalf("x = %s, complete %v, %v; want 1, complete", x, r.Inv.Completed(), r.Err)
	}
}
