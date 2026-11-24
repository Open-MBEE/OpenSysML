package grpc

import (
	"context"
	"errors"
	"testing"
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
