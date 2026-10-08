package lsp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// chainedWorkspace writes n files into a folder, each importing the one before
// it, and returns a server whose folder scan has not run yet.
func chainedWorkspace(t *testing.T, n int) (*Server, *fakeClient, []string, []string) {
	t.Helper()
	dir := t.TempDir()
	paths := make([]string, n)
	texts := make([]string, n)
	for i := range paths {
		text := fmt.Sprintf("package Lib%d {\n    part def Widget%d;\n", i, i)
		if i > 0 {
			text += fmt.Sprintf("    private import Lib%d::*;\n    part w%d : Widget%d;\n", i-1, i, i-1)
		}
		text += "}\n"
		paths[i] = filepath.Join(dir, fmt.Sprintf("lib%d.sysml", i))
		texts[i] = text
		if err := os.WriteFile(paths[i], []byte(text), 0o600); err != nil {
			t.Fatalf("write %s: %v", paths[i], err)
		}
	}
	s := NewServer(model.NewWorkspace())
	fc := &fakeClient{}
	s.client = fc
	if _, err := s.Initialize(context.Background(), &protocol.InitializeParams{
		WorkspaceFolders: []protocol.WorkspaceFolder{{URI: string(uri.File(dir)), Name: "chain"}},
	}); err != nil {
		t.Fatalf("Initialize err = %v", err)
	}
	return s, fc, paths, texts
}

// Opening the files a folder scan indexed, as an editor restoring its tabs does,
// must not redo the scan's work document by document: the scan is one batch of
// the workspace, and a buffer holding what was scanned is not an edit. Every
// generation of the workspace is a full invalidation of its memoized semantics,
// so the count of them is the measure; one diagnostics push per document shows
// no open re-analyzed the others.
func TestOpeningScannedFilesIsNotAnEdit(t *testing.T) {
	s, fc, paths, texts := chainedWorkspace(t, 6)
	ctx := context.Background()
	ws := s.ws

	before := ws.Generation()
	if err := s.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		t.Fatalf("Initialized err = %v", err)
	}
	if got := ws.Generation() - before; got != 1 {
		t.Fatalf("the folder scan took %d generations for %d files, want one batch", got, len(paths))
	}

	scanned := ws.Generation()
	for i, path := range paths {
		openFile(t, s, path, texts[i])
	}
	if ws.Generation() != scanned {
		t.Fatalf("opening the scanned files moved the generation from %d to %d", scanned, ws.Generation())
	}
	if got := len(fc.all()); got != len(paths) {
		t.Fatalf("%d diagnostics pushes for %d opens, want one per document", got, len(paths))
	}
	for _, path := range paths {
		if msgs, ok := lastDiagnostics(fc, path); !ok || len(msgs) != 0 {
			t.Errorf("%s: diagnostics %v published=%v, want a clean push", filepath.Base(path), msgs, ok)
		}
		if doc := ws.Document(path); doc == nil || doc.Version != 1 || !ws.IsOpen(path) {
			t.Errorf("%s: document %+v open=%v, want version 1 and open", filepath.Base(path), doc, ws.IsOpen(path))
		}
	}

	// The last file's `w5 : Widget4` reads the file before it through its import.
	last := paths[len(paths)-1]
	line := strings.Index(texts[len(paths)-1], "part w5 : Widget4")
	lineNo := strings.Count(texts[len(paths)-1][:line], "\n")
	col := len("    part w5 : Widget4") - 1
	hover, err := s.Hover(ctx, &protocol.HoverParams{TextDocumentPositionParams: protocol.TextDocumentPositionParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(last)},
		Position:     protocol.Position{Line: uint32(lineNo), Character: uint32(col)},
	}})
	if err != nil {
		t.Fatalf("Hover err = %v", err)
	}
	if hover == nil || !strings.Contains(hover.Contents.Value, "Widget4") {
		t.Fatalf("hover over Widget4 = %+v, want its definition", hover)
	}
	if ws.Generation() != scanned {
		t.Fatalf("the hover moved the generation from %d to %d", scanned, ws.Generation())
	}

	// A buffer that differs from the file is an edit, and is what the diagnostics report.
	edited := strings.Replace(texts[0], "Widget0;", "Widget0;\n    part broken : Missing;", 1)
	s.applyDidChange(ctx, paths[0], []rawContentChange{{Text: edited}}, 2)
	if ws.Generation() == scanned {
		t.Fatal("an edited buffer left the generation alone")
	}
	if msgs := diagnosticsFor(fc, paths[0]); len(msgs) != 1 || !strings.Contains(msgs[0], "Missing") {
		t.Fatalf("diagnostics after the edit = %v, want the unresolved name", msgs)
	}
}
