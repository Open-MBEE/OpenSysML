package lsp

import (
	"bufio"
	"context"
	"net"
	"reflect"
	"testing"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

func TestInitializeAdvertisesCapabilities(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	res, err := s.Initialize(context.Background(), &protocol.InitializeParams{})
	if err != nil {
		t.Fatalf("Initialize error: %v", err)
	}
	sync, ok := res.Capabilities.TextDocumentSync.(*protocol.TextDocumentSyncOptions)
	if !ok {
		t.Fatalf("TextDocumentSync = %T, want *protocol.TextDocumentSyncOptions", res.Capabilities.TextDocumentSync)
	}
	if !sync.OpenClose {
		t.Error("OpenClose = false, want true")
	}
	if sync.Change != protocol.TextDocumentSyncKindIncremental {
		t.Errorf("Change = %v, want Incremental", sync.Change)
	}
	// A client only sends didSave when the server asks for it.
	if sync.Save == nil {
		t.Error("Save not advertised, so didSave never arrives")
	}
	if res.Capabilities.HoverProvider != true {
		t.Error("HoverProvider not advertised")
	}
	if res.Capabilities.DefinitionProvider != true {
		t.Error("DefinitionProvider not advertised")
	}
	if res.Capabilities.ReferencesProvider != true {
		t.Error("ReferencesProvider not advertised")
	}
	if res.Capabilities.DocumentSymbolProvider != true {
		t.Error("DocumentSymbolProvider not advertised")
	}
	if res.Capabilities.WorkspaceSymbolProvider != true {
		t.Error("WorkspaceSymbolProvider not advertised")
	}
	if res.Capabilities.DocumentFormattingProvider != true {
		t.Error("DocumentFormattingProvider not advertised")
	}
	if res.Capabilities.DocumentRangeFormattingProvider != true {
		t.Error("DocumentRangeFormattingProvider not advertised")
	}
	if cp := res.Capabilities.CompletionProvider; cp == nil {
		t.Error("CompletionProvider not advertised")
	} else if !reflect.DeepEqual(cp.TriggerCharacters, []string{":", "."}) {
		t.Errorf("TriggerCharacters = %v, want [: .]", cp.TriggerCharacters)
	}
}

func TestShutdownAndExit(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	if err := s.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown error: %v", err)
	}
	if err := s.Exit(context.Background()); err != nil {
		t.Errorf("Exit error: %v", err)
	}
	if code := s.ExitCode(); code != 0 {
		t.Errorf("ExitCode after shutdown then exit = %d, want 0", code)
	}
}

// The exit notification must release Run, whatever the client asked before it:
// the status it earns is 0 after a shutdown and 1 without one.
func TestExitEndsTheSessionWithTheStatusLSPRequires(t *testing.T) {
	for _, tc := range []struct {
		name     string
		shutdown bool
		want     int
	}{
		{name: "exit after shutdown", shutdown: true, want: 0},
		{name: "exit without shutdown", shutdown: false, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, server := net.Pipe()
			t.Cleanup(func() { _ = client.Close() })

			s := NewServer(model.NewWorkspace())
			done := make(chan error, 1)
			go func() { done <- s.Run(context.Background(), server) }()

			if err := client.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
				t.Fatalf("SetDeadline: %v", err)
			}
			r := bufio.NewReader(client)
			if tc.shutdown {
				if err := writeMessage(client, map[string]any{"jsonrpc": "2.0", "id": 1, "method": "shutdown"}); err != nil {
					t.Fatalf("write shutdown: %v", err)
				}
				if msg := readMessage(t, r); msg["error"] != nil {
					t.Fatalf("shutdown answered %v, want a result", msg)
				}
			}
			if err := writeMessage(client, map[string]any{"jsonrpc": "2.0", "method": "exit"}); err != nil {
				t.Fatalf("write exit: %v", err)
			}

			select {
			case err := <-done:
				if err != nil {
					t.Errorf("Run returned %v, want nil for a session the client exited", err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("Run did not return after the exit notification")
			}
			if code := s.ExitCode(); code != tc.want {
				t.Errorf("ExitCode = %d, want %d", code, tc.want)
			}
		})
	}
}

// A notification after shutdown carries no id to refuse, so it never reaches the
// handlers at all — before shutdown the same notification does.
func TestNotificationAfterShutdownIsDropped(t *testing.T) {
	didOpen, err := jsonrpc2.NewNotification(protocol.MethodTextDocumentDidOpen, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI: "file:///tmp/dropped.sysml", LanguageID: "sysml", Version: 1, Text: "package P;\n",
		},
	})
	if err != nil {
		t.Fatalf("NewNotification: %v", err)
	}
	shutdown, err := jsonrpc2.NewCall(jsonrpc2.NewNumberID(1), protocol.MethodShutdown, nil)
	if err != nil {
		t.Fatalf("NewCall: %v", err)
	}
	reply := func(ctx context.Context, result any, err error) error { return nil }

	for _, shutDown := range []bool{false, true} {
		served := false
		s := NewServer(model.NewWorkspace())
		handler := s.lifecycleHandler(func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
			served = true
			return reply(ctx, nil, nil)
		})
		if shutDown {
			if err := handler(context.Background(), reply, shutdown); err != nil {
				t.Fatalf("shutdown: %v", err)
			}
			served = false
		}
		if err := handler(context.Background(), reply, didOpen); err != nil {
			t.Fatalf("didOpen: %v", err)
		}
		if served == shutDown {
			t.Errorf("didOpen reached the handlers = %v with shutdown = %v", served, shutDown)
		}
	}
}

// After a shutdown request the session owes every further request the
// InvalidRequest error, and every further notification silence.
func TestAfterShutdownOnlyExitIsServed(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })

	s := NewServer(model.NewWorkspace())
	done := make(chan error, 1)
	go func() { done <- s.Run(context.Background(), server) }()

	if err := client.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		t.Fatalf("SetDeadline: %v", err)
	}
	r := bufio.NewReader(client)
	// net.Pipe is unbuffered, so the writes run alongside the reads below.
	writeErr := make(chan error, 1)
	go func() {
		for _, msg := range []any{
			map[string]any{"jsonrpc": "2.0", "id": 1, "method": "shutdown"},
			map[string]any{"jsonrpc": "2.0", "id": 2, "method": "textDocument/documentSymbol",
				"params": map[string]any{"textDocument": map[string]any{"uri": "file:///tmp/after-shutdown.sysml"}}},
			map[string]any{"jsonrpc": "2.0", "id": 3, "method": "textDocument/hover",
				"params": map[string]any{"textDocument": map[string]any{"uri": "file:///tmp/after-shutdown.sysml"},
					"position": map[string]any{"line": 0, "character": 0}}},
		} {
			if err := writeMessage(client, msg); err != nil {
				writeErr <- err
				return
			}
		}
		writeErr <- nil
	}()

	// Replies may arrive in any order: the dispatch below the wrapper is
	// asynchronous, so each is judged by its own id.
	answers := map[int]map[string]any{}
	for len(answers) < 3 {
		msg := readMessage(t, r)
		id, ok := msg["id"].(float64)
		if !ok {
			t.Fatalf("the server sent %v after shutdown, want only the replies owed", msg)
		}
		answers[int(id)] = msg
	}
	if answers[1]["error"] != nil {
		t.Fatalf("shutdown answered %v, want a result", answers[1])
	}
	for _, id := range []int{2, 3} {
		failure, ok := answers[id]["error"].(map[string]any)
		if !ok {
			t.Fatalf("request %d after shutdown answered %v, want an error", id, answers[id])
		}
		if code, _ := failure["code"].(float64); int(code) != int(jsonrpc2.InvalidRequest) {
			t.Errorf("request %d error code = %v, want %d (InvalidRequest)", id, failure["code"], jsonrpc2.InvalidRequest)
		}
	}

	if err := <-writeErr; err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := writeMessage(client, map[string]any{"jsonrpc": "2.0", "method": "exit"}); err != nil {
		t.Fatalf("write exit: %v", err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after the exit notification")
	}
}
