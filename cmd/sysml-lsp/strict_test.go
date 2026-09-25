package main

import (
	"testing"
	"time"
)

// lspExtensionModel uses OpenSysML notation no SysML v2 production admits.
const lspExtensionModel = "package M { attribute def Alarm; state def S { state off { defer Alarm; } } }"

// The severities an editor draws: 1 error, 2 warning.
const (
	severityError   = 1.0
	severityWarning = 2.0
)

// -strict is the server's answer to a client that cannot set the setting, so it
// must decide the severity of the first diagnostics the editor is pushed.
func TestStrictFlagServesStrictDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want float64
	}{
		{"default", nil, severityWarning},
		{"-strict", []string{"-strict"}, severityError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := startServer(t, tc.args...)
			s.initialize(1)
			s.notify("textDocument/didOpen", map[string]any{
				"textDocument": map[string]any{
					"uri":        "file:///m.sysml",
					"languageId": "sysml",
					"version":    1,
					"text":       lspExtensionModel,
				},
			})
			if got := firstPublishedSeverity(s); got != tc.want {
				t.Errorf("severity = %v, want %v", got, tc.want)
			}
			s.request(2, "shutdown", nil)
			s.response(2)
			s.notify("exit", nil)
			s.waitStatus(20 * time.Second)
		})
	}
}

// firstPublishedSeverity reads until the server publishes a diagnostic.
func firstPublishedSeverity(s *session) float64 {
	s.t.Helper()
	// The first diagnostic can wait on a cold standard-library parse.
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		msg := s.readBy(deadline)
		if msg["method"] != "textDocument/publishDiagnostics" {
			continue
		}
		params, ok := msg["params"].(map[string]any)
		if !ok {
			continue
		}
		diags, ok := params["diagnostics"].([]any)
		if !ok || len(diags) == 0 {
			continue
		}
		first, ok := diags[0].(map[string]any)
		if !ok {
			continue
		}
		severity, ok := first["severity"].(float64)
		if !ok {
			s.t.Fatalf("diagnostic without a severity: %v", first)
		}
		return severity
	}
	s.t.Fatalf("no diagnostics published\nstderr: %s", s.stderr.String())
	return 0
}
