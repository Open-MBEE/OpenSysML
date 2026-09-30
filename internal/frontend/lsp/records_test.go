package lsp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

// Hover over a reference into a document held as its record shows the
// declaration's documentation as it does over a loaded one: the hover hydrates
// the recorded document and reads the comment from its tree.
func TestHoverIntoARecordedDocumentShowsItsDocumentation(t *testing.T) {
	cache, err := libs.NewCacheIn(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	lib := filepath.Join(dir, "lib.sysml")
	main := filepath.Join(dir, "main.sysml")
	const libSource = "package Lib {\n\t/* The bus every satellite rides on. */\n\tpart def Bus;\n}\n"
	const mainSource = "package Main { private import Lib::*; part sat : Bus; }\n"
	for path, text := range map[string]string{lib: libSource, main: mainSource} {
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ws := model.NewWorkspace(model.WithRecordCache(cache))
	s := NewServer(ws)
	s.client = &fakeClient{}
	ctx := context.Background()
	if _, err := s.Initialize(ctx, &protocol.InitializeParams{
		WorkspaceFolders: []protocol.WorkspaceFolder{{URI: string(uri.File(dir)), Name: "records"}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{lib, main} {
		text := libSource
		if path == main {
			text = mainSource
		}
		if err := s.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{TextDocument: protocol.TextDocumentItem{
			URI: uri.File(path), LanguageID: "sysml", Version: 1, Text: text,
		}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DidSave(ctx, &protocol.DidSaveTextDocumentParams{TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(lib)}}); err != nil {
		t.Fatal(err)
	}
	if err := s.DidClose(ctx, &protocol.DidCloseTextDocumentParams{TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(lib)}}); err != nil {
		t.Fatal(err)
	}
	if !ws.Recorded(lib) {
		t.Fatal("lib.sysml closed unchanged is not held as its record: the fixture is vacuous")
	}
	off := strings.Index(mainSource, ": Bus") + len(": ")
	res, err := s.Hover(ctx, &protocol.HoverParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(main)},
			Position:     offsetToPosition([]byte(mainSource), off),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || !strings.Contains(res.Contents.Value, "part def Bus") ||
		!strings.Contains(res.Contents.Value, "The bus every satellite rides on.") {
		t.Fatalf("hover over Bus = %+v, want its declaration with its documentation", res)
	}
	if ws.Recorded(lib) {
		t.Fatal("hover read documentation from lib.sysml without hydrating it")
	}
}
