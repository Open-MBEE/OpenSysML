package lsp

import (
	"context"
	"testing"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

const lspTypo = "package P { state def M { entry; then a; state a; state b; transition first a when Pnig then b; } }"

func TestInitializeReadsTheDisabledLints(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options any
		want    int
	}{
		{"flat key", map[string]any{"disabledLints": []any{passes.CodeUndeclaredSignal}}, 1},
		{"nested section", map[string]any{"sysml": map[string]any{"disabledLints": []any{passes.CodeUndeclaredSignal}}}, 1},
		{"dotted key", map[string]any{"sysml.disabledLints": []any{passes.CodeUndeclaredSignal}}, 1},
		{"unknown code", map[string]any{"disabledLints": []any{"bogus"}}, 0},
		{"not a list", map[string]any{"disabledLints": "undeclared-signal"}, 0},
		{"no options", nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := model.NewWorkspace()
			s := NewServer(ws)
			if _, err := s.Initialize(context.Background(), &protocol.InitializeParams{
				InitializationOptions: tc.options,
			}); err != nil {
				t.Fatal(err)
			}
			if len(ws.DisabledLints()) != tc.want {
				t.Fatalf("disabled lints = %v, want %d", ws.DisabledLints(), tc.want)
			}
		})
	}
}

// Disabling a lint mid-session republishes what is open without its findings,
// and strict mode leaves a lint a warning.
func TestDidChangeConfigurationRepublishesWithoutTheLint(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	fc := &fakeClient{}
	s.client = fc
	ws.Open("a.sysml", []byte(lspTypo), 1)

	if err := s.DidChangeConfiguration(context.Background(), &protocol.DidChangeConfigurationParams{
		Settings: map[string]any{"sysml": map[string]any{"strictConformance": true}},
	}); err != nil {
		t.Fatal(err)
	}
	if sev := firstSeverity(t, fc.all()); sev != protocol.DiagnosticSeverityWarning {
		t.Fatalf("strict lint severity = %v, want warning", sev)
	}

	if err := s.DidChangeConfiguration(context.Background(), &protocol.DidChangeConfigurationParams{
		Settings: map[string]any{"sysml": map[string]any{"disabledLints": []any{passes.CodeUndeclaredSignal}}},
	}); err != nil {
		t.Fatal(err)
	}
	published := fc.all()
	if last := published[len(published)-1]; len(last.Diagnostics) != 0 {
		t.Fatalf("published %v after disabling the lint", last.Diagnostics)
	}
}

// enabledLints is read in the same shapes as disabledLints, and keeps an
// opt-in lint in what is republished.
func TestDidChangeConfigurationEnablesAnOptInLint(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	if _, err := s.Initialize(context.Background(), &protocol.InitializeParams{
		InitializationOptions: map[string]any{"sysml.enabledLints": []any{"bogus"}},
	}); err != nil {
		t.Fatal(err)
	}
	if len(ws.EnabledLints()) != 0 {
		t.Fatalf("an unknown code was taken: %v", ws.EnabledLints())
	}
	fc := &fakeClient{}
	s.client = fc
	ws.Open("a.sysml", []byte("package P { private import ScalarValues::*; attribute x : Real = 0.1; }"), 1)
	if err := s.DidChangeConfiguration(context.Background(), &protocol.DidChangeConfigurationParams{
		Settings: map[string]any{"sysml": map[string]any{"enabledLints": []any{passes.CodeRoundedRealLiteral}}},
	}); err != nil {
		t.Fatal(err)
	}
	published := fc.all()
	if len(published) == 0 {
		t.Fatal("enabling a lint republished nothing")
	}
	last := published[len(published)-1]
	if len(last.Diagnostics) != 1 || last.Diagnostics[0].Code != passes.CodeRoundedRealLiteral {
		t.Fatalf("published %v after enabling the lint", last.Diagnostics)
	}
}
