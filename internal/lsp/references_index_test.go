package lsp

import (
	"context"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
)

// A declaration in lib, a use of it through a wildcard import, and a same-named
// declaration elsewhere that lib may redirect the use to. Only lib is ever edited.
const (
	refIdxLib   = "package Lib { part def Wheel; }\n"
	refIdxUse   = "package Use { import Lib::*; part w : Wheel; }\n"
	refIdxOther = "package Other { part def Wheel; }\n"
)

func openRefIndexWorkspace(t *testing.T) (*model.Workspace, *Server, map[string]string) {
	t.Helper()
	ws := model.NewWorkspace()
	s := NewServer(ws)
	names := map[string]string{
		"lib":   uri.File("/tmp/refidx_lib.sysml").Filename(),
		"use":   uri.File("/tmp/refidx_use.sysml").Filename(),
		"other": uri.File("/tmp/refidx_other.sysml").Filename(),
	}
	ws.Open(names["lib"], []byte(refIdxLib), 1)
	ws.Open(names["use"], []byte(refIdxUse), 1)
	ws.Open(names["other"], []byte(refIdxOther), 1)
	return ws, s, names
}

// indexReferencesAt runs textDocument/references with the cursor on the first
// occurrence of needle in the document's current text, declaration excluded.
func indexReferencesAt(t *testing.T, ws *model.Workspace, s *Server, name, needle string) []protocol.Location {
	t.Helper()
	src := ws.Document(name).Content
	off := strings.Index(string(src), needle)
	if off < 0 {
		t.Fatalf("%q not in %s", needle, name)
	}
	locs, err := s.References(context.Background(), &protocol.ReferenceParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition(src, off),
		},
	})
	if err != nil {
		t.Fatalf("References err = %v", err)
	}
	return locs
}

// indexRenameAt runs textDocument/rename with the cursor on the first occurrence of
// needle and returns the edited documents' names.
func indexRenameAt(t *testing.T, ws *model.Workspace, s *Server, name, needle string) map[string]int {
	t.Helper()
	src := ws.Document(name).Content
	off := strings.Index(string(src), needle)
	if off < 0 {
		t.Fatalf("%q not in %s", needle, name)
	}
	edit, err := s.Rename(context.Background(), &protocol.RenameParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
			Position:     offsetToPosition(src, off),
		},
		NewName: "Renamed",
	})
	if err != nil {
		t.Fatalf("Rename err = %v", err)
	}
	out := map[string]int{}
	for u, edits := range edit.Changes {
		out[uriToName(u)] = len(edits)
	}
	return out
}

func wantLocations(t *testing.T, locs []protocol.Location, wantDocs ...string) {
	t.Helper()
	if len(locs) != len(wantDocs) {
		t.Fatalf("references = %d %v, want %d in %v", len(locs), locs, len(wantDocs), wantDocs)
	}
	for i, l := range locs {
		if got := uriToName(l.URI); got != wantDocs[i] {
			t.Errorf("reference %d in %s, want %s", i, got, wantDocs[i])
		}
	}
}

// Editing the declaring document redirects a use elsewhere to a different
// declaration; both declarations' answers change without the use being touched.
func TestReferencesFollowCrossDocumentRedirect(t *testing.T) {
	ws, s, n := openRefIndexWorkspace(t)

	wantLocations(t, indexReferencesAt(t, ws, s, n["lib"], "Wheel"), n["use"])
	wantLocations(t, indexReferencesAt(t, ws, s, n["other"], "Wheel"))

	// Lib now re-exports Other's Wheel under the same name; Use::w reaches it,
	// as does the import itself.
	ws.Update(n["lib"], []byte("package Lib { public import Other::Wheel; }\n"), 2)
	wantLocations(t, indexReferencesAt(t, ws, s, n["other"], "Wheel"), n["lib"], n["use"])

	// A same-named alias: the use writes the alias's name and reaches Other's Wheel.
	ws.Update(n["lib"], []byte("package Lib { alias Wheel for Other::Wheel; }\n"), 3)
	wantLocations(t, indexReferencesAt(t, ws, s, n["other"], "Wheel"), n["lib"], n["use"])
	wantLocations(t, indexReferencesAt(t, ws, s, n["lib"], "Wheel"), n["use"])

	// Back to a declaration of its own: Other's Wheel loses the use again.
	ws.Update(n["lib"], []byte(refIdxLib), 4)
	wantLocations(t, indexReferencesAt(t, ws, s, n["lib"], "Wheel"), n["use"])
	wantLocations(t, indexReferencesAt(t, ws, s, n["other"], "Wheel"))
}

// Removing the declaration from the declaring document leaves the use resolving
// to nothing: it is listed for no declaration, and has no target itself.
func TestReferencesFollowCrossDocumentRemoval(t *testing.T) {
	ws, s, n := openRefIndexWorkspace(t)
	wantLocations(t, indexReferencesAt(t, ws, s, n["use"], "Wheel;"), n["use"])

	ws.Update(n["lib"], []byte("package Lib { part def Tyre; }\n"), 2)
	wantLocations(t, indexReferencesAt(t, ws, s, n["use"], "Wheel;"))
	wantLocations(t, indexReferencesAt(t, ws, s, n["other"], "Wheel"))
	wantLocations(t, indexReferencesAt(t, ws, s, n["lib"], "Tyre"))

	ws.Update(n["lib"], []byte(refIdxLib), 3)
	wantLocations(t, indexReferencesAt(t, ws, s, n["use"], "Wheel;"), n["use"])
}

// Rename edits the name a segment writes: once Lib's Wheel is an alias for Other's,
// renaming Other's Wheel leaves the use alone and renaming the alias edits it.
func TestRenameFollowsCrossDocumentRedirect(t *testing.T) {
	ws, s, n := openRefIndexWorkspace(t)

	if got := indexRenameAt(t, ws, s, n["lib"], "Wheel"); got[n["lib"]] != 1 || got[n["use"]] != 1 || len(got) != 2 {
		t.Fatalf("rename Lib::Wheel edits = %v, want lib:1 use:1", got)
	}

	ws.Update(n["lib"], []byte("package Lib { alias Wheel for Other::Wheel; }\n"), 2)
	if got := indexRenameAt(t, ws, s, n["other"], "Wheel"); got[n["other"]] != 1 || got[n["lib"]] != 1 || len(got) != 2 {
		t.Fatalf("rename Other::Wheel edits = %v, want other:1 lib:1 (the alias target), not the use", got)
	}
	if got := indexRenameAt(t, ws, s, n["lib"], "Wheel"); got[n["lib"]] != 1 || got[n["use"]] != 1 || len(got) != 2 {
		t.Fatalf("rename alias Wheel edits = %v, want lib:1 use:1", got)
	}

	ws.Update(n["lib"], []byte("package Lib { public import Other::Wheel; }\n"), 3)
	if got := indexRenameAt(t, ws, s, n["other"], "Wheel"); got[n["other"]] != 1 || got[n["lib"]] != 1 || got[n["use"]] != 1 || len(got) != 3 {
		t.Fatalf("rename re-exported Wheel edits = %v, want other:1 lib:1 use:1", got)
	}
}

// Closing a document without on-disk content, or removing one, drops its
// locations; closing back to on-disk content re-reads them from that text.
func TestReferencesDropClosedAndRemovedDocuments(t *testing.T) {
	ws, s, n := openRefIndexWorkspace(t)
	second := uri.File("/tmp/refidx_use2.sysml").Filename()
	ws.Open(second, []byte("package Use2 { import Lib::*; part w2 : Wheel; }\n"), 1)
	wantLocations(t, indexReferencesAt(t, ws, s, n["lib"], "Wheel"), n["use"], second)

	ws.Close(second)
	wantLocations(t, indexReferencesAt(t, ws, s, n["lib"], "Wheel"), n["use"])

	ws.SetOnDisk(n["use"], []byte("package Use { import Lib::*; part a : Wheel; part b : Wheel; }\n"))
	wantLocations(t, indexReferencesAt(t, ws, s, n["lib"], "Wheel"), n["use"])
	ws.Close(n["use"])
	wantLocations(t, indexReferencesAt(t, ws, s, n["lib"], "Wheel"), n["use"], n["use"])

	ws.Remove(n["use"])
	wantLocations(t, indexReferencesAt(t, ws, s, n["lib"], "Wheel"))
	if got := indexRenameAt(t, ws, s, n["lib"], "Wheel"); got[n["lib"]] != 1 || len(got) != 1 {
		t.Fatalf("rename after remove edits = %v, want lib:1 only", got)
	}
}

// Switching conformance mode drops the index like any other change; the answer
// afterwards is still the current one.
func TestReferencesSurviveConformanceModeChange(t *testing.T) {
	ws, s, n := openRefIndexWorkspace(t)
	wantLocations(t, indexReferencesAt(t, ws, s, n["lib"], "Wheel"), n["use"])
	ws.SetConformanceMode(diag.ConformanceStrict)
	wantLocations(t, indexReferencesAt(t, ws, s, n["lib"], "Wheel"), n["use"])
}
