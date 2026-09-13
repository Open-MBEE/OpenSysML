package runtime

import (
	"errors"
	"testing"
)

const performModel = `package test {
	state def Machine {
		entry; then idle;
		state idle;
		state done;
		transition idle_done first idle accept go then done;
	}
}`

// A queued event is exactly one signal or one call; a bare payload with no
// signal to carry it is refused before it reaches the queue.
func TestEnqueueRejectsMalformedEvents(t *testing.T) {
	m := parseExploreModel(t, performModel)
	ctx, _ := m.fresh()
	exec, err := ctx.CreateStateExecutorFor(m.state(t, "Machine"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Release()
	payload := NewStringValue("x")
	for name, event := range map[string]QueuedEvent{
		"empty":               {},
		"signal and call":     {Signal: "go", Call: "op"},
		"value beside args":   {Signal: "go", Args: map[string]Value{"a": payload}, Value: &payload},
		"value beside call":   {Call: "op", Value: &payload},
		"value without event": {Value: &payload},
	} {
		if err := exec.Enqueue(event); !errors.Is(err, ErrMalformedEvent) {
			t.Errorf("%s: Enqueue = %v, want ErrMalformedEvent", name, err)
		}
	}
	if err := exec.Enqueue(QueuedEvent{Signal: "go", Value: &payload}); err != nil {
		t.Errorf("signal with payload: %v", err)
	}
}

// A performance that fails before it returns its executor withdraws that
// executor from the context's clock, so the context can be reused.
func TestPerformStateReleasesOnError(t *testing.T) {
	m := parseExploreModel(t, performModel)
	ctx, _ := m.fresh()
	sym := m.state(t, "Machine")
	if _, err := ctx.PerformState(sym, nil, []QueuedEvent{{}}); !errors.Is(err, ErrMalformedEvent) {
		t.Fatalf("PerformState = %v, want ErrMalformedEvent", err)
	}
	if n := len(ctx.clock.waiters); n != 0 {
		t.Fatalf("%d executors left on the clock after a failed performance, want none", n)
	}
	exec, err := ctx.PerformState(sym, nil, []QueuedEvent{{Signal: "go"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := exec.Outcome().FinalState; got != "done" {
		t.Fatalf("final state %q, want done", got)
	}
}
