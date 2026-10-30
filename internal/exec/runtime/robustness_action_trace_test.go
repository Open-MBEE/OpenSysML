package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestRuntimeRobustnessActionTrace covers traced runs that cannot finish and events no accept takes.
func TestRuntimeRobustnessActionTrace(t *testing.T) {
	t.Run("deadlocked_run_keeps_its_records", testActionTraceDeadlockKeepsRecords)
	t.Run("event_no_accept_waits_for", testActionTraceEventNoAcceptWaitsFor)
	t.Run("event_after_completion", testActionTraceEventAfterCompletion)
}

const actionTraceRobustnessSource = `
	package test {
		private import ScalarValues::*;
		item def Go;
		item def Abort;
		part def Craft;
		part craft : Craft;
		action def Wait {
			attribute count : Integer = 0;
			first start;
			then action tick assign count := count + 1;
			then action hold accept go : Go;
			then done;
		}
	}`

func startTracedWait(t *testing.T) (*Context, *ActionExecutor, *TraceRecorder, *Instance, func(string) Message) {
	t.Helper()
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, actionTraceRobustnessSource))
	trace := NewTraceRecorder()
	ctx.SetTrace(trace)
	craft, err := ctx.Instantiate(oneSymbol(t, idx, "test::craft"))
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	exec, _, err := ctx.StartActionPerformedBy(oneSymbol(t, idx, "test::Wait"), craft, map[string]Value{})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	message := func(name string) Message {
		msg, err := ctx.SignalMessage(oneSymbol(t, idx, "test::"+name), nil, craft)
		if err != nil {
			t.Fatalf("message %s: %v", name, err)
		}
		return msg
	}
	return ctx, exec, trace, craft, message
}

// testActionTraceDeadlockKeepsRecords: a traced action nothing wakes reports the typed
// deadlock, and the recorder still holds every record made up to the wait.
func testActionTraceDeadlockKeepsRecords(t *testing.T) {
	_, exec, trace, _, _ := startTracedWait(t)
	err := runWithWatchdog(t, exec.RunToCompletion)
	if !errors.Is(err, ErrAcceptDeadlock) {
		t.Fatalf("expected ErrAcceptDeadlock, got: %v", err)
	}
	var texts []string
	for _, r := range trace.Records() {
		if r.Kind == TraceStep || r.Kind == TraceAssign || r.Kind == TraceWait || r.Kind == TraceComplete {
			texts = append(texts, r.Text())
		}
	}
	want := []string{"step start", "step start -> tick", "assign count = 1", "step tick -> hold", "wait Go"}
	if strings.Join(texts, "\n") != strings.Join(want, "\n") {
		t.Errorf("partial trace:\n%s\nwant:\n%s", strings.Join(texts, "\n"), strings.Join(want, "\n"))
	}
}

// testActionTraceEventNoAcceptWaitsFor: an event no parked accept takes is a typed error
// naming the event and the accepts that do wait, not a hang and not a silent drop.
func testActionTraceEventNoAcceptWaitsFor(t *testing.T) {
	ctx, exec, trace, _, message := startTracedWait(t)
	err := runWithWatchdog(t, func() error {
		return ctx.DeliverEvents(exec, []Message{message("Abort")})
	})
	if err == nil {
		t.Fatal("expected an error for an event no accept waits for")
	}
	for _, want := range []string{"Abort", "accept go"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected %q in the report, got: %v", want, err)
		}
	}
	for _, r := range trace.Records() {
		if r.Kind == TraceAccept {
			t.Errorf("an unaccepted event was recorded as accepted: %s", r.Text())
		}
	}
	if err := ctx.DeliverEvents(exec, []Message{message("Go")}); err != nil {
		t.Fatalf("the waiting accept still takes its event: %v", err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run after the event: %v", err)
	}
}

// testActionTraceEventAfterCompletion: an event left over once the action has completed is
// a typed error, since nothing of the run can ever take it.
func testActionTraceEventAfterCompletion(t *testing.T) {
	ctx, exec, trace, _, message := startTracedWait(t)
	if err := ctx.DeliverEvents(exec, []Message{message("Go")}); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if err := exec.RunToCompletion(); err != nil {
		t.Fatalf("run: %v", err)
	}
	completed := false
	for _, r := range trace.Records() {
		completed = completed || r.Kind == TraceComplete
	}
	if !completed {
		t.Fatal("the completed run recorded no completion")
	}
	err := runWithWatchdog(t, func() error {
		return ctx.DeliverEvents(exec, []Message{message("Go")})
	})
	if err == nil || !strings.Contains(err.Error(), "Go") {
		t.Fatalf("expected a typed error naming the leftover event, got: %v", err)
	}
}
