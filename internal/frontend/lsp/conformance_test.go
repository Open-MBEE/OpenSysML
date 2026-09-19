package lsp

import (
	"context"
	"testing"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// lspExtension uses notation of ours: a warning in the editor by default, an
// error once the editor asks strictly.
const lspExtension = "package P { attribute def Alarm; state def S { state a { defer Alarm; } } }"

func TestInitializeReadsTheStrictConformanceOption(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options any
		want    diag.ConformanceMode
	}{
		{"flat key", map[string]any{"strictConformance": true}, diag.ConformanceStrict},
		{"nested section", map[string]any{"sysml": map[string]any{"strictConformance": true}}, diag.ConformanceStrict},
		{"dotted key", map[string]any{"sysml.strictConformance": true}, diag.ConformanceStrict},
		{"explicit false", map[string]any{"strictConformance": false}, diag.ConformanceDefault},
		{"unrelated options", map[string]any{"other": true}, diag.ConformanceDefault},
		{"no options", nil, diag.ConformanceDefault},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := model.NewWorkspace()
			s := NewServer(ws)
			if _, err := s.Initialize(context.Background(), &protocol.InitializeParams{
				InitializationOptions: tc.options,
			}); err != nil {
				t.Fatal(err)
			}
			if got := ws.ConformanceMode(); got != tc.want {
				t.Fatalf("mode = %v, want %v", got, tc.want)
			}
		})
	}
}

// A malformed setting must not be read as a request to answer the other
// question.
func TestStrictConformanceIgnoresANonBooleanSetting(t *testing.T) {
	ws := model.NewWorkspace(model.WithConformanceMode(diag.ConformanceStrict))
	s := NewServer(ws)
	if _, err := s.Initialize(context.Background(), &protocol.InitializeParams{
		InitializationOptions: map[string]any{"strictConformance": "yes"},
	}); err != nil {
		t.Fatal(err)
	}
	if ws.ConformanceMode() != diag.ConformanceStrict {
		t.Fatalf("mode = %v, want the mode left alone", ws.ConformanceMode())
	}
}

// Changing the setting mid-session republishes what is open, so the editor is
// not left showing the other mode's verdict.
func TestDidChangeConfigurationRepublishesUnderTheNewMode(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	fc := &fakeClient{}
	s.client = fc

	ws.Open("a.sysml", []byte(lspExtension), 1)
	s.publishDiagnostics(context.Background(), "a.sysml")
	if sev := firstSeverity(t, fc.all()); sev != protocol.DiagnosticSeverityWarning {
		t.Fatalf("default severity = %v, want warning", sev)
	}

	if err := s.DidChangeConfiguration(context.Background(), &protocol.DidChangeConfigurationParams{
		Settings: map[string]any{"sysml": map[string]any{"strictConformance": true}},
	}); err != nil {
		t.Fatal(err)
	}
	if ws.ConformanceMode() != diag.ConformanceStrict {
		t.Fatalf("mode = %v, want strict", ws.ConformanceMode())
	}
	if sev := firstSeverity(t, fc.all()); sev != protocol.DiagnosticSeverityError {
		t.Fatalf("strict severity = %v, want error", sev)
	}
}

// A payload that says nothing about the mode publishes nothing new.
func TestDidChangeConfigurationIgnoresUnrelatedSettings(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	fc := &fakeClient{}
	s.client = fc
	ws.Open("a.sysml", []byte(lspExtension), 1)

	if err := s.DidChangeConfiguration(context.Background(), &protocol.DidChangeConfigurationParams{
		Settings: map[string]any{"editor": map[string]any{"tabSize": 4}},
	}); err != nil {
		t.Fatal(err)
	}
	if got := fc.all(); len(got) != 0 {
		t.Fatalf("published %d set(s), want none", len(got))
	}
}

// firstSeverity is the severity of the first diagnostic of the last publish.
func firstSeverity(t *testing.T, published []*protocol.PublishDiagnosticsParams) protocol.DiagnosticSeverity {
	t.Helper()
	if len(published) == 0 {
		t.Fatal("nothing published")
	}
	last := published[len(published)-1]
	if len(last.Diagnostics) == 0 {
		t.Fatal("published an empty diagnostic set")
	}
	return last.Diagnostics[0].Severity
}
