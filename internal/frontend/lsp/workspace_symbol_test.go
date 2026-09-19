package lsp

import (
	"context"
	"testing"

	"go.lsp.dev/protocol"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

func TestWorkspaceSymbolFindsByQuery(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	ws.Open("a.sysml", []byte("package Alpha { namespace Nested; }"), 1)
	ws.Open("b.sysml", []byte("package Beta;"), 1)

	got, err := s.Symbols(context.Background(), &protocol.WorkspaceSymbolParams{Query: "est"})
	if err != nil {
		t.Fatalf("Symbols err = %v", err)
	}
	// "est" matches "Nested" (case-insensitive substring), not Alpha/Beta.
	if len(got) != 1 || got[0].Name != "Nested" {
		t.Fatalf("Symbols(est) = %v, want [Nested]", got)
	}
	if got[0].Kind != protocol.SymbolKindNamespace {
		t.Errorf("Kind = %v, want Namespace", got[0].Kind)
	}
	// Nested lives inside Alpha, in a.sysml.
	if got[0].ContainerName != "Alpha" {
		t.Errorf("ContainerName = %q, want %q", got[0].ContainerName, "Alpha")
	}
	if got[0].Location.URI != nameToURI("a.sysml") {
		t.Errorf("URI = %v, want %v", got[0].Location.URI, nameToURI("a.sysml"))
	}
}

func TestWorkspaceSymbolAggregatesAcrossDocuments(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	ws.Open("a.sysml", []byte("package Widget;"), 1)
	ws.Open("b.sysml", []byte("package Gadget;"), 1)

	// "get" matches both Widget and Gadget across the two documents.
	got, err := s.Symbols(context.Background(), &protocol.WorkspaceSymbolParams{Query: "get"})
	if err != nil {
		t.Fatalf("Symbols err = %v", err)
	}
	names := map[string]bool{}
	for _, si := range got {
		names[si.Name] = true
	}
	if len(got) != 2 || !names["Widget"] || !names["Gadget"] {
		t.Fatalf("Symbols(get) = %v, want [Widget Gadget]", got)
	}
}

func TestWorkspaceSymbolEmptyQueryReturnsAll(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	ws.Open("a.sysml", []byte("package Alpha { namespace Nested; }"), 1)

	got, err := s.Symbols(context.Background(), &protocol.WorkspaceSymbolParams{Query: ""})
	if err != nil {
		t.Fatalf("Symbols err = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Symbols('') = %d symbols, want 2 (Alpha, Nested)", len(got))
	}
}
