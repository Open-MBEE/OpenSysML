package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// writeMessage frames obj as an LSP message on conn.
func writeMessage(conn net.Conn, obj any) error {
	body, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(conn, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}

// readMessage reads one framed LSP message from r, failing on unframed output.
func readMessage(t *testing.T, r *bufio.Reader) map[string]any {
	t.Helper()
	msg, err := readFramedMessage(r)
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

// readFramedMessage reads one framed LSP message from r; it is usable off the
// test goroutine, where a failing test cannot be reported directly.
func readFramedMessage(r *bufio.Reader) (map[string]any, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("read header: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("unframed server output: %q", line)
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			length, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, fmt.Errorf("bad Content-Length %q: %w", value, err)
			}
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("message without Content-Length header")
	}
	body := make([]byte, length)
	for read := 0; read < length; {
		n, err := r.Read(body[read:])
		if err != nil {
			return nil, fmt.Errorf("read body: %w", err)
		}
		read += n
	}
	var msg map[string]any
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal %q: %w", body, err)
	}
	return msg, nil
}

// A served session must handle $/cancelRequest: an unwrapped handler answers it
// as an unknown method, and protocol.CancelHandler rejects the numeric ids real
// clients send as malformed.
func TestRunHandlerChainHandlesCancelRequest(t *testing.T) {
	for _, id := range []any{int32(7), "req-7"} {
		handler := runHandler(NewServer(model.NewWorkspace()))
		req, err := jsonrpc2.NewNotification(protocol.MethodCancelRequest, protocol.CancelParams{ID: id})
		if err != nil {
			t.Fatalf("NewNotification: %v", err)
		}
		replied := make(chan error, 1)
		reply := func(ctx context.Context, result any, err error) error {
			replied <- err
			return nil
		}
		if err := handler(context.Background(), reply, req); err != nil {
			t.Fatalf("handler: %v", err)
		}
		select {
		case err := <-replied:
			if err != nil {
				t.Errorf("$/cancelRequest with id %v answered with %v, want it handled", id, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("$/cancelRequest with id %v was never answered", id)
		}
	}
}

// Regression: Run must start exactly one read loop. protocol.NewServer already
// starts one, so an extra conn.Go raced two readers over the same stream and
// corrupted framing after a few messages.
func TestRunServesEveryRequestOverOneStream(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })

	s := NewServer(model.NewWorkspace())
	done := make(chan error, 1)
	go func() { done <- s.Run(context.Background(), server) }()

	// Writes run concurrently with reads: net.Pipe is unbuffered, so the
	// server's diagnostics notifications must be drained as they are sent.
	const requests = 25
	writeErr := make(chan error, 1)
	go func() {
		messages := []any{
			map[string]any{
				"jsonrpc": "2.0", "id": 1, "method": "initialize",
				"params": map[string]any{"capabilities": map[string]any{}},
			},
			map[string]any{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}},
			map[string]any{
				"jsonrpc": "2.0", "method": "textDocument/didOpen",
				"params": map[string]any{"textDocument": map[string]any{
					"uri": "file:///tmp/run.sysml", "languageId": "sysml", "version": 1,
					"text": "package P {\n    part def Wheel { attribute pressure; }\n    part w : Wheel;\n}\n",
				}},
			},
		}
		for id := 2; id <= requests+1; id++ {
			messages = append(messages, map[string]any{
				"jsonrpc": "2.0", "id": id, "method": "textDocument/completion",
				"params": map[string]any{
					"textDocument": map[string]any{"uri": "file:///tmp/run.sysml"},
					"position":     map[string]any{"line": 2, "character": 18},
				},
			})
		}
		messages = append(messages, map[string]any{"jsonrpc": "2.0", "id": 100, "method": "shutdown"})
		for _, msg := range messages {
			if err := writeMessage(client, msg); err != nil {
				writeErr <- err
				return
			}
		}
		writeErr <- nil
	}()

	if err := client.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	r := bufio.NewReader(client)
	// Responses may arrive in any order: the handler chain is asynchronous.
	seen := map[int]bool{}
	for len(seen) < requests+2 {
		msg := readMessage(t, r)
		id, ok := msg["id"].(float64)
		if !ok {
			continue // a notification, e.g. publishDiagnostics
		}
		if seen[int(id)] {
			t.Fatalf("duplicate response for id %d", int(id))
		}
		seen[int(id)] = true
	}
	for id := 1; id <= requests+1; id++ {
		if !seen[id] {
			t.Errorf("no response for request id %d", id)
		}
	}
	if !seen[100] {
		t.Error("no response for the shutdown request")
	}
	if err := <-writeErr; err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = client.Close()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after the stream closed")
	}
}
