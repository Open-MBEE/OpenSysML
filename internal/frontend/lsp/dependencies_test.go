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

// interchangeFixture is the committed project tree: a project whose model
// imports Widgets, installed under .sysand/lib and using Gears, and
// VendoredParts, installed under a store only a library root names.
func interchangeFixture(t *testing.T) (proj, store string) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "tests", "testdata", "interchange"))
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, "proj"), filepath.Join(root, "store")
}

// projectWorkspace returns a server initialized on folder with the given
// initializationOptions, its folder scan done.
func projectWorkspace(t *testing.T, folder string, options map[string]any) (*Server, *fakeClient) {
	t.Helper()
	s := NewServer(model.NewWorkspace())
	fc := &fakeClient{}
	s.client = fc
	ctx := context.Background()
	params := &protocol.InitializeParams{
		WorkspaceFoldersInitializeParams: workspaceFolders(protocol.WorkspaceFolder{URI: uri.File(folder), Name: filepath.Base(folder)}),
	}
	if options != nil {
		params.InitializationOptions = mustLSPAny(t, options)
	}
	if _, err := s.Initialize(ctx, params); err != nil {
		t.Fatalf("Initialize err = %v", err)
	}
	if err := s.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		t.Fatalf("Initialized err = %v", err)
	}
	return s, fc
}

func readFixture(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func TestFolderScanIndexesTheProjectsTheFolderUses(t *testing.T) {
	proj, _ := interchangeFixture(t)
	s, fc := projectWorkspace(t, proj, nil)
	lib := filepath.Join(proj, ".sysand", "lib")

	widgets := filepath.Join(lib, "acme-widgets_1.0.0", "Widgets.sysml")
	gears := filepath.Join(lib, "acme-gears_2.0.0", "Gears.kerml")
	for _, name := range []string{widgets, gears} {
		if s.ws.Document(name) == nil {
			t.Errorf("%s not indexed: the installed projects the folder's project uses are part of its workspace", name)
		}
	}
	for _, name := range []string{filepath.Join(lib, "acme-gears_1.0.0", "Gears.kerml"), filepath.Join(proj, ".unrelated", "shadow.sysml")} {
		if s.ws.Document(name) != nil {
			t.Errorf("%s indexed: only the version used and no unrelated hidden directory", name)
		}
	}

	// Only VendoredParts, installed under no known root, is left unresolved.
	main := filepath.Join(proj, "model", "main.sysml")
	openFile(t, s, main, readFixture(t, main))
	msgs := diagnosticsFor(fc, main)
	if len(msgs) != 2 {
		t.Fatalf("diagnostics for main = %v, want the two VendoredParts references", msgs)
	}
	for _, msg := range msgs {
		if !strings.Contains(msg, "VendoredPart") {
			t.Errorf("diagnostic %q, want one about VendoredParts", msg)
		}
	}
}

func TestLibraryRootsSettingAddsAStoreOfInstalledProjects(t *testing.T) {
	proj, store := interchangeFixture(t)
	s, fc := projectWorkspace(t, proj, map[string]any{"sysml": map[string]any{"libraryRoots": []any{store}}})

	vendored := filepath.Join(store, "acme", "Vendored Parts", "VendoredParts.sysml")
	if s.ws.Document(vendored) == nil {
		t.Fatalf("%s not indexed through the libraryRoots setting", vendored)
	}
	main := filepath.Join(proj, "model", "main.sysml")
	openFile(t, s, main, readFixture(t, main))
	if msgs := diagnosticsFor(fc, main); len(msgs) != 0 {
		t.Errorf("diagnostics for main = %v, want none with every used project found", msgs)
	}
}

func TestLibraryRootsConfigurationChangeReindexesTheDependencies(t *testing.T) {
	proj, store := interchangeFixture(t)
	s, fc := projectWorkspace(t, proj, nil)
	ctx := context.Background()
	main := filepath.Join(proj, "model", "main.sysml")
	openFile(t, s, main, readFixture(t, main))
	if msgs := diagnosticsFor(fc, main); len(msgs) == 0 {
		t.Fatal("diagnostics for main = none before the store is named")
	}

	// A relative root is taken from the folder; the change republishes main.
	relative, err := filepath.Rel(proj, store)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DidChangeConfiguration(ctx, &protocol.DidChangeConfigurationParams{
		Settings: mustLSPAny(t, map[string]any{"sysml.libraryRoots": []any{relative}}),
	}); err != nil {
		t.Fatalf("DidChangeConfiguration err = %v", err)
	}
	vendored := filepath.Join(store, "acme", "Vendored Parts", "VendoredParts.sysml")
	if s.ws.Document(vendored) == nil {
		t.Fatalf("%s not indexed after the setting named its store", vendored)
	}
	if msgs := diagnosticsFor(fc, main); len(msgs) != 0 {
		t.Errorf("diagnostics for main = %v after the change, want none", msgs)
	}

	// Taking the root away drops what only it contributed; a malformed value
	// changes nothing.
	if err := s.DidChangeConfiguration(ctx, &protocol.DidChangeConfigurationParams{
		Settings: mustLSPAny(t, map[string]any{"libraryRoots": "not a list"}),
	}); err != nil {
		t.Fatalf("DidChangeConfiguration err = %v", err)
	}
	if s.ws.Document(vendored) == nil {
		t.Errorf("%s dropped by a malformed setting", vendored)
	}
	if err := s.DidChangeConfiguration(ctx, &protocol.DidChangeConfigurationParams{
		Settings: mustLSPAny(t, map[string]any{"libraryRoots": []any{}}),
	}); err != nil {
		t.Fatalf("DidChangeConfiguration err = %v", err)
	}
	if s.ws.Document(vendored) != nil {
		t.Errorf("%s still indexed after the root was taken away", vendored)
	}
	widgets := filepath.Join(proj, ".sysand", "lib", "acme-widgets_1.0.0", "Widgets.sysml")
	if s.ws.Document(widgets) == nil {
		t.Errorf("%s dropped with the store, though the project's own library still holds it", widgets)
	}
	if msgs := diagnosticsFor(fc, main); len(msgs) != 2 {
		t.Errorf("diagnostics for main = %v after the root was taken away, want the two VendoredParts references", msgs)
	}
}

func TestRescanKeepsTheDependenciesAndDropsAFolderWithThem(t *testing.T) {
	proj, store := interchangeFixture(t)
	s, _ := projectWorkspace(t, proj, map[string]any{"libraryRoots": []any{store}})
	ctx := context.Background()
	vendored := filepath.Join(store, "acme", "Vendored Parts", "VendoredParts.sysml")
	widgets := filepath.Join(proj, ".sysand", "lib", "acme-widgets_1.0.0", "Widgets.sysml")

	s.Rescan(ctx)
	for _, name := range []string{vendored, widgets} {
		if s.ws.Document(name) == nil {
			t.Errorf("%s dropped by a rescan that found it again", name)
		}
	}

	if err := s.DidChangeWorkspaceFolders(ctx, &protocol.DidChangeWorkspaceFoldersParams{
		Event: protocol.WorkspaceFoldersChangeEvent{Removed: []protocol.WorkspaceFolder{{URI: uri.File(proj), Name: "proj"}}},
	}); err != nil {
		t.Fatalf("DidChangeWorkspaceFolders err = %v", err)
	}
	for _, name := range []string{vendored, widgets, filepath.Join(proj, "model", "main.sysml")} {
		if s.ws.Document(name) != nil {
			t.Errorf("%s still indexed after its folder was removed", name)
		}
	}
}

func TestOpeningAFileInAnInstalledProjectIndexesWhatItUses(t *testing.T) {
	proj, _ := interchangeFixture(t)
	s := NewServer(model.NewWorkspace())
	fc := &fakeClient{}
	s.client = fc
	if _, err := s.Initialize(context.Background(), &protocol.InitializeParams{}); err != nil {
		t.Fatalf("Initialize err = %v", err)
	}
	widgets := filepath.Join(proj, ".sysand", "lib", "acme-widgets_1.0.0", "Widgets.sysml")
	openFile(t, s, widgets, readFixture(t, widgets))

	gears := filepath.Join(proj, ".sysand", "lib", "acme-gears_2.0.0", "Gears.kerml")
	if s.ws.Document(gears) == nil {
		t.Fatalf("%s not indexed: an installed project's usages are found in the library it is installed in", gears)
	}
	if msgs := diagnosticsFor(fc, widgets); len(msgs) != 0 {
		t.Errorf("diagnostics for Widgets = %v, want none", msgs)
	}
}

// twoProjectWorkspace writes project A, whose model imports Shared from
// project B installed beside it, and project B as a folder of its own, and
// returns both directories; B is reached through a library root naming root.
func twoProjectWorkspace(t *testing.T) (a, b, root string) {
	t.Helper()
	root = t.TempDir()
	a = filepath.Join(root, "a")
	b = filepath.Join(root, "b")
	writeFile(t, filepath.Join(a, ".project.json"), `{"name": "a", "publisher": "acme", "version": "1", "usage": [{"resource": "pkg:r/acme/b"}]}`)
	writeFile(t, filepath.Join(a, "model", "main.sysml"), "package Main {\n    private import Shared::*;\n    part s : Thing;\n}\n")
	writeFile(t, filepath.Join(a, "common", "local.sysml"), "package Local {\n    part def L;\n}\n")
	writeFile(t, filepath.Join(b, ".project.json"), `{"name": "b", "publisher": "acme", "version": "1"}`)
	writeFile(t, filepath.Join(b, "Shared.sysml"), "package Shared {\n    part def Thing;\n}\n")
	return a, b, root
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRemovingAFolderKeepsTheSourcesAnotherFoldersProjectUses(t *testing.T) {
	a, b, root := twoProjectWorkspace(t)
	s := NewServer(model.NewWorkspace())
	s.client = &fakeClient{}
	ctx := context.Background()
	if _, err := s.Initialize(ctx, &protocol.InitializeParams{
		WorkspaceFoldersInitializeParams: workspaceFolders(protocol.WorkspaceFolder{URI: uri.File(a), Name: "a"}, protocol.WorkspaceFolder{URI: uri.File(b), Name: "b"}),
		InitializationOptions:            mustLSPAny(t, map[string]any{"libraryRoots": []any{root}}),
	}); err != nil {
		t.Fatalf("Initialize err = %v", err)
	}
	if err := s.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		t.Fatalf("Initialized err = %v", err)
	}
	shared := filepath.Join(b, "Shared.sysml")
	if s.ws.Document(shared) == nil {
		t.Fatalf("%s not indexed", shared)
	}

	if err := s.DidChangeWorkspaceFolders(ctx, &protocol.DidChangeWorkspaceFoldersParams{
		Event: protocol.WorkspaceFoldersChangeEvent{Removed: []protocol.WorkspaceFolder{{URI: uri.File(b), Name: "b"}}},
	}); err != nil {
		t.Fatalf("DidChangeWorkspaceFolders err = %v", err)
	}
	if s.ws.Document(shared) == nil {
		t.Errorf("%s dropped with folder b, though folder a's project uses it", shared)
	}
	if err := s.DidChangeWorkspaceFolders(ctx, &protocol.DidChangeWorkspaceFoldersParams{
		Event: protocol.WorkspaceFoldersChangeEvent{Removed: []protocol.WorkspaceFolder{{URI: uri.File(a), Name: "a"}}},
	}); err != nil {
		t.Fatalf("DidChangeWorkspaceFolders err = %v", err)
	}
	if s.ws.Document(shared) != nil {
		t.Errorf("%s still indexed once no folder uses it", shared)
	}
}

func TestChangedProjectFileFindsTheProjectsAgain(t *testing.T) {
	a, b, root := twoProjectWorkspace(t)
	writeFile(t, filepath.Join(a, ".project.json"), `{"name": "a", "publisher": "acme", "version": "1"}`)
	s, fc := projectWorkspace(t, a, map[string]any{"libraryRoots": []any{root}})
	ctx := context.Background()
	main := filepath.Join(a, "model", "main.sysml")
	openFile(t, s, main, "package Main {\n    private import Shared::*;\n    part s : Thing;\n}\n")
	shared := filepath.Join(b, "Shared.sysml")
	if s.ws.Document(shared) != nil || len(diagnosticsFor(fc, main)) == 0 {
		t.Fatalf("%s indexed before the project used it", shared)
	}

	// Adding the usage, as a package manager does, is a change to the project
	// file the editor watches.
	writeFile(t, filepath.Join(a, ".project.json"), `{"name": "a", "publisher": "acme", "version": "1", "usage": [{"resource": "pkg:r/acme/b"}]}`)
	if err := s.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: []protocol.FileEvent{{URI: uri.File(filepath.Join(a, ".project.json")), Type: protocol.FileChangeTypeChanged}},
	}); err != nil {
		t.Fatalf("DidChangeWatchedFiles err = %v", err)
	}
	if s.ws.Document(shared) == nil {
		t.Errorf("%s not indexed after the project file named its project", shared)
	}
	if msgs := diagnosticsFor(fc, main); len(msgs) != 0 {
		t.Errorf("diagnostics for main = %v after the usage was added, want none", msgs)
	}
}

func TestOpeningAFileInAProjectOutsideFoldersIndexesTheWholeProject(t *testing.T) {
	a, _, _ := twoProjectWorkspace(t)
	s := NewServer(model.NewWorkspace())
	s.client = &fakeClient{}
	if _, err := s.Initialize(context.Background(), &protocol.InitializeParams{}); err != nil {
		t.Fatalf("Initialize err = %v", err)
	}
	main := filepath.Join(a, "model", "main.sysml")
	openFile(t, s, main, "package Main {\n    private import Local::*;\n    part l : L;\n}\n")

	local := filepath.Join(a, "common", "local.sysml")
	if s.ws.Document(local) == nil {
		t.Fatalf("%s not indexed: the opened document's project is its workspace", local)
	}
	if err := s.DidClose(context.Background(), &protocol.DidCloseTextDocumentParams{TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(main)}}); err != nil {
		t.Fatalf("DidClose err = %v", err)
	}
	if s.ws.Document(local) != nil {
		t.Errorf("%s still indexed after the last document of its project closed", local)
	}
}
