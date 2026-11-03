package workspace_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	checkws "github.com/Open-MBEE/OpenSysML/internal/check/workspace"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// A library path holding two copies of the standard library is one warning,
// on the first file of the second copy, naming both trees relative to the
// variable so the reader can find them.
func TestDuplicateLibrariesReportsALibraryPathHoldingTwoCopies(t *testing.T) {
	bundled := model.NewWorkspace()
	names := bundled.LibraryNames()
	if len(names) == 0 {
		t.Fatal("no bundled library files")
	}
	dir := t.TempDir()
	for _, tree := range []string{"one", "two"} {
		for _, name := range names {
			doc := bundled.LibraryDocument(name)
			if doc == nil {
				t.Fatalf("%s not bundled", name)
			}
			path := filepath.Join(dir, tree, "sysml.library", filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, doc.Content, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Setenv(libs.LibraryPathEnvVar, dir)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	ws := model.NewWorkspace()
	ws.Open("/ws/m.sysml", []byte("package M { attribute r : ScalarValues::Real; }"), 1)
	if diags := ws.Diagnostics("/ws/m.sysml"); len(diags) != 0 {
		t.Fatalf("the model does not analyse clean against two copies: %v", diags)
	}
	got := ws.WorkspaceFindings()
	if len(got) != 1 {
		t.Fatalf("%d findings, want 1: %+v", len(got), got)
	}
	f := got[0]
	if !f.Library || !strings.HasPrefix(f.Document, "two/sysml.library/") || !strings.HasPrefix(f.Used, "one/sysml.library/") {
		t.Errorf("finding on %q (used %q, library %v), want a library file of two/ with one/ used", f.Document, f.Used, f.Library)
	}
	if len(f.Packages) != len(names) {
		t.Errorf("%d packages duplicated, want one per library file (%d)", len(f.Packages), len(names))
	}
	msg := f.Diagnostic.Message
	for _, want := range []string{
		"library packages (",
		"are declared both under two/sysml.library and under one/sysml.library; references resolve to the declarations under one/sysml.library, and the ones under two/sysml.library are unused",
		"; the paths are relative to " + libs.LibraryPathEnvVar + ", " + dir,
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message = %q,\nwant it to contain %q", msg, want)
		}
	}
	if f.Diagnostic.Code != checkws.CodeLibraryDeclaredTwice {
		t.Errorf("code = %q", f.Diagnostic.Code)
	}
	if ws.FindingsOn("/ws/m.sysml") != nil {
		t.Error("the finding is on the library file, not on the model")
	}
}
