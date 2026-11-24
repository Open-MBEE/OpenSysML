package grpc

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// TestSessionTurnContextFollowsTheCall checks the context a session's tools are
// bound to ends with the call Under is running, and stays open outside one.
func TestSessionTurnContextFollowsTheCall(t *testing.T) {
	ss := &Session{turn: make(chan struct{}, 1)}
	tools := turnScoped{ss}

	ctx, cancel := context.WithCancel(context.Background())
	err := ss.Under(ctx, func() error {
		if tools.Err() != nil {
			t.Error("the tools' context had ended before the call's")
		}
		cancel()
		<-tools.Done()
		if !errors.Is(tools.Err(), context.Canceled) {
			t.Errorf("the tools' context after the call was cancelled = %v, want context.Canceled", tools.Err())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Under = %v", err)
	}
	if tools.Err() != nil {
		t.Errorf("the tools' context outside a call = %v, want it open", tools.Err())
	}
}

// TestSessionToolEndedWithItsCallIsTheCallsEnd checks a tool failing with the
// end of its call's context answers the call's status, not a model failure,
// while the same error outside an ended call stays the model's.
func TestSessionToolEndedWithItsCallIsTheCallsEnd(t *testing.T) {
	ss := &Session{turn: make(chan struct{}, 1)}
	tool := fmt.Errorf("tool solve: %w", context.DeadlineExceeded)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	live, release := context.WithCancel(context.Background())
	defer release()
	ss.call.Store(&live)
	var failure *SessionFailure
	if err := ss.runFailure(tool, "perform: %v"); !errors.As(err, &failure) {
		t.Errorf("a tool's own failure under a live call = %v, want a SessionFailure", err)
	}
	ss.call.Store(&ctx)
	if err := ss.runFailure(tool, "perform: %v"); !errors.Is(err, errSessionInterrupted) {
		t.Errorf("a tool ended with its call = %v, want the call's interruption", err)
	}
	ss.call.Store(nil)
}
