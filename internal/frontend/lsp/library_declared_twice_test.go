package lsp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	checkws "github.com/Open-MBEE/OpenSysML/internal/check/workspace"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// findings keeps the published diagnostics carrying the duplicate-library code, by URI.
func libraryDeclaredTwice(published []*protocol.PublishDiagnosticsParams) map[uri.URI][]protocol.Diagnostic {
	out := map[uri.URI][]protocol.Diagnostic{}
	for _, p := range published {
		for _, d := range p.Diagnostics {
			if diagnosticCode(d) == checkws.CodeLibraryDeclaredTwice {
				out[p.URI] = append(out[p.URI], d)
			}
		}
	}
	return out
}

// Two open documents declaring one library package: the warning is published
// once, on the copy references do not resolve to, and names the other.
func TestPublishDiagnosticsReportsALibraryDeclaredTwiceOnce(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	fc := &fakeClient{}
	s.client = fc
	ctx := context.Background()
	first, second := "/ws/lib/Shapes.sysml", "/ws/vendor/Shapes.sysml"
	ws.Open(first, []byte("library package Shapes { part def Box; }"), 1)
	ws.Open(second, []byte("library package Shapes { part def Box; }"), 1)
	s.publishDiagnostics(ctx, first)
	s.publishDiagnostics(ctx, second)
	s.refreshOpenDiagnostics(ctx, "")

	got := libraryDeclaredTwice(fc.all())
	if len(got) != 1 || len(got[uri.File(second)]) == 0 {
		t.Fatalf("library-declared-twice published on %v, want on %s alone", got, second)
	}
	for _, p := range fc.all() {
		if p.URI == uri.File(second) && len(libraryDeclaredTwice([]*protocol.PublishDiagnosticsParams{p})[p.URI]) > 1 {
			t.Errorf("one publication carries the warning more than once: %v", p.Diagnostics)
		}
	}
	d := got[uri.File(second)][0]
	if d.Severity != protocol.DiagnosticSeverityWarning || optString(d.Source) != checkws.Source {
		t.Errorf("severity %v source %q, want a warning from %s", d.Severity, optString(d.Source), checkws.Source)
	}
	if !strings.Contains(diagnosticMessage(t, d), first) || !strings.Contains(diagnosticMessage(t, d), "references resolve to the declaration in "+first) {
		t.Errorf("message = %q, want it to name %s as the copy in use", diagnosticMessage(t, d), first)
	}
	if d.Range.Start.Character != uint32(len("library package ")) || d.Range.End.Character != uint32(len("library package Shapes")) {
		t.Errorf("range = %v, want the package name", d.Range)
	}
	if cd := d.CodeDescription; cd.Href == "" || !strings.HasSuffix(string(cd.Href), "#"+checkws.CodeLibraryDeclaredTwice) {
		t.Errorf("codeDescription = %v, want the catalogue section of the code", cd)
	}
}

// A copy the folder scan indexed but no tab has open still reaches the problems
// list, under its own URI, and is withdrawn once the duplication is gone.
func TestPublishFindingsReachesADocumentNotOpenAndWithdraws(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "lib", "Shapes.sysml")
	second := filepath.Join(dir, "vendor", "Shapes.sysml")
	for _, path := range []string{first, second} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("library package Shapes { part def Box; }"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ws := model.NewWorkspace()
	s := NewServer(ws)
	fc := &fakeClient{}
	s.client = fc
	ctx := context.Background()
	s.loadFilesFromDisk([]string{first, second})
	s.refreshOpenDiagnostics(ctx, "")

	got := libraryDeclaredTwice(fc.all())
	if len(got) != 1 || len(got[uri.File(second)]) != 1 {
		t.Fatalf("library-declared-twice published on %v, want once on %s", got, second)
	}
	if !strings.Contains(diagnosticMessage(t, got[uri.File(second)][0]), first) {
		t.Errorf("message = %q, want it to name %s", diagnosticMessage(t, got[uri.File(second)][0]), first)
	}

	ws.Remove(second)
	s.refreshOpenDiagnostics(ctx, "")
	last := fc.all()[len(fc.all())-1]
	if last.URI != uri.File(second) || len(last.Diagnostics) != 0 {
		t.Fatalf("after the copy is gone the last publication is %v, want an empty set on %s", last, second)
	}
}

// A closed document shadowed by two different trees gets both warnings in one
// publication: a later publication for the same URI would replace the first.
func TestPublishFindingsSendsAClosedDocumentsFindingsTogether(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		filepath.Join(dir, "a", "Alpha.sysml"): "library package Alpha { part def A; }",
		filepath.Join(dir, "b", "Beta.sysml"):  "library package Beta { part def B; }",
		filepath.Join(dir, "c", "Both.sysml"):  "library package Alpha { part def A; }\nlibrary package Beta { part def B; }",
	}
	var paths []string
	for path, text := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	ws := model.NewWorkspace()
	s := NewServer(ws)
	fc := &fakeClient{}
	s.client = fc
	s.loadFilesFromDisk(paths)
	s.refreshOpenDiagnostics(context.Background(), "")

	both := uri.File(filepath.Join(dir, "c", "Both.sysml"))
	var publications int
	for _, p := range fc.all() {
		if p.URI != both {
			continue
		}
		publications++
		if n := len(libraryDeclaredTwice([]*protocol.PublishDiagnosticsParams{p})[both]); n != 2 {
			t.Errorf("publication on Both.sysml carries %d duplicate-library warnings, want 2: %v", n, p.Diagnostics)
		}
	}
	if publications != 1 {
		t.Errorf("Both.sysml was published %d times, want once", publications)
	}
}
