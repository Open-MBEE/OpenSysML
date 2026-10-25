package lsp

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// A view debugs the behavior it exposes from another document, performed by an
// object from a third; edits there move or end the session, duplicates are refused.
func TestDebugTargetDeclaredInAnotherDocument(t *testing.T) {
	ctx := context.Background()
	split := strings.Index(debugMachine, "package MachineViews")
	machines, views := debugMachine[:split], debugMachine[split:]
	dir := t.TempDir()
	open := func(t *testing.T, s *Server, name, src string) uri.URI {
		t.Helper()
		docURI := uri.File(filepath.Join(dir, name))
		if err := s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
			TextDocument: protocol.TextDocumentItem{URI: docURI, LanguageID: "sysml", Version: 1, Text: src},
		}); err != nil {
			t.Fatalf("DidOpen %s: %v", name, err)
		}
		return docURI
	}
	s, viewsURI, rec := debugServer(t, "views.sysml", views)
	machinesURI := open(t, s, "machines.sysml", machines)
	open(t, s, "robots.sysml", "package Robots { part def Rover { exhibit state ops : Machines::Ops; } }\n")
	r := render(t, s, viewsURI, "MachineViews::opsView")
	id := ids(t, r)

	snap := mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: viewsURI},
		View:         "MachineViews::opsView", Target: "Machines::Ops", Object: "Robots::Rover",
	})
	if snap.Target != "Machines::Ops" || snap.Object != "Robots::Rover" || snap.Root != r.Nodes[0].ID {
		t.Fatalf("start snapshot = %s", describe(snap))
	}
	wantState(t, snap, debugWaiting)
	wantStrings(t, "initial active states", snap.ActiveStates, []string{id["motion"], id["idle"], id["clock"], id["waiting"]})

	// An edit to the machine's document that leaves the machine as declared
	// keeps the session running; one rewriting the machine ends it.
	s.applyDidChange(ctx, uriToName(machinesURI), []rawContentChange{{Text: strings.Replace(machines, "attribute def Halt;", "attribute def Halt;\n\tattribute def Stop;", 1)}}, 2)
	for _, moved := range rec.debugChanged() {
		if moved.Session != snap.Session || moved.State == debugEnded {
			t.Fatalf("debugChanged after an unrelated edit to the machine's document = %s", describe(moved))
		}
		wantStrings(t, "active states after an unrelated edit", moved.ActiveStates, snap.ActiveStates)
	}
	s.applyDidChange(ctx, uriToName(machinesURI), []rawContentChange{{Text: strings.Replace(machines, "state elapsed;", "state elapsed;\n\t\tstate cooling;", 1)}}, 3)
	changed := rec.debugChanged()
	last := changed[len(changed)-1]
	if last.Session != snap.Session || last.State != debugEnded || !strings.Contains(last.Reason, "Machines::Ops was edited") {
		t.Fatalf("debugChanged after the machine was edited = %s", describe(last))
	}

	// Closing the machine's document ends a session on it too.
	snap = mustDebug(t, s, MethodDebugStart, &debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: viewsURI}, View: "MachineViews::opsView", Target: "Machines::Ops",
	})
	if err := s.DidClose(ctx, &protocol.DidCloseTextDocumentParams{TextDocument: protocol.TextDocumentIdentifier{URI: machinesURI}}); err != nil {
		t.Fatalf("DidClose: %v", err)
	}
	changed = rec.debugChanged()
	last = changed[len(changed)-1]
	if last.Session != snap.Session || last.State != debugEnded || !strings.Contains(last.Reason, "machines.sysml was closed") {
		t.Fatalf("debugChanged after closing the machine's document = %s", describe(last))
	}

	// The same name declared by two documents names neither.
	open(t, s, "machines.sysml", machines)
	spareURI := open(t, s, "spare.sysml", machines)
	_, err := s.DebugStart(&debugStartParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: viewsURI}, View: "MachineViews::opsView", Target: "Machines::Ops",
	})
	if !errors.Is(err, ErrDebugTarget) || !strings.Contains(err.Error(), machinesURI.Filename()+", "+spareURI.Filename()) {
		t.Fatalf("start of an ambiguous target: err = %v, want %v naming both documents", err, ErrDebugTarget)
	}
	var wire *jsonrpc2.Error
	if !errors.As(err, &wire) || wire.Code != jsonrpc2.InvalidParams {
		t.Errorf("err = %v, want an InvalidParams reply", err)
	}
}
