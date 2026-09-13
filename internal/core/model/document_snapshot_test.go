package model

import (
	"bytes"
	"sync"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
)

// sameSlice reports whether a and b are the same slice value: same length over
// the same backing array (two empty slices count as the same).
func sameSlice[T any](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	return len(a) == 0 || &a[0] == &b[0]
}

// A document handed out by the workspace is a snapshot: updating, analyzing and
// closing the document afterwards installs new state and leaves the snapshot's
// every field the value newDocument built.
func TestDocumentSnapshotSurvivesUpdate(t *testing.T) {
	ws := NewWorkspace()
	const name = "snap.sysml"
	first := []byte("package P { import ISQ::*; part def A; } part")
	ws.SetOnDisk(name, []byte("package D;"))
	ws.Open(name, first, 1)

	snap := ws.Document(name)
	if snap == nil {
		t.Fatal("Document = nil after Open")
	}
	if len(snap.ParseDiagnostics) == 0 || len(snap.ParseWarnings) == 0 {
		t.Fatalf("fixture yields %d parse diagnostics and %d warnings, want both non-empty so the slices are checked",
			len(snap.ParseDiagnostics), len(snap.ParseWarnings))
	}
	content, root, scope := snap.Content, snap.AST, snap.Scope
	diags, warns := snap.ParseDiagnostics, snap.ParseWarnings
	diag0, warn0 := diags[0], warns[0]

	ws.Diagnostics(name)
	ws.Update(name, []byte("package Q { part def B; }"), 2)
	ws.Diagnostics(name)
	ws.Close(name)

	if cur := ws.Document(name); cur == nil || cur == snap || cur.Version != 0 {
		t.Fatalf("Document after Close = %+v, want a fresh on-disk document", cur)
	}
	if snap.Name != name || snap.Version != 1 {
		t.Errorf("snapshot Name, Version = %q, %d; want %q, 1", snap.Name, snap.Version, name)
	}
	if !sameSlice(snap.Content, content) || !bytes.Equal(snap.Content, first) {
		t.Errorf("snapshot Content = %q, want the bytes it was opened with", snap.Content)
	}
	if snap.AST != root || snap.Scope != scope {
		t.Error("snapshot AST or Scope replaced")
	}
	if len(snap.AST.Members) != 2 {
		t.Errorf("snapshot AST has %d members, want 2", len(snap.AST.Members))
	}
	if _, ok := snap.Scope.LookupLocal("P"); !ok {
		t.Error("snapshot Scope lost P")
	}
	if _, ok := snap.Scope.LookupLocal("Q"); ok {
		t.Error("snapshot Scope gained Q from the update")
	}
	if !sameSlice(snap.ParseDiagnostics, diags) || !sameDiagnostic(snap.ParseDiagnostics[0], diag0) {
		t.Error("snapshot ParseDiagnostics changed")
	}
	if !sameSlice(snap.ParseWarnings, warns) || !sameDiagnostic(snap.ParseWarnings[0], warn0) {
		t.Error("snapshot ParseWarnings changed")
	}
}

// sameDiagnostic reports whether a and b carry the same span, message and code.
func sameDiagnostic(a, b parser.Diagnostic) bool {
	return a.Span == b.Span && a.Message == b.Message && a.Code == b.Code
}

// The workspace owns the bytes it is given: the buffer a caller passed to Open,
// Update or SetOnDisk can be overwritten afterwards (clobber) without the
// document built from it, or the source its AST spans index into, changing.
func TestDocumentSnapshotOwnsContent(t *testing.T) {
	const want = "package P { part def A; }"
	old := []byte("package Old;")
	for _, tc := range []struct {
		name string
		run  func(ws *Workspace, name string, buf []byte, clobber func())
	}{
		{"Open", func(ws *Workspace, name string, buf []byte, clobber func()) {
			ws.Open(name, buf, 1)
			clobber()
		}},
		{"Update", func(ws *Workspace, name string, buf []byte, clobber func()) {
			ws.Open(name, old, 1)
			ws.Update(name, buf, 2)
			clobber()
		}},
		{"SetOnDisk", func(ws *Workspace, name string, buf []byte, clobber func()) {
			ws.SetOnDisk(name, buf)
			clobber()
		}},
		{"SetOnDiskWhileOpenThenClose", func(ws *Workspace, name string, buf []byte, clobber func()) {
			ws.Open(name, old, 1)
			ws.SetOnDisk(name, buf)
			clobber()
			ws.Close(name)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ws := NewWorkspace()
			const name = "own.sysml"
			buf := []byte(want)
			tc.run(ws, name, buf, func() { copy(buf, "package Q { part def B; }") })
			snap := ws.Document(name)
			if snap == nil {
				t.Fatal("Document = nil")
			}
			if sameSlice(snap.Content, buf) || sameSlice(snap.sf.Bytes(), buf) {
				t.Fatal("document aliases the caller's buffer")
			}
			if got := string(snap.Content); got != want {
				t.Errorf("Content = %q, want %q", got, want)
			}
			if got := string(snap.sf.Bytes()); got != want {
				t.Errorf("source bytes = %q, want %q", got, want)
			}
			if got := snap.sf.Text(snap.AST.Members[0].Span()); got != want {
				t.Errorf("text of the package member = %q, want %q", got, want)
			}
			if _, ok := snap.Scope.LookupLocal("P"); !ok {
				t.Error("Scope lost P")
			}
		})
	}
}

// Reading a snapshot, or the current document, races with nothing a reindex
// does, and every document read is one revision throughout: content, AST and
// scope never mix. Run with -race.
func TestDocumentSnapshotReadsConcurrentWithReindex(t *testing.T) {
	ws := NewWorkspace()
	const name = "flip.sysml"
	revisions := [][]byte{[]byte("package P { part def Alpha; }\n"), []byte("package Q { part def Bravo; }\n")}
	packages := []string{"P", "Q"}
	ws.Open(name, revisions[0], 1)
	snap := ws.Document(name)

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for version := 2; ; version++ {
			select {
			case <-stop:
				return
			default:
			}
			ws.Update(name, revisions[(version-1)%2], version)
			ws.Diagnostics(name)
		}
	}()
	defer func() { close(stop); <-done }()

	check := func(doc *Document) {
		i := (doc.Version - 1) % 2
		if !bytes.Equal(doc.Content, revisions[i]) {
			t.Errorf("version %d has content %q, want %q", doc.Version, doc.Content, revisions[i])
		}
		if len(doc.AST.Members) != 1 {
			t.Errorf("version %d has %d members, want 1", doc.Version, len(doc.AST.Members))
		}
		if _, ok := doc.Scope.LookupLocal(packages[i]); !ok {
			t.Errorf("version %d has no %s in scope", doc.Version, packages[i])
		}
		if _, ok := doc.Scope.LookupLocal(packages[1-i]); ok {
			t.Errorf("version %d has %s in scope", doc.Version, packages[1-i])
		}
		if len(doc.ParseDiagnostics) != 0 || len(doc.ParseWarnings) != 0 {
			t.Errorf("version %d has %d diagnostics and %d warnings, want none", doc.Version,
				len(doc.ParseDiagnostics), len(doc.ParseWarnings))
		}
	}

	const readers = 4
	var wg sync.WaitGroup
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				check(snap)
				if doc := ws.Document(name); doc == nil {
					t.Error("Document = nil while the document is open")
				} else {
					check(doc)
				}
			}
		}()
	}
	wg.Wait()
	if snap.Version != 1 {
		t.Errorf("snapshot Version = %d, want 1", snap.Version)
	}
}
