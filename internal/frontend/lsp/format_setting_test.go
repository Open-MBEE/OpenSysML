package lsp

import (
	"context"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
	"go.lsp.dev/protocol"
)

func TestInitializeReadsTheKeepLineBreaksOption(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options any
		want    bool
	}{
		{"flat key", map[string]any{"keepLineBreaks": true}, true},
		{"nested section", map[string]any{"sysml": map[string]any{"keepLineBreaks": true}}, true},
		{"dotted key", map[string]any{"sysml.keepLineBreaks": true}, true},
		{"explicit false", map[string]any{"keepLineBreaks": false}, false},
		{"unrelated options", map[string]any{"other": true}, false},
		{"no options", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewServer(model.NewWorkspace())
			if _, err := s.Initialize(context.Background(), &protocol.InitializeParams{
				InitializationOptions: mustLSPAny(t, tc.options),
			}); err != nil {
				t.Fatal(err)
			}
			if got := s.keepLineBreaks; got != tc.want {
				t.Fatalf("keepLineBreaks = %v, want %v", got, tc.want)
			}
		})
	}
}

// The flag starts the server keeping line breaks; a client's explicit false
// then switches the layout back on, as the setting always wins once set.
func TestKeepLineBreaksFlagIsTheStartingValue(t *testing.T) {
	s := NewServer(model.NewWorkspace(), WithKeepLineBreaks(true))
	if _, err := s.Initialize(context.Background(), &protocol.InitializeParams{
		InitializationOptions: mustLSPAny(t, map[string]any{"other": true}),
	}); err != nil {
		t.Fatal(err)
	}
	if !s.keepLineBreaks {
		t.Fatal("initialize without the setting forgot the flag")
	}
	changeConfiguration(t, s, map[string]any{"sysml": map[string]any{"keepLineBreaks": false}})
	if s.keepLineBreaks {
		t.Fatal("an explicit false did not switch the layout back on")
	}
}

func TestKeepLineBreaksIgnoresANonBooleanSetting(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	if _, err := s.Initialize(context.Background(), &protocol.InitializeParams{
		InitializationOptions: mustLSPAny(t, map[string]any{"keepLineBreaks": "true"}),
	}); err != nil {
		t.Fatal(err)
	}
	if s.keepLineBreaks {
		t.Fatal("a string setting switched keepLineBreaks on")
	}
	changeConfiguration(t, s, map[string]any{"keepLineBreaks": true})
	changeConfiguration(t, s, map[string]any{"keepLineBreaks": 1})
	if !s.keepLineBreaks {
		t.Fatal("a numeric setting switched keepLineBreaks off")
	}
}

func TestDidChangeConfigurationTogglesKeepLineBreaks(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	for _, step := range []struct {
		settings map[string]any
		want     bool
	}{
		{map[string]any{"keepLineBreaks": true}, true},
		{map[string]any{"editor": map[string]any{"tabSize": 4}}, true},
		{map[string]any{"sysml.keepLineBreaks": false}, false},
		{map[string]any{"sysml": map[string]any{"keepLineBreaks": true}}, true},
	} {
		changeConfiguration(t, s, step.settings)
		if s.keepLineBreaks != step.want {
			t.Fatalf("after %v keepLineBreaks = %v, want %v", step.settings, s.keepLineBreaks, step.want)
		}
	}
}

// changeConfiguration pushes settings to the server as an editor does.
func changeConfiguration(t *testing.T, s *Server, settings map[string]any) {
	t.Helper()
	if err := s.DidChangeConfiguration(context.Background(), &protocol.DidChangeConfigurationParams{
		Settings: mustLSPAny(t, settings),
	}); err != nil {
		t.Fatal(err)
	}
}
