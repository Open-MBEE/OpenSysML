package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
)

// orderedSession serves a fake handler through inOrder over a framed
// connection, recording when each method is entered and left.
type orderedSession struct {
	t      *testing.T
	client net.Conn
	r      *bufio.Reader
	gate   chan struct{}
	slowIn chan struct{}
	mu     sync.Mutex
	log    []string
}

func newOrderedSession(t *testing.T) *orderedSession {
	t.Helper()
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })
	if err := client.SetDeadline(time.Now().Add(wireTimeout)); err != nil {
		t.Fatalf("SetDeadline: %v", err)
	}
	o := &orderedSession{t: t, client: client, r: bufio.NewReader(client), gate: make(chan struct{}), slowIn: make(chan struct{})}
	fake := func(ctx context.Context, req *jsonrpc2.Request) (any, error) {
		o.record(req.Method() + " in")
		defer o.record(req.Method() + " out")
		if req.Method() == "slow" {
			close(o.slowIn)
			<-o.gate
		}
		return req.Method(), nil
	}
	conn := jsonrpc2.NewConn(jsonrpc2.NewStream(server), jsonrpc2.WithCodec(lspCodec{}))
	conn.Go(context.Background(), protocol.CancelHandler(inOrder(answerCancelled(fake))))
	t.Cleanup(func() { _ = server.Close() })
	return o
}

func (o *orderedSession) record(event string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.log = append(o.log, event)
}

func (o *orderedSession) events() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return slices.Clone(o.log)
}

func (o *orderedSession) send(id int, method string, params string) {
	o.t.Helper()
	msg := wireMessage{JSONRPC: "2.0", Method: method, Params: json.RawMessage(params)}
	if id != 0 {
		msg.ID = &id
	}
	if err := writeMessage(o.client, msg); err != nil {
		o.t.Fatalf("write %s: %v", method, err)
	}
}

// Handlers run one at a time in wire order: a request sent after a slow
// notification waits for it instead of overtaking it.
func TestInOrderRunsHandlersInWireOrder(t *testing.T) {
	for range 20 {
		o := newOrderedSession(t)
		o.send(0, "slow", `{}`)
		<-o.slowIn
		o.send(1, "fast", `{}`)
		close(o.gate)
		reply := readMessage(t, o.r)
		if reply["result"] != "fast" {
			t.Fatalf("fast answered %v", reply)
		}
		want := []string{"slow in", "slow out", "fast in", "fast out"}
		if got := o.events(); !slices.Equal(got, want) {
			t.Fatalf("handler events = %v, want %v", got, want)
		}
	}
}

// A request queued behind a slow handler can still be cancelled: the read
// loop is free, so the cancel is seen, and the request answers without running.
func TestInOrderAnswersCancelledWhileQueued(t *testing.T) {
	o := newOrderedSession(t)
	o.send(0, "slow", `{}`)
	<-o.slowIn
	o.send(1, "fast", `{}`)
	o.send(0, protocol.MethodCancelRequest, `{"id":1}`)
	reply := readMessage(t, o.r)
	failure, _ := reply["error"].(map[string]any)
	if code, _ := failure["code"].(float64); int64(code) != int64(protocol.ErrRequestCancelled.Code) {
		t.Fatalf("queued request answered %v, want request cancelled", reply)
	}
	if got := o.events(); !slices.Equal(got, []string{"slow in"}) {
		t.Fatalf("handler events before the gate opened = %v, want the slow handler alone", got)
	}
	close(o.gate)
	o.send(2, "after", `{}`)
	if reply := readMessage(t, o.r); reply["result"] != "after" {
		t.Fatalf("request after the gate answered %v", reply)
	}
	if got := o.events(); slices.ContainsFunc(got, func(e string) bool { return strings.HasPrefix(e, "fast") }) {
		t.Fatalf("the cancelled request ran: %v", got)
	}
}
