package runtime

import (
	"errors"
	goruntime "runtime"
	"testing"
)

// TestRuntimeRobustnessResumableInlineDoBody exercises an inline do body run one
// statement a round: what an exit leaves behind of a body paused mid-loop, and the
// budget a body that never ends runs into.
func TestRuntimeRobustnessResumableInlineDoBody(t *testing.T) {
	t.Run("exit_mid_loop_drops_the_pending_iterations", testDoBodyExitMidLoopDropsThePendingIterations)
	t.Run("exit_mid_iteration_drops_the_rest_of_the_iteration", testDoBodyExitMidIterationDropsTheRestOfTheIteration)
	t.Run("exit_on_a_clock_wait_after_a_loop_leaves_no_timer", testDoBodyExitOnAClockWaitAfterALoopLeavesNoTimer)
	t.Run("non_terminating_body_exceeds_the_step_limit", testDoBodyNonTerminatingExceedsTheStepLimit)
	t.Run("non_terminating_flow_body_exceeds_the_step_limit", testDoBodyNonTerminatingFlowExceedsTheStepLimit)
}

// doBodyMachine is a machine whose `active` state runs body as its do behavior
// until a Stop is dispatched to it.
func doBodyMachine(body string) string {
	return `package test {
		private import ScalarValues::*;
		attribute def Stop;
		state Machine {
			attribute total : Integer = 0;
			attribute after : Integer = 0;
			entry; then active;
			state active {
				do action work { ` + body + ` }
				exit action leave { assign after := total; }
			}
			state stopped;
			transition first active accept Stop then stopped;
		}
	}`
}

// assertDoBodyAbandoned checks the run of the do body exited with the state is
// ended as abandoned and nothing of it is left due, waiting, or running.
func assertDoBodyAbandoned(t *testing.T, exec *StateExecutor, run *doRun, goroutines int) {
	t.Helper()
	if !run.body.ended || !errors.Is(run.body.err, ErrActionDeadlock) {
		t.Errorf("the paused body ended = %v with %v; want it abandoned with ErrActionDeadlock", run.body.ended, run.body.err)
	}
	if len(run.body.cursor) != 0 || len(run.body.resuming) != 0 {
		t.Errorf("frames %d kept and %d resuming after the abandonment; want none", len(run.body.cursor), len(run.body.resuming))
	}
	if exec.HasPendingDoWork() || len(exec.doActions) != 0 {
		t.Errorf("%d do behaviors, pending work %v after the exit; want none", len(exec.doActions), exec.HasPendingDoWork())
	}
	if waits := exec.ctx.Clock().Waits(); len(waits) != 0 {
		t.Errorf("%d wait(s) left on the clock; want none", len(waits))
	}
	if got := goruntime.NumGoroutine(); got > goroutines {
		t.Errorf("%d goroutines after the run, %d before; want none left behind", got, goroutines)
	}
	if leaf := activeLeaf(exec); leaf != "stopped" {
		t.Errorf("active state %s; want stopped", leaf)
	}
}

// begunDoRun runs the first do round and returns the one do behavior it began.
func begunDoRun(t *testing.T, exec *StateExecutor) *doRun {
	t.Helper()
	if ran, err := exec.RunDoRound(); err != nil || ran != 1 {
		t.Fatalf("first do round ran %d with %v; want the one behavior begun", ran, err)
	}
	if len(exec.doActions) != 1 || exec.doActions[0].run == nil {
		t.Fatalf("%d do behaviors after the first round; want the one under way", len(exec.doActions))
	}
	return exec.doActions[0].run
}

// pausedDoRun is begunDoRun for a body paused between statements.
func pausedDoRun(t *testing.T, exec *StateExecutor) *doRun {
	t.Helper()
	run := begunDoRun(t, exec)
	if !run.body.paused.yielded {
		t.Fatalf("the body paused %+v; want yielded between statements", run.body.paused)
	}
	return run
}

// testDoBodyExitMidLoopDropsThePendingIterations: a `for` body of one statement
// yields after each iteration; the Stop, dispatched after the round that ran the
// third, drops the two left with the behavior, the exit behavior runs, and
// nothing of the loop stays behind.
func testDoBodyExitMidLoopDropsThePendingIterations(t *testing.T) {
	goroutines := goruntime.NumGoroutine()
	exec := stateExecutorForSource(t, "Machine", doBodyMachine(`
		for i in 1..5 {
			assign total := total + i;
		}
	`))
	run := pausedDoRun(t, exec)
	if _, err := exec.RunDoRound(); err != nil {
		t.Fatalf("second do round: %v", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(3)) {
		t.Fatalf("total = %v after two rounds; want 3, one iteration a round", total)
	}
	exec.SendSignal("Stop", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	data := exec.StateData()
	if !valueEqual(data["total"], integerValue(6)) || !valueEqual(data["after"], integerValue(6)) {
		t.Errorf("total = %v, after = %v; want 6 and 6: one iteration in the round before the Stop, none after, the exit behavior did run", data["total"], data["after"])
	}
	assertDoBodyAbandoned(t, exec, run, goroutines)
}

// testDoBodyExitMidIterationDropsTheRestOfTheIteration: an iteration of two
// statements is two rounds, so the Stop dispatched after the round that ran the
// second iteration's first statement leaves its second unrun with the iterations after.
func testDoBodyExitMidIterationDropsTheRestOfTheIteration(t *testing.T) {
	goroutines := goruntime.NumGoroutine()
	exec := stateExecutorForSource(t, "Machine", doBodyMachine(`
		for i in 1..3 {
			assign total := total + i;
			assign total := total * 10;
		}
	`))
	run := pausedDoRun(t, exec)
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(1)) {
		t.Fatalf("total = %v after one round; want 1, the first statement of the first iteration", total)
	}
	if _, err := exec.RunDoRound(); err != nil {
		t.Fatalf("second do round: %v", err)
	}
	if total := exec.StateData()["total"]; !valueEqual(total, integerValue(10)) {
		t.Fatalf("total = %v after two rounds; want 10, the first iteration's second statement", total)
	}
	exec.SendSignal("Stop", nil)
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run to completion: %v", err)
	}
	data := exec.StateData()
	if !valueEqual(data["total"], integerValue(12)) || !valueEqual(data["after"], integerValue(12)) {
		t.Errorf("total = %v, after = %v; want 12 and 12: the round before the Stop ran the second iteration's `+ i`, its `* 10` and the third iteration never ran", data["total"], data["after"])
	}
	assertDoBodyAbandoned(t, exec, run, goroutines)
}

// testDoBodyExitOnAClockWaitAfterALoopLeavesNoTimer: a body whose flow loops in
// one node, then waits on the clock at the next — the one round performs the node
// and parks the token at the wait — is exited by the Stop while the wait is armed;
// the wait goes with the behavior, so the clock holds nothing of the state left.
func testDoBodyExitOnAClockWaitAfterALoopLeavesNoTimer(t *testing.T) {
	goroutines := goruntime.NumGoroutine()
	exec := stateExecutorForSource(t, "Machine", doBodyMachine(`
		first start;
		then action sum { for i in 1..2 { assign total := total + i; } }
		then action pause accept after 10;
		then action reset assign total := 0;
		then done;
	`))
	run := begunDoRun(t, exec)
	if !run.body.paused.onWait {
		t.Fatalf("the body paused %+v; want a wait on the clock after the loop", run.body.paused)
	}
	if waits := exec.ctx.Clock().Waits(); len(waits) != 1 {
		t.Fatalf("%d wait(s) on the clock; want the body's one", len(waits))
	}
	if exec.HasPendingDoWork() {
		t.Fatal("the body is due while its wait on the clock goes on")
	}
	exec.SendSignal("Stop", nil)
	if err := exec.RunToQuiescence(); err != nil {
		t.Fatalf("run to quiescence: %v", err)
	}
	data := exec.StateData()
	if !valueEqual(data["total"], integerValue(3)) || !valueEqual(data["after"], integerValue(3)) {
		t.Errorf("total = %v, after = %v; want 3 and 3: the body ended at its wait", data["total"], data["after"])
	}
	assertDoBodyAbandoned(t, exec, run, goroutines)
}

// testDoBodyNonTerminatingExceedsTheStepLimit: a loop that never ends spends the
// budget one iteration a round, and the run ends with the typed error rather than
// hanging.
func testDoBodyNonTerminatingExceedsTheStepLimit(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", doBodyMachine(`
		loop {
			assign total := total + 1;
		}
	`))
	pausedDoRun(t, exec)
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("run to completion = %v; want ErrStepLimitExceeded", err)
	}
	if total := exec.StateData()["total"]; total.Kind != ValConst || total.Const.Int < 2 {
		t.Errorf("total = %v; want the iterations run one a round before the budget ran out", total)
	}
}

// testDoBodyNonTerminatingFlowExceedsTheStepLimit: a body stating a token flow
// that cycles through a merge never retires its token; one node a round, it
// ends with the typed error.
func testDoBodyNonTerminatingFlowExceedsTheStepLimit(t *testing.T) {
	exec := stateExecutorForSource(t, "Machine", doBodyMachine(`
		first start;
		then merge again;
		then action bump { assign total := total + 1; }
		then again;
	`))
	err := exec.RunToCompletion()
	if !errors.Is(err, ErrStepLimitExceeded) {
		t.Fatalf("run to completion = %v; want ErrStepLimitExceeded", err)
	}
}
