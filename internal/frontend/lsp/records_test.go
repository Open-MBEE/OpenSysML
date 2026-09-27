package lsp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// Saving a document writes its record to the cache, so a later workspace holds
// it closed as that record; closing a document unchanged on disk demotes it to
// the record, and opening it hydrates it again.
func TestDidSaveWritesRecordAndCloseDemotes(t *testing.T) {
	cache, err := libs.NewCacheIn(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	lib := filepath.Join(dir, "lib.sysml")
	main := filepath.Join(dir, "main.sysml")
	const libSource = "package Lib { part def Bus; }"
	const mainSource = "package Main { private import Lib::*; part sat : Bus; part odd : Missing; }"
	for path, text := range map[string]string{lib: libSource, main: mainSource} {
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ws := model.NewWorkspace(model.WithRecordCache(cache))
	s := NewServer(ws)
	fc := &fakeClient{}
	s.client = fc
	ctx := context.Background()
	if _, err := s.Initialize(ctx, &protocol.InitializeParams{
		WorkspaceFolders: []protocol.WorkspaceFolder{{URI: string(uri.File(dir)), Name: "records"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		t.Fatal(err)
	}
	if err := s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{
		URI: uri.File(main), LanguageID: "sysml", Version: 1, Text: mainSource,
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.DidSave(ctx, &protocol.DidSaveTextDocumentParams{TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(main)}}); err != nil {
		t.Fatal(err)
	}
	want := diagnosticsFor(fc, main)
	if len(want) == 0 {
		t.Fatal("main.sysml reports nothing: the fixture is vacuous")
	}

	later := model.NewWorkspace(model.WithRecordCache(cache))
	later.OpenAll([]model.Input{
		{Name: lib, Content: []byte(libSource), Version: 1},
		{Name: main, Content: []byte(mainSource), Version: 1},
	})
	if !later.Recorded(main) {
		t.Fatal("the saved main.sysml is not held as its record by a later workspace")
	}
	got := later.Diagnostics(main)
	if len(got) != len(want) {
		t.Fatalf("recorded main.sysml reports %d diagnostics, the save published %d", len(got), len(want))
	}
	for i, d := range got {
		if d.Message != want[i] {
			t.Fatalf("recorded diagnostic %d is %q, published %q", i, d.Message, want[i])
		}
	}

	if err := s.DidClose(ctx, &protocol.DidCloseTextDocumentParams{TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(main)}}); err != nil {
		t.Fatal(err)
	}
	if !ws.Recorded(main) {
		t.Fatal("main.sysml closed unchanged is not held as its record")
	}
	if err := s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{
		URI: uri.File(main), LanguageID: "sysml", Version: 2, Text: mainSource,
	}}); err != nil {
		t.Fatal(err)
	}
	if ws.Recorded(main) {
		t.Fatal("an open main.sysml is held as its record")
	}
	if got := diagnosticsFor(fc, main); len(got) != len(want) {
		t.Fatalf("reopened main.sysml published %v, want %v", got, want)
	}
}
