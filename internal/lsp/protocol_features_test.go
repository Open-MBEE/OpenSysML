package lsp

import (
	"bufio"
	"context"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// Regression: semanticTokens/full and codeAction were answered with JSON-RPC
// -32601 (method not found) by the embedded default server, so editors got no
// highlighting and no quick fixes. This drives a real framed session and asserts
// both are served, with the capabilities that let a client ask for them.
func TestRunServesSemanticTokensAndCodeAction(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })

	s := NewServer(model.NewWorkspace())
	done := make(chan error, 1)
	go func() { done <- s.Run(context.Background(), server) }()

	const src = "package P {\n    part def Wheel;\n    part w : Wheeel;\n}\n"
	const file = "file:///tmp/protocol_features.sysml"
	doc := map[string]any{"uri": file}
	requests := []map[string]any{
		{"jsonrpc": "2.0", "id": 1, "method": "initialize",
			"params": map[string]any{"capabilities": map[string]any{}}},
		{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}},
		{"jsonrpc": "2.0", "method": "textDocument/didOpen",
			"params": map[string]any{"textDocument": map[string]any{
				"uri": file, "languageId": "sysml", "version": 1, "text": src,
			}}},
		{"jsonrpc": "2.0", "id": 2, "method": "textDocument/semanticTokens/full",
			"params": map[string]any{"textDocument": doc}},
		{"jsonrpc": "2.0", "id": 3, "method": "textDocument/semanticTokens/range",
			"params": map[string]any{"textDocument": doc, "range": map[string]any{
				"start": map[string]any{"line": 1, "character": 0},
				"end":   map[string]any{"line": 2, "character": 0},
			}}},
		{"jsonrpc": "2.0", "id": 4, "method": "textDocument/codeAction",
			"params": map[string]any{"textDocument": doc, "range": map[string]any{
				"start": map[string]any{"line": 2, "character": 13},
				"end":   map[string]any{"line": 2, "character": 19},
			}, "context": map[string]any{"diagnostics": []any{}}}},
		{"jsonrpc": "2.0", "id": 5, "method": "shutdown"},
	}

	writeErr := make(chan error, 1)
	go func() {
		for _, req := range requests {
			if err := writeMessage(client, req); err != nil {
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
	responses := map[int]map[string]any{}
	for len(responses) < 5 {
		msg := readMessage(t, r)
		id, ok := msg["id"].(float64)
		if !ok {
			continue // a notification, e.g. publishDiagnostics
		}
		responses[int(id)] = msg
	}
	if err := <-writeErr; err != nil {
		t.Fatalf("write: %v", err)
	}

	for id, msg := range responses {
		if e, ok := msg["error"].(map[string]any); ok {
			t.Fatalf("request %d answered with error %v", id, e)
		}
	}

	caps, ok := responses[1]["result"].(map[string]any)["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("initialize result = %v", responses[1]["result"])
	}
	tokens, ok := caps["semanticTokensProvider"].(map[string]any)
	if !ok {
		t.Fatalf("no semanticTokensProvider in %v", caps)
	}
	if tokens["full"] != true || tokens["range"] != true {
		t.Errorf("semanticTokensProvider = %v, want full and range", tokens)
	}
	legend, ok := tokens["legend"].(map[string]any)
	if !ok {
		t.Fatalf("no legend in %v", tokens)
	}
	if types, ok := legend["tokenTypes"].([]any); !ok || len(types) == 0 {
		t.Errorf("legend tokenTypes = %v, want the classifier's types", legend["tokenTypes"])
	}
	if mods, ok := legend["tokenModifiers"].([]any); !ok || len(mods) == 0 {
		t.Errorf("legend tokenModifiers = %v, want the classifier's modifiers", legend["tokenModifiers"])
	}
	// Delta is not implemented, so the provider must not claim it.
	if _, claimed := tokens["delta"]; claimed {
		t.Errorf("semanticTokensProvider claims delta support: %v", tokens)
	}
	action, ok := caps["codeActionProvider"].(map[string]any)
	if !ok {
		t.Fatalf("no codeActionProvider in %v", caps)
	}
	if kinds := action["codeActionKinds"]; !reflect.DeepEqual(kinds, []any{"quickfix", "refactor.rewrite"}) {
		t.Errorf("codeActionKinds = %v, want [quickfix refactor.rewrite]", kinds)
	}

	for _, id := range []int{2, 3} {
		result, ok := responses[id]["result"].(map[string]any)
		if !ok {
			t.Fatalf("semantic tokens result for %d = %v", id, responses[id]["result"])
		}
		data, ok := result["data"].([]any)
		if !ok || len(data) == 0 || len(data)%5 != 0 {
			t.Errorf("semantic tokens data for %d = %v", id, result["data"])
		}
	}

	actions, ok := responses[4]["result"].([]any)
	if !ok || len(actions) == 0 {
		t.Fatalf("codeAction result = %v, want a quick fix for the unresolved name", responses[4]["result"])
	}
	fix, ok := actions[0].(map[string]any)
	if !ok {
		t.Fatalf("code action = %v", actions[0])
	}
	if fix["kind"] != "quickfix" || fix["title"] != "Change 'Wheeel' to 'Wheel'" {
		t.Errorf("code action = %v", fix)
	}
	changes, ok := fix["edit"].(map[string]any)["changes"].(map[string]any)
	if edits, _ := changes[file].([]any); !ok || len(edits) == 0 {
		t.Errorf("code action edit = %v, want text edits for %s", fix["edit"], file)
	}

	_ = client.Close()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after the stream closed")
	}
}
