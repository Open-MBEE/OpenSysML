package lsp

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

const (
	libSource  = "package Lib {\n    part def Widget;\n}\n"
	mainSource = "package Main {\n    private import Lib::*;\n    part w : Widget;\n}\n"
)

// multiFileWorkspace writes lib.sysml and main.sysml into a temporary folder and
// returns a server initialized on it, with the folder scan already done.
func multiFileWorkspace(t *testing.T) (*Server, *fakeClient, string, string, string) {
	t.Helper()
	dir := t.TempDir()
	lib := filepath.Join(dir, "lib.sysml")
	main := filepath.Join(dir, "main.sysml")
	if err := os.WriteFile(lib, []byte(libSource), 0o600); err != nil {
		t.Fatalf("write lib: %v", err)
	}
	if err := os.WriteFile(main, []byte(mainSource), 0o600); err != nil {
		t.Fatalf("write main: %v", err)
	}

	s := NewServer(model.NewWorkspace())
	fc := &fakeClient{}
	s.client = fc
	ctx := context.Background()
	if _, err := s.Initialize(ctx, &protocol.InitializeParams{
		WorkspaceFolders: []protocol.WorkspaceFolder{{URI: string(uri.File(dir)), Name: "multi"}},
	}); err != nil {
		t.Fatalf("Initialize err = %v", err)
	}
	if err := s.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		t.Fatalf("Initialized err = %v", err)
	}
	return s, fc, dir, lib, main
}

// diagnosticsFor returns the messages of the last diagnostics published for name.
func diagnosticsFor(fc *fakeClient, name string) []string {
	msgs, _ := lastDiagnostics(fc, name)
	return msgs
}

// lastDiagnostics returns the messages of the last diagnostics published for
// name, and whether any were published at all.
func lastDiagnostics(fc *fakeClient, name string) ([]string, bool) {
	var msgs []string
	found := false
	for _, p := range fc.all() {
		if p.URI != uri.File(name) {
			continue
		}
		found = true
		msgs = msgs[:0]
		for _, d := range p.Diagnostics {
			msgs = append(msgs, d.Message)
		}
	}
	return msgs, found
}

// waitForDiagnostics polls until name's last published diagnostics satisfy want,
// which the debounced cross-document refresh reaches asynchronously.
func waitForDiagnostics(t *testing.T, fc *fakeClient, name string, want func([]string) bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		msgs := diagnosticsFor(fc, name)
		if want(msgs) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("diagnostics for %s = %v, still unwanted after 5s", name, msgs)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func openFile(t *testing.T, s *Server, path, text string) {
	t.Helper()
	if err := s.DidOpen(context.Background(), &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        uri.File(path),
			LanguageID: "sysml",
			Version:    1,
			Text:       text,
		},
	}); err != nil {
		t.Fatalf("DidOpen err = %v", err)
	}
}

func TestFolderScanResolvesNamesFromUnopenedFiles(t *testing.T) {
	s, fc, _, lib, main := multiFileWorkspace(t)

	// Only main.sysml is opened; Lib::Widget lives in a file the editor never
	// showed, and must still resolve.
	openFile(t, s, main, mainSource)
	if msgs := diagnosticsFor(fc, main); len(msgs) != 0 {
		t.Fatalf("diagnostics for main = %v, want none", msgs)
	}
	if s.ws.Document(lib) == nil {
		t.Errorf("lib.sysml not indexed by the folder scan")
	}
	if s.ws.IsOpen(lib) {
		t.Errorf("lib.sysml reported open; the scan records on-disk content only")
	}
}

func TestFolderScanSkipsHiddenDirectories(t *testing.T) {
	s, _, dir, _, _ := multiFileWorkspace(t)

	hidden := filepath.Join(dir, ".git", "hidden.sysml")
	if err := os.MkdirAll(filepath.Dir(hidden), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(hidden, []byte("package Hidden;\n"), 0o600); err != nil {
		t.Fatalf("write hidden: %v", err)
	}
	s.loadFolder(dir)
	if s.ws.Document(hidden) != nil {
		t.Errorf("indexed %q, want hidden directories skipped", hidden)
	}
}

func TestWatchedFileCreateAndChangeReindex(t *testing.T) {
	s, fc, dir, _, main := multiFileWorkspace(t)
	ctx := context.Background()
	openFile(t, s, main, mainSource)

	// A second definition file appears outside the editor; main.sysml can use it
	// as soon as the client reports the creation.
	extra := filepath.Join(dir, "extra.sysml")
	if err := os.WriteFile(extra, []byte("package Extra {\n    part def Gizmo;\n}\n"), 0o600); err != nil {
		t.Fatalf("write extra: %v", err)
	}
	if err := s.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: []*protocol.FileEvent{{URI: uri.File(extra), Type: protocol.FileChangeTypeCreated}},
	}); err != nil {
		t.Fatalf("DidChangeWatchedFiles err = %v", err)
	}
	if s.ws.Document(extra) == nil {
		t.Fatalf("extra.sysml not indexed after a create event")
	}

	usesGizmo := "package Main {\n    private import Extra::*;\n    part g : Gizmo;\n}\n"
	if err := s.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri.File(main)},
			Version:                2,
		},
		// A whole-document range: the typed DidChange path splices at [0,0) for a
		// zero Range, which would leave the old text in place (see sync.go).
		ContentChanges: []protocol.TextDocumentContentChangeEvent{{
			Range: protocol.Range{End: protocol.Position{Line: 4}},
			Text:  usesGizmo,
		}},
	}); err != nil {
		t.Fatalf("DidChange err = %v", err)
	}
	if msgs := diagnosticsFor(fc, main); len(msgs) != 0 {
		t.Fatalf("diagnostics for main = %v, want none", msgs)
	}

	// Rewriting the file on disk renames what it declares; the open document's
	// diagnostics follow.
	if err := os.WriteFile(extra, []byte("package Extra {\n    part def Doohickey;\n}\n"), 0o600); err != nil {
		t.Fatalf("rewrite extra: %v", err)
	}
	if err := s.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: []*protocol.FileEvent{{URI: uri.File(extra), Type: protocol.FileChangeTypeChanged}},
	}); err != nil {
		t.Fatalf("DidChangeWatchedFiles err = %v", err)
	}
	if msgs := diagnosticsFor(fc, main); len(msgs) == 0 {
		t.Fatalf("diagnostics for main = none, want an unresolved Gizmo")
	}
}

func TestWatchedFileDeleteUnindexesClosedDocument(t *testing.T) {
	s, fc, _, lib, main := multiFileWorkspace(t)
	ctx := context.Background()
	openFile(t, s, main, mainSource)

	if err := os.Remove(lib); err != nil {
		t.Fatalf("remove lib: %v", err)
	}
	if err := s.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: []*protocol.FileEvent{{URI: uri.File(lib), Type: protocol.FileChangeTypeDeleted}},
	}); err != nil {
		t.Fatalf("DidChangeWatchedFiles err = %v", err)
	}
	if s.ws.Document(lib) != nil {
		t.Errorf("lib.sysml still indexed after deletion")
	}
	if msgs := diagnosticsFor(fc, main); len(msgs) == 0 {
		t.Fatalf("diagnostics for main = none, want unresolved references")
	}
}

func TestWatchedFileDeleteKeepsOpenBuffer(t *testing.T) {
	s, fc, _, lib, main := multiFileWorkspace(t)
	ctx := context.Background()
	openFile(t, s, main, mainSource)
	openFile(t, s, lib, libSource)

	// The file is gone from disk, but its editor buffer is still authoritative.
	if err := os.Remove(lib); err != nil {
		t.Fatalf("remove lib: %v", err)
	}
	if err := s.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: []*protocol.FileEvent{{URI: uri.File(lib), Type: protocol.FileChangeTypeDeleted}},
	}); err != nil {
		t.Fatalf("DidChangeWatchedFiles err = %v", err)
	}
	if s.ws.Document(lib) == nil {
		t.Fatalf("open buffer for lib.sysml dropped on disk deletion")
	}
	if msgs := diagnosticsFor(fc, main); len(msgs) != 0 {
		t.Fatalf("diagnostics for main = %v, want none", msgs)
	}
}

func TestDidCloseKeepsDocumentIndexedFromDisk(t *testing.T) {
	s, fc, _, lib, main := multiFileWorkspace(t)
	ctx := context.Background()
	openFile(t, s, main, mainSource)
	openFile(t, s, lib, libSource)

	if err := s.DidClose(ctx, &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(lib)},
	}); err != nil {
		t.Fatalf("DidClose err = %v", err)
	}
	if s.ws.Document(lib) == nil {
		t.Fatalf("lib.sysml unindexed by closing its tab")
	}
	if msgs := diagnosticsFor(fc, main); len(msgs) != 0 {
		t.Fatalf("diagnostics for main = %v, want none", msgs)
	}
}

func TestDidCloseDropsDocumentWithNoFileOnDisk(t *testing.T) {
	s := NewServer(model.NewWorkspace())
	s.client = &fakeClient{}
	name := uri.File(filepath.Join(t.TempDir(), "untitled.sysml")).Filename()
	openFile(t, s, name, "package P;\n")

	if err := s.DidClose(context.Background(), &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(name)},
	}); err != nil {
		t.Fatalf("DidClose err = %v", err)
	}
	if s.ws.Document(name) != nil {
		t.Errorf("document with no file on disk kept after close")
	}
}

func TestDidCloseWithdrawsDiagnostics(t *testing.T) {
	s, fc, _, lib, _ := multiFileWorkspace(t)
	ctx := context.Background()

	// The buffer holds an error the file on disk does not. Closing discards the
	// buffer, and a closed document's markers are never refreshed again, so they
	// are withdrawn rather than frozen at what closing happened to see.
	openFile(t, s, lib, "package Lib {\n    part def Widget;\n    part b : Nope;\n}\n")
	if msgs := diagnosticsFor(fc, lib); len(msgs) == 0 {
		t.Fatalf("diagnostics for the lib buffer = none, want an unresolved Nope")
	}
	if err := s.DidClose(ctx, &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(lib)},
	}); err != nil {
		t.Fatalf("DidClose err = %v", err)
	}
	msgs, published := lastDiagnostics(fc, lib)
	if !published || len(msgs) != 0 {
		t.Fatalf("diagnostics for the closed lib = %v (published=%v), want an empty set", msgs, published)
	}
}

// serialClient records whether two diagnostics sends ever overlapped.
type serialClient struct {
	baseClient
	mu       sync.Mutex
	active   int
	overlaps atomic.Bool
}

func (c *serialClient) PublishDiagnostics(ctx context.Context, params *protocol.PublishDiagnosticsParams) error {
	c.mu.Lock()
	c.active++
	if c.active > 1 {
		c.overlaps.Store(true)
	}
	c.mu.Unlock()
	time.Sleep(time.Millisecond)
	c.mu.Lock()
	c.active--
	c.mu.Unlock()
	return nil
}

func TestPublishDiagnosticsSerialized(t *testing.T) {
	s, _, _, lib, main := multiFileWorkspace(t)
	ctx := context.Background()
	openFile(t, s, main, mainSource)
	openFile(t, s, lib, libSource)

	// The coalesced sweep publishes from a timer goroutine while handlers publish
	// from theirs; an overlap would let the older set land last on the client.
	sc := &serialClient{}
	s.client = sc
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		name := main
		if i%2 == 0 {
			name = lib
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				s.publishDiagnostics(ctx, name)
				s.clearDiagnostics(ctx, name)
			}
		}()
	}
	wg.Wait()
	if sc.overlaps.Load() {
		t.Errorf("diagnostics sends overlapped, want them serialized")
	}
}

// closeOnPublishClient closes a document while a sweep is publishing, the window
// in which a closed tab's withdrawn markers could be republished.
type closeOnPublishClient struct {
	baseClient
	ws      *model.Workspace
	trigger string
	close   string

	mu         sync.Mutex
	closed     bool
	afterClose []string
}

func (c *closeOnPublishClient) PublishDiagnostics(ctx context.Context, params *protocol.PublishDiagnosticsParams) error {
	name := uriToName(params.URI)
	c.mu.Lock()
	if c.closed {
		c.afterClose = append(c.afterClose, name)
	}
	trip := !c.closed && name == c.trigger
	c.mu.Unlock()
	if trip {
		c.ws.Close(c.close)
		c.mu.Lock()
		c.closed = true
		c.mu.Unlock()
	}
	return nil
}

func TestRefreshOpenDiagnosticsSkipsDocumentClosedMidSweep(t *testing.T) {
	// Open-document order is map order, so repeat until the sweep hits main first.
	for i := 0; i < 10; i++ {
		s, _, _, lib, main := multiFileWorkspace(t)
		openFile(t, s, main, mainSource)
		openFile(t, s, lib, libSource)

		cc := &closeOnPublishClient{ws: s.ws, trigger: main, close: lib}
		s.client = cc
		s.refreshOpenDiagnostics(context.Background(), "")

		cc.mu.Lock()
		after := append([]string(nil), cc.afterClose...)
		cc.mu.Unlock()
		for _, name := range after {
			if name == lib {
				t.Fatalf("lib.sysml diagnostics republished after its tab closed")
			}
		}
	}
}

func TestDidChangeWorkspaceFoldersIndexesAddedFolder(t *testing.T) {
	s, fc, _, _, main := multiFileWorkspace(t)
	ctx := context.Background()

	// main.sysml uses a name declared in a folder added after initialize; the
	// client's watcher reports changes only, so the folder has to be walked.
	openFile(t, s, main, "package Main {\n    private import Extra::*;\n    part g : Gadget;\n}\n")
	if msgs := diagnosticsFor(fc, main); len(msgs) == 0 {
		t.Fatalf("diagnostics for main = none, want Gadget unresolved before the folder is added")
	}
	// Outside the initial folder, which stays registered and would otherwise
	// still cover the file after the added folder goes away.
	added := t.TempDir()
	extra := filepath.Join(added, "extra.sysml")
	if err := os.WriteFile(extra, []byte("package Extra {\n    part def Gadget;\n}\n"), 0o600); err != nil {
		t.Fatalf("write extra: %v", err)
	}
	if err := s.DidChangeWorkspaceFolders(ctx, &protocol.DidChangeWorkspaceFoldersParams{
		Event: protocol.WorkspaceFoldersChangeEvent{
			Added: []protocol.WorkspaceFolder{{URI: string(uri.File(added)), Name: "extra"}},
		},
	}); err != nil {
		t.Fatalf("DidChangeWorkspaceFolders err = %v", err)
	}
	if s.ws.Document(extra) == nil {
		t.Errorf("extra.sysml not indexed after its folder was added")
	}
	if msgs := diagnosticsFor(fc, main); len(msgs) != 0 {
		t.Errorf("diagnostics for main = %v, want none once the folder is indexed", msgs)
	}

	// Removing the folder again unindexes what it contributed.
	if err := s.DidChangeWorkspaceFolders(ctx, &protocol.DidChangeWorkspaceFoldersParams{
		Event: protocol.WorkspaceFoldersChangeEvent{
			Removed: []protocol.WorkspaceFolder{{URI: string(uri.File(added)), Name: "extra"}},
		},
	}); err != nil {
		t.Fatalf("DidChangeWorkspaceFolders err = %v", err)
	}
	if s.ws.Document(extra) != nil {
		t.Errorf("extra.sysml still indexed after its folder was removed")
	}
	if msgs := diagnosticsFor(fc, main); len(msgs) == 0 {
		t.Errorf("diagnostics for main = none, want Gadget unresolved again")
	}
}

func TestDidChangeWorkspaceFoldersKeepsOpenBuffer(t *testing.T) {
	s, _, dir, lib, _ := multiFileWorkspace(t)

	// A removed folder leaves a buffer the editor still shows alone.
	openFile(t, s, lib, libSource)
	if err := s.DidChangeWorkspaceFolders(context.Background(), &protocol.DidChangeWorkspaceFoldersParams{
		Event: protocol.WorkspaceFoldersChangeEvent{
			Removed: []protocol.WorkspaceFolder{{URI: string(uri.File(dir)), Name: "multi"}},
		},
	}); err != nil {
		t.Fatalf("DidChangeWorkspaceFolders err = %v", err)
	}
	if s.ws.Document(lib) == nil {
		t.Errorf("open lib.sysml dropped with its folder")
	}
}

func TestDidChangeWorkspaceFoldersKeepsNestedFolder(t *testing.T) {
	s, _, dir, lib, _ := multiFileWorkspace(t)
	ctx := context.Background()

	// LSP allows nested folders: removing the outer one must leave the files the
	// inner one still contributes indexed.
	nested := filepath.Join(dir, "nested")
	inner := filepath.Join(nested, "inner.sysml")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(inner, []byte("package Inner {\n    part def Gizmo;\n}\n"), 0o600); err != nil {
		t.Fatalf("write inner: %v", err)
	}
	if err := s.DidChangeWorkspaceFolders(ctx, &protocol.DidChangeWorkspaceFoldersParams{
		Event: protocol.WorkspaceFoldersChangeEvent{
			Added: []protocol.WorkspaceFolder{{URI: string(uri.File(nested)), Name: "nested"}},
		},
	}); err != nil {
		t.Fatalf("DidChangeWorkspaceFolders err = %v", err)
	}
	if err := s.DidChangeWorkspaceFolders(ctx, &protocol.DidChangeWorkspaceFoldersParams{
		Event: protocol.WorkspaceFoldersChangeEvent{
			Removed: []protocol.WorkspaceFolder{{URI: string(uri.File(dir)), Name: "multi"}},
		},
	}); err != nil {
		t.Fatalf("DidChangeWorkspaceFolders err = %v", err)
	}
	if s.ws.Document(inner) == nil {
		t.Errorf("inner.sysml unindexed with the outer folder, though its own folder remains")
	}
	if s.ws.Document(lib) != nil {
		t.Errorf("lib.sysml still indexed after the only folder holding it was removed")
	}
}

func TestLoadFromDiskSkipsOversizedFile(t *testing.T) {
	s, _, dir, _, _ := multiFileWorkspace(t)

	// The scan's size cap has to hold on the watched-file path too, or the first
	// change event pulls a file the startup walk refused to read.
	huge := filepath.Join(dir, "huge.sysml")
	body := append([]byte(libSource), make([]byte, maxScannedFileSize)...)
	if err := os.WriteFile(huge, body, 0o600); err != nil {
		t.Fatalf("write huge: %v", err)
	}
	if err := s.DidChangeWatchedFiles(context.Background(), &protocol.DidChangeWatchedFilesParams{
		Changes: []*protocol.FileEvent{{URI: uri.File(huge), Type: protocol.FileChangeTypeCreated}},
	}); err != nil {
		t.Fatalf("DidChangeWatchedFiles err = %v", err)
	}
	if s.ws.Document(huge) != nil {
		t.Errorf("huge.sysml indexed though it exceeds the scan's size cap")
	}
}

func TestLoadFromDiskKeepsIndexOnUnreadableFile(t *testing.T) {
	s, _, _, lib, _ := multiFileWorkspace(t)

	// A read that fails for any reason other than the file being gone — a lock
	// or a momentary permission problem — must not unindex what it declares.
	if err := os.Chmod(lib, 0o000); err != nil {
		t.Fatalf("chmod lib: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(lib, 0o600) })
	if _, err := os.ReadFile(lib); err == nil {
		t.Skip("file still readable; test needs an unreadable file")
	}
	s.loadFromDisk(lib)
	if s.ws.Document(lib) == nil {
		t.Errorf("lib.sysml unindexed by a transient read failure")
	}
}

func TestWatchedFileDeleteClearsDiagnostics(t *testing.T) {
	s, fc, _, lib, _ := multiFileWorkspace(t)
	ctx := context.Background()

	// Diagnostics reach the client for lib.sysml, and then the file disappears.
	openFile(t, s, lib, "package Lib {\n    part b : Nope;\n}\n")
	if msgs := diagnosticsFor(fc, lib); len(msgs) == 0 {
		t.Fatalf("diagnostics for lib = none, want an unresolved Nope")
	}
	if err := s.DidClose(ctx, &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(lib)},
	}); err != nil {
		t.Fatalf("DidClose err = %v", err)
	}
	if err := os.Remove(lib); err != nil {
		t.Fatalf("remove lib: %v", err)
	}
	if err := s.DidChangeWatchedFiles(ctx, &protocol.DidChangeWatchedFilesParams{
		Changes: []*protocol.FileEvent{{URI: uri.File(lib), Type: protocol.FileChangeTypeDeleted}},
	}); err != nil {
		t.Fatalf("DidChangeWatchedFiles err = %v", err)
	}
	msgs, published := lastDiagnostics(fc, lib)
	if !published || len(msgs) != 0 {
		t.Fatalf("diagnostics for the deleted lib = %v (published=%v), want an empty set", msgs, published)
	}
}

func TestDidChangeRefreshesOtherOpenDocuments(t *testing.T) {
	s, fc, _, lib, main := multiFileWorkspace(t)
	ctx := context.Background()
	openFile(t, s, main, mainSource)
	openFile(t, s, lib, libSource)

	// An unsaved edit to lib.sysml renames what main.sysml uses; main's markers
	// must follow the edit burst, not wait for a save.
	if err := s.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri.File(lib)},
			Version:                2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			{Text: "package Lib {\n    part def Gadget;\n}\n"},
		},
	}); err != nil {
		t.Fatalf("DidChange err = %v", err)
	}
	waitForDiagnostics(t, fc, main, func(msgs []string) bool { return len(msgs) > 0 })
}

func TestDidChangeCoalescesOtherOpenDocuments(t *testing.T) {
	s, fc, _, lib, main := multiFileWorkspace(t)
	ctx := context.Background()
	openFile(t, s, main, mainSource)
	openFile(t, s, lib, libSource)

	// Typing is a burst of changes; re-analyzing main.sysml on each one would
	// serialize behind the workspace lock, so the sweep runs once at the end.
	for version := 2; version < 12; version++ {
		if err := s.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
			TextDocument: protocol.VersionedTextDocumentIdentifier{
				TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri.File(lib)},
				Version:                int32(version),
			},
			ContentChanges: []protocol.TextDocumentContentChangeEvent{
				{Text: "package Lib {\n    part def Gadget;\n}\n"},
			},
		}); err != nil {
			t.Fatalf("DidChange err = %v", err)
		}
	}
	waitForDiagnostics(t, fc, main, func(msgs []string) bool { return len(msgs) > 0 })
	if got := publishCount(fc, main); got > 5 {
		t.Errorf("published %d times for main across 10 edits, want the burst coalesced", got)
	}
}

// publishCount reports how many times diagnostics were published for name.
func publishCount(fc *fakeClient, name string) int {
	n := 0
	for _, p := range fc.all() {
		if p.URI == uri.File(name) {
			n++
		}
	}
	return n
}

func TestInitializeFoldersFallsBackToRoot(t *testing.T) {
	dir := t.TempDir()
	got := initializeFolders(&protocol.InitializeParams{RootURI: uri.File(dir)})
	if len(got) != 1 || got[0] != uri.File(dir).Filename() {
		t.Errorf("folders = %v, want [%s]", got, uri.File(dir).Filename())
	}
	got = initializeFolders(&protocol.InitializeParams{RootPath: dir})
	if len(got) != 1 || got[0] != dir {
		t.Errorf("folders = %v, want [%s]", got, dir)
	}
}

// A document opened under none of the session's folders has its own directory
// indexed, so the sibling files it imports resolve as inside a folder.
func TestOpeningFileOutsideFoldersIndexesItsDirectory(t *testing.T) {
	dir := t.TempDir()
	lib := filepath.Join(dir, "parts", "lib.sysml")
	main := filepath.Join(dir, "main.sysml")
	if err := os.MkdirAll(filepath.Dir(lib), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lib, []byte(libSource), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(main, []byte(mainSource), 0o600); err != nil {
		t.Fatal(err)
	}

	s := NewServer(model.NewWorkspace())
	fc := &fakeClient{}
	s.client = fc
	ctx := context.Background()
	if _, err := s.Initialize(ctx, &protocol.InitializeParams{}); err != nil {
		t.Fatalf("Initialize err = %v", err)
	}
	if err := s.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		t.Fatalf("Initialized err = %v", err)
	}

	openFile(t, s, main, mainSource)
	if msgs := diagnosticsFor(fc, main); len(msgs) != 0 {
		t.Fatalf("diagnostics for main = %v, want none", msgs)
	}
	if s.ws.Document(lib) == nil {
		t.Errorf("lib.sysml not indexed when its sibling was opened")
	}
	if s.ws.IsOpen(lib) {
		t.Errorf("lib.sysml reported open; the scan records on-disk content only")
	}
}

// The directory scan an open document triggers never overwrites another open
// buffer, whose text the editor owns.
func TestOpeningFileOutsideFoldersKeepsOpenBuffers(t *testing.T) {
	dir := t.TempDir()
	lib := filepath.Join(dir, "lib.sysml")
	main := filepath.Join(dir, "main.sysml")
	if err := os.WriteFile(lib, []byte("package Lib {\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(main, []byte(mainSource), 0o600); err != nil {
		t.Fatal(err)
	}

	s := NewServer(model.NewWorkspace())
	fc := &fakeClient{}
	s.client = fc
	if _, err := s.Initialize(context.Background(), &protocol.InitializeParams{}); err != nil {
		t.Fatalf("Initialize err = %v", err)
	}

	// The editor holds Widget in an unsaved buffer; the disk copy lacks it.
	openFile(t, s, lib, libSource)
	openFile(t, s, main, mainSource)
	if msgs := diagnosticsFor(fc, main); len(msgs) != 0 {
		t.Fatalf("diagnostics for main = %v, want none", msgs)
	}
	if !s.ws.IsOpen(lib) {
		t.Errorf("lib.sysml no longer open after the sibling scan")
	}
}
