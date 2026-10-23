package jupyter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl"
)

// workspaceEngine is an engine over a session rooted at a directory holding
// lib.sysml, as the kernel is rooted at the notebook's directory.
func workspaceEngine(t *testing.T) (*REPLEngine, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lib.sysml"), []byte("package Lib { attribute a = 3; calc def Twice { in x; 2 * x } }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sess := repl.NewSession()
	if err := sess.SetWorkspaceRoot(dir); err != nil {
		t.Fatal(err)
	}
	return NewREPLEngine(sess), dir
}

func TestACellImportingAPackageAFileBesideTheNotebookDeclaresLoadsIt(t *testing.T) {
	e, _ := workspaceEngine(t)
	out := mustRun(t, e, "private import Lib::*;\na")
	for _, want := range []string{"✓ import Lib::*", "✓ package Lib", "note: loaded lib.sysml, which declares Lib"} {
		if !strings.Contains(out.text(), want) {
			t.Errorf("want %q in the cell's output:\n%s", want, out.text())
		}
	}
	if len(out.results) != 1 || !strings.Contains(out.results[0][mimeText].(string), "= 3") {
		t.Errorf("a = %v", out.results)
	}
	out = mustRun(t, e, "%calc Lib::Twice(4)")
	if !strings.Contains(out.text(), "= [8]") {
		t.Errorf("%%calc output = %q", out.text())
	}
}

func TestACellImportingANameNoFileDeclaresFails(t *testing.T) {
	e, _ := workspaceEngine(t)
	failure := mustFail(t, e, "private import Nope::*;")
	if !strings.Contains(failure.Value, "unresolved reference: Nope") {
		t.Errorf("failure = %+v", failure)
	}
	if got := e.Session().Files(); len(got) != 0 {
		t.Errorf("files = %v, want nothing loaded", got)
	}
}

func TestWorkspaceShowsTheNotebookDirectoryAndClearDropsTheLoadedFile(t *testing.T) {
	e, dir := workspaceEngine(t)
	if out := mustRun(t, e, "%workspace"); !strings.Contains(out.text(), "workspace root: "+dir) {
		t.Errorf("%%workspace = %q", out.text())
	}
	mustRun(t, e, "private import Lib::*;")
	mustRun(t, e, "%clear")
	if got := e.Session().Files(); len(got) != 0 {
		t.Errorf("files after %%clear = %v", got)
	}
}
