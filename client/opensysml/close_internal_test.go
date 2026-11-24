package opensysml

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// heldCaller stands in for a service whose ServerInfo answers only once
// released, and records whether it was closed while that call was in it.
type heldCaller struct {
	caller
	entered, release chan struct{}
	inCall           atomic.Bool
	closedInCall     atomic.Bool
}

func (h *heldCaller) serverInfo(context.Context) (*pb.ServerInfoResponse, error) {
	h.inCall.Store(true)
	close(h.entered)
	<-h.release
	h.inCall.Store(false)
	return &pb.ServerInfoResponse{Version: "held"}, nil
}

func (h *heldCaller) close() error {
	if h.inCall.Load() {
		h.closedInCall.Store(true)
	}
	return nil
}

// Close does not release the implementation under a call that began before it:
// it waits for the call, and refuses the ones that begin meanwhile.
func TestCloseWaitsForACallInFlight(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		held := &heldCaller{entered: make(chan struct{}), release: make(chan struct{})}
		c := &client{caller: held}
		answered := make(chan error, 1)
		go func() {
			_, err := c.ServerInfo(context.Background())
			answered <- err
		}()
		<-held.entered
		closed := make(chan struct{})
		go func() {
			_ = c.Close()
			close(closed)
		}()
		synctest.Wait()
		select {
		case <-closed:
			t.Fatal("Close returned while a call was in flight")
		default:
		}
		if _, err := c.ServerInfo(context.Background()); !errors.Is(err, CodeUnavailable) {
			t.Fatalf("a call begun during Close answered %v, want CodeUnavailable", err)
		}
		close(held.release)
		if err := <-answered; err != nil {
			t.Fatalf("the call in flight answered %v", err)
		}
		<-closed
		if held.closedInCall.Load() {
			t.Fatal("the implementation was closed under a call in flight")
		}
	})
}
