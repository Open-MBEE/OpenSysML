package workspace_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	checkws "github.com/Open-MBEE/OpenSysML/internal/check/workspace"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

// The fixture pair under tests/testdata/workspace/library_declared_twice: a
// library package and a vendored copy of it beside a model using both. The copy
// nearer the root is the one references resolve to; the vendored one is reported.
func TestDuplicateLibrariesFixturePair(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "..", "tests", "testdata", "workspace", "library_declared_twice"))
	if err != nil {
		t.Fatal(err)
	}
	ws := model.NewWorkspace()
	var names []string
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".sysml" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		ws.Open(path, data, 1)
		names = append(names, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 3 {
		t.Fatalf("fixture holds %d files, want 3: %v", len(names), names)
	}
	got := ws.WorkspaceFindings()
	if len(got) != 1 {
		t.Fatalf("%d findings, want 1: %+v", len(got), got)
	}
	f := got[0]
	first, second := filepath.Join(dir, "shapes.sysml"), filepath.Join(dir, "vendor", "shapes.sysml")
	if f.Document != second || f.Used != first || f.Library {
		t.Errorf("finding on %q (used %q), want the vendored copy with %s used", f.Document, f.Used, first)
	}
	if want := "library package Shapes is declared both here and in " + first + "; references resolve to the declaration in " + first + ", and this one is unused"; f.Diagnostic.Message != want || f.Diagnostic.Code != checkws.CodeLibraryDeclaredTwice {
		t.Errorf("finding = [%s] %q,\nwant        [%s] %q", f.Diagnostic.Code, f.Diagnostic.Message, checkws.CodeLibraryDeclaredTwice, want)
	}
	// Shapes::Sphere is declared by the unused copy alone, so the model's
	// reference to it fails: the warning is what explains the unresolved name.
	var unresolved []string
	for _, d := range ws.Diagnostics(filepath.Join(dir, "model.sysml")) {
		if d.Code == "unresolved" {
			unresolved = append(unresolved, d.Message)
		}
	}
	if len(unresolved) != 1 || !strings.Contains(unresolved[0], "Shapes::Sphere") {
		t.Errorf("model.sysml unresolved = %v, want Shapes::Sphere alone", unresolved)
	}
}
