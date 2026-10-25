package lsp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// A document opened under no folder has its own directory indexed and nothing
// above it: a model file in the parent directory stays out of the workspace.
func TestOpenedDocumentIndexesOnlyItsOwnDirectory(t *testing.T) {
	root := t.TempDir()
	stray := filepath.Join(root, "stray.sysml")
	if err := os.WriteFile(stray, []byte("package Stray;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "proj")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(dir, "sibling.sysml")
	if err := os.WriteFile(sibling, []byte("package Sibling;\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	s := NewServer(model.NewWorkspace())
	docURI := uri.File(filepath.Join(dir, "doc.sysml"))
	openDoc(t, s, docURI, "package P;\n")

	names := s.ws.DocumentNames()
	if !containsName(names, sibling) {
		t.Errorf("sibling %s not indexed; documents = %v", sibling, names)
	}
	if containsName(names, stray) {
		t.Errorf("%s in the parent directory was indexed; documents = %v", stray, names)
	}
	if len(s.openDirs) != 1 || !s.openDirs[dir] {
		t.Errorf("indexed directories = %v, want only %s", s.openDirs, dir)
	}
}

// Every helper that opens a document places it under the test's root: a stray
// model file in the system temp directory is never indexed, nor any directory outside.
func TestOpenHelpersStayUnderTheTestRoot(t *testing.T) {
	stray := filepath.Join(os.TempDir(), "opensysml-lsp-stray-guard.sysml")
	if err := os.WriteFile(stray, []byte("package OpenSysMLStrayGuard;\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(stray) })
	testRoot := filepath.Dir(t.TempDir())

	const src = "package P { part def Wheel; }\n"
	check := func(what string, s *Server) {
		t.Helper()
		if names := s.ws.DocumentNames(); containsName(names, stray) {
			t.Errorf("%s indexed %s outside the test root; documents = %v", what, stray, names)
		}
		if len(s.openDirs) == 0 {
			t.Errorf("%s indexed no directory for the document it opened", what)
		}
		for dir := range s.openDirs {
			if !underFolder(dir, testRoot) {
				t.Errorf("%s indexed %s, outside the test root %s", what, dir, testRoot)
			}
		}
	}

	w := newWireSession(t)
	w.open(src)
	if !strings.HasPrefix(uriToName(uri.URI(w.uri)), testRoot+string(filepath.Separator)) {
		t.Errorf("wire session document %s is outside the test root %s", w.uri, testRoot)
	}
	check("wireSession.open", w.s)

	s := NewServer(model.NewWorkspace())
	openDoc(t, s, tempDocURI(t, "a.sysml"), src)
	check("openDoc(tempDocURI)", s)

	s, _ = renderServer(t, "kit.sysml", src)
	check("renderServer", s)

	s, _, _ = debugServer(t, "m.sysml", src)
	check("debugServer", s)

	s = NewServer(model.NewWorkspace())
	if err := s.DidOpen(context.Background(), &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: tempDocURI(t, "b.sysml"), LanguageID: "sysml", Version: 1, Text: src},
	}); err != nil {
		t.Fatalf("DidOpen err = %v", err)
	}
	check("DidOpen(tempDocURI)", s)
}

func containsName(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}
