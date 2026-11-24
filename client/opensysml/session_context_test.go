package opensysml_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

const spinningSessionSource = `package Spin {
	private import ScalarValues::*;
	part def Top {
		attribute turns : Integer = 0;
		action spin {
			first start;
			merge m;
			action a;
			succession first start then m;
			succession first m then a;
			succession first a then m;
		}
	}
	part top : Top;
}`

// openSpinning opens a session over a model whose action never ends, under a
// step budget no test run reaches, and instantiates the part that performs it.
func openSpinning(t *testing.T) (*opensysml.Session, opensysml.InstanceID) {
	t.Helper()
	t.Setenv("OPENSYSML_MAX_STEPS", "1000000000000")
	t.Setenv("OPENSYSML_MAX_ACTION_STEPS", "1000000000000")
	client := newClient(t)
	session, err := opensysml.OpenSession(client, parse(t, client, spinningSessionSource))
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	top, err := session.Instantiate("Spin::top")
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	return session, top
}

// statusCode is the code of the status err is, or CodeOK for any other error
// or none.
func statusCode(err error) opensysml.Code {
	var status *opensysml.StatusError
	if errors.As(err, &status) {
		return status.Code
	}
	return opensysml.CodeOK
}

// TestSessionContextStopsARunUnderWay checks a performance that would not end
// stops when its context does, with the status the context ended with, and
// that the session answers the next call.
func TestSessionContextStopsARunUnderWay(t *testing.T) {
	session, top := openSpinning(t)

	deadline, release := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer release()
	if _, err := session.PerformContext(deadline, top, "Spin::Top::spin", nil); statusCode(err) != opensysml.CodeDeadlineExceeded {
		t.Fatalf("PerformContext past its deadline = %v, want CodeDeadlineExceeded", err)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	if _, err := session.PerformContext(cancelled, top, "Spin::Top::spin", nil); statusCode(err) != opensysml.CodeCanceled {
		t.Fatalf("PerformContext cancelled = %v, want CodeCanceled", err)
	}

	turns, err := session.FeatureContext(context.Background(), top, "turns")
	if err != nil || turns.Value != opensysml.Int(0) {
		t.Fatalf("Feature after the stopped runs = %#v, %v; want Int(0)", turns, err)
	}
}

// TestSessionContextGivesUpWaitingItsTurn checks a call queued behind a run
// under way answers when its own context ends, and leaves that run alone.
func TestSessionContextGivesUpWaitingItsTurn(t *testing.T) {
	session, top := openSpinning(t)

	running, stop := context.WithCancel(context.Background())
	performed := make(chan error, 1)
	go func() {
		_, err := session.PerformContext(running, top, "Spin::Top::spin", nil)
		performed <- err
	}()
	// The read below is refused at once whether or not the run has begun, so
	// wait until it has to see the queued case.
	waitFor(t, func() bool {
		short, release := context.WithTimeout(context.Background(), time.Millisecond)
		defer release()
		_, err := session.FeatureContext(short, top, "turns")
		return statusCode(err) == opensysml.CodeDeadlineExceeded
	})
	select {
	case err := <-performed:
		t.Fatalf("the run under way ended when a queued call gave up: %v", err)
	default:
	}
	stop()
	if err := <-performed; statusCode(err) != opensysml.CodeCanceled {
		t.Fatalf("PerformContext cancelled = %v, want CodeCanceled", err)
	}

	done, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := session.AdvanceContext(done, 0); statusCode(err) != opensysml.CodeCanceled {
		t.Fatalf("AdvanceContext under an ended context = %v, want CodeCanceled", err)
	}
}

func waitFor(t *testing.T, met func() bool) {
	t.Helper()
	for end := time.Now().Add(30 * time.Second); time.Now().Before(end); time.Sleep(5 * time.Millisecond) {
		if met() {
			return
		}
	}
	t.Fatal("the condition was never met")
}

// TestSessionCloseDoesNotHoldCallsBehindARun checks a call made while Close
// waits for a run under way is refused at once rather than held until the run ends.
func TestSessionCloseDoesNotHoldCallsBehindARun(t *testing.T) {
	session, top := openSpinning(t)

	running, stop := context.WithCancel(context.Background())
	defer stop()
	performed := make(chan error, 1)
	go func() {
		_, err := session.PerformContext(running, top, "Spin::Top::spin", nil)
		performed <- err
	}()
	waitFor(t, func() bool {
		short, release := context.WithTimeout(context.Background(), time.Millisecond)
		defer release()
		_, err := session.FeatureContext(short, top, "turns")
		return statusCode(err) == opensysml.CodeDeadlineExceeded
	})

	closed := make(chan struct{})
	go func() {
		defer close(closed)
		_ = session.Close()
	}()
	refused := make(chan struct{})
	go func() {
		defer close(refused)
		for {
			short, release := context.WithTimeout(context.Background(), time.Millisecond)
			_, err := session.FeatureContext(short, top, "turns")
			release()
			if statusCode(err) == opensysml.CodeUnavailable {
				return
			}
		}
	}()
	select {
	case <-refused:
	case <-time.After(30 * time.Second):
		t.Fatal("a call made while Close waited for a run was held behind it")
	}
	stop()
	<-closed
	if err := <-performed; statusCode(err) != opensysml.CodeCanceled {
		t.Fatalf("PerformContext cancelled = %v, want CodeCanceled", err)
	}
}
