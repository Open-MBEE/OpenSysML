package main

import (
	"testing"
	"time"
)

// -keep-line-breaks is the server's answer to a client that cannot set the
// setting: Format Document then leaves a one-line body on its line.
func TestKeepLineBreaksFlagKeepsAOneLineBody(t *testing.T) {
	for _, tc := range []struct {
		name  string
		args  []string
		edits int
	}{
		{"default", nil, 1},
		{"-keep-line-breaks", []string{"-keep-line-breaks"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := startServer(t, tc.args...)
			s.initialize(1)
			s.notify("textDocument/didOpen", map[string]any{
				"textDocument": map[string]any{
					"uri":        "file:///p.sysml",
					"languageId": "sysml",
					"version":    1,
					"text":       "package P { part def A; }\n",
				},
			})
			s.request(2, "textDocument/formatting", map[string]any{
				"textDocument": map[string]any{"uri": "file:///p.sysml"},
				"options":      map[string]any{"tabSize": 4, "insertSpaces": true},
			})
			result, _ := s.response(2)["result"].([]any)
			if len(result) != tc.edits {
				t.Errorf("formatting answered %d edits, want %d: %v", len(result), tc.edits, result)
			}
			s.request(3, "shutdown", nil)
			s.response(3)
			s.notify("exit", nil)
			s.waitStatus(20 * time.Second)
		})
	}
}
