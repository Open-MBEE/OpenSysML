package lsp

import (
	"bufio"
	"context"
	"net"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// Drives a real framed session through signatureHelp, codeLens, codeLens/resolve
// and inlayHint, which the embedded default server answered with method-not-found,
// and checks initialize advertises each.
func TestRunServesSignatureHelpCodeLensAndInlayHints(t *testing.T) {
	client, server := net.Pipe()
	t.Cleanup(func() { _ = client.Close() })

	s := NewServer(model.NewWorkspace())
	done := make(chan error, 1)
	go func() { done <- s.Run(context.Background(), server) }()

	const file = "file:///tmp/protocol_editor_facts.sysml"
	doc := map[string]any{"uri": file}
	callPos := positionAfter(t, editorFactsModel, "fell = Fall(20.0, 9.")
	lines := uint32(len(editorFactsModel))
	requests := []map[string]any{
		{"jsonrpc": "2.0", "id": 1, "method": "initialize",
			"params": map[string]any{"capabilities": map[string]any{}}},
		{"jsonrpc": "2.0", "method": "initialized", "params": map[string]any{}},
		{"jsonrpc": "2.0", "method": "textDocument/didOpen",
			"params": map[string]any{"textDocument": map[string]any{
				"uri": file, "languageId": "sysml", "version": 1, "text": editorFactsModel,
			}}},
		{"jsonrpc": "2.0", "id": 2, "method": "textDocument/signatureHelp",
			"params": map[string]any{"textDocument": doc, "position": map[string]any{
				"line": callPos.Line, "character": callPos.Character,
			}}},
		{"jsonrpc": "2.0", "id": 3, "method": "textDocument/codeLens",
			"params": map[string]any{"textDocument": doc}},
		{"jsonrpc": "2.0", "id": 4, "method": "textDocument/inlayHint",
			"params": map[string]any{"textDocument": doc, "range": map[string]any{
				"start": map[string]any{"line": 0, "character": 0},
				"end":   map[string]any{"line": lines, "character": 0},
			}}},
	}

	writeErr := make(chan error, 1)
	write := func(reqs []map[string]any) {
		for _, req := range reqs {
			if err := writeMessage(client, req); err != nil {
				writeErr <- err
				return
			}
		}
		writeErr <- nil
	}
	go write(requests)

	if err := client.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	r := bufio.NewReader(client)
	responses := map[int]map[string]any{}
	read := func(n int) {
		for len(responses) < n {
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
	}
	read(4)

	caps, ok := responses[1]["result"].(map[string]any)["capabilities"].(map[string]any)
	if !ok {
		t.Fatalf("initialize result = %v", responses[1]["result"])
	}
	help, ok := caps["signatureHelpProvider"].(map[string]any)
	if !ok || help["triggerCharacters"] == nil {
		t.Errorf("signatureHelpProvider = %v, want trigger characters", caps["signatureHelpProvider"])
	}
	lensCap, ok := caps["codeLensProvider"].(map[string]any)
	if !ok || lensCap["resolveProvider"] != true {
		t.Errorf("codeLensProvider = %v, want resolveProvider", caps["codeLensProvider"])
	}
	if _, ok := caps["inlayHintProvider"].(map[string]any); !ok {
		t.Errorf("inlayHintProvider = %v, want the capability", caps["inlayHintProvider"])
	}

	sig, ok := responses[2]["result"].(map[string]any)
	if !ok {
		t.Fatalf("signatureHelp result = %v", responses[2]["result"])
	}
	sigs, _ := sig["signatures"].([]any)
	if len(sigs) != 1 || sigs[0].(map[string]any)["label"] != "Fall(h : Real, [g : Real]) : Real" {
		t.Errorf("signatures = %v", sig["signatures"])
	}
	if sig["activeParameter"] != float64(1) {
		t.Errorf("activeParameter = %v, want 1", sig["activeParameter"])
	}

	lenses, ok := responses[3]["result"].([]any)
	if !ok || len(lenses) == 0 {
		t.Fatalf("codeLens result = %v", responses[3]["result"])
	}
	var fall map[string]any
	for _, l := range lenses {
		lens := l.(map[string]any)
		if data, ok := lens["data"].(map[string]any); ok && data["element"] == "Demo::Fall" {
			fall = lens
		}
	}
	if fall == nil {
		t.Fatalf("no unresolved lens on Demo::Fall in %v", lenses)
	}

	hints, ok := responses[4]["result"].([]any)
	if !ok || len(hints) == 0 {
		t.Fatalf("inlayHint result = %v", responses[4]["result"])
	}
	labels := map[string]bool{}
	for _, h := range hints {
		labels[h.(map[string]any)["label"].(string)] = true
	}
	if !labels[": Real"] || !labels["= 20.0"] {
		t.Errorf("inlay hint labels = %v, want a type and a value", labels)
	}

	go write([]map[string]any{
		{"jsonrpc": "2.0", "id": 5, "method": "codeLens/resolve", "params": fall},
		{"jsonrpc": "2.0", "id": 6, "method": "shutdown"},
	})
	read(6)
	resolved, ok := responses[5]["result"].(map[string]any)
	if !ok {
		t.Fatalf("codeLens/resolve result = %v", responses[5]["result"])
	}
	cmd, _ := resolved["command"].(map[string]any)
	if cmd == nil || cmd["title"] != "3 references" || cmd["command"] != "editor.action.showReferences" {
		t.Errorf("resolved command = %v", resolved["command"])
	}

	_ = client.Close()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after the stream closed")
	}
}
