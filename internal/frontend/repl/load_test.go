package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// The declarations a file references may live in a file loaded after it: every
// file of a load is accepted before the buffer is analyzed.
func TestSubmitAllIsOrderIndependent(t *testing.T) {
	a := "package A { part def Wheel; }\n"
	b := "package B { private import A::*; part w : Wheel; }\n"

	for _, order := range [][]string{{b, a}, {a, b}} {
		s := NewSession()
		res := s.SubmitAll(order)
		if len(res.Diagnostics) != 0 {
			t.Fatalf("B loaded first should still resolve A, got %v", res.Diagnostics)
		}
		if len(res.Declared) != 2 {
			t.Fatalf("want both packages declared, got %v", res.Declared)
		}
	}
}

// The same models loaded one submission at a time only resolve in dependency
// order, which is what loading a project as one submission avoids.
func TestLoadingADirectoryResolvesRegardlessOfFileName(t *testing.T) {
	dir := t.TempDir()
	// Sorted order puts the referencing file first.
	writeFile(t, filepath.Join(dir, "a-uses.sysml"), "package Uses { private import Defs::*; part w : Wheel; }\n")
	writeFile(t, filepath.Join(dir, "b-defs.sysml"), "package Defs { part def Wheel; }\n")

	s := NewSession()
	out, err := s.LoadPaths([]string{dir})
	if err != nil {
		t.Fatalf("LoadPaths: %v", err)
	}
	joined := strings.Join(out, "\n")
	if strings.Contains(joined, "error") || strings.Contains(joined, "unresolved") {
		t.Fatalf("directory load reported problems:\n%s", joined)
	}
	if !strings.Contains(joined, "loaded 2 files") {
		t.Fatalf("load did not report both files:\n%s", joined)
	}
	if got := s.List(); len(got) != 2 {
		t.Fatalf("want 2 snippets in the session, got %d: %v", len(got), got)
	}
}

func TestMetaLoadAcceptsADirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.sysml"), "package A { }\n")
	writeFile(t, filepath.Join(dir, "nested", "b.sysml"), "package B { }\n")

	s := NewSession()
	out, _, err := s.RunMeta("%load " + dir)
	if err != nil {
		t.Fatalf("%%load <dir>: %v", err)
	}
	if !strings.Contains(strings.Join(out, "\n"), "loaded 2 files") {
		t.Fatalf("unexpected output: %v", out)
	}
	if got := s.List(); len(got) != 2 {
		t.Fatalf("want 2 snippets, got %v", got)
	}
}

func TestMetaLoadAcceptsAGlob(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.sysml"), "package A { }\n")
	writeFile(t, filepath.Join(dir, "b.sysml"), "package B { }\n")
	writeFile(t, filepath.Join(dir, "notes.md"), "not a model\n")

	s := NewSession()
	if _, _, err := s.RunMeta("%load " + filepath.Join(dir, "*.sysml")); err != nil {
		t.Fatalf("%%load <glob>: %v", err)
	}
	if got := s.List(); len(got) != 2 {
		t.Fatalf("want 2 snippets, got %v", got)
	}
}

func TestMetaLoadAcceptsSeveralPaths(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, filepath.Join(dir, "a.sysml"), "package A { }\n")
	sub := filepath.Join(dir, "sub")
	writeFile(t, filepath.Join(sub, "b.sysml"), "package B { }\n")

	s := NewSession()
	if _, _, err := s.RunMeta("%load " + a + " " + sub); err != nil {
		t.Fatalf("%%load <file> <dir>: %v", err)
	}
	if got := s.List(); len(got) != 2 {
		t.Fatalf("want 2 snippets, got %v", got)
	}
}

func TestMetaLoadReportsADirectoryWithNoModels(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "README.md"), "nothing to load\n")

	s := NewSession()
	_, _, err := s.RunMeta("%load " + dir)
	if err == nil || !strings.Contains(err.Error(), "no .sysml or .kerml files") {
		t.Fatalf("want a no-model-files error, got %v", err)
	}
}

func TestMetaLoadReportsAMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.sysml")
	s := NewSession()
	_, _, err := s.RunMeta("%load " + missing)
	if err == nil || !strings.Contains(err.Error(), "nope.sysml") {
		t.Fatalf("want an error naming the missing file, got %v", err)
	}
}

func TestMetaLoadWithoutArgumentsShowsUsage(t *testing.T) {
	s := NewSession()
	out, _, err := s.RunMeta("%load")
	if err != nil {
		t.Fatalf("%%load: %v", err)
	}
	if len(out) != 1 || !strings.HasPrefix(out[0], "usage: %load") {
		t.Fatalf("unexpected output: %v", out)
	}
}

// Two files of one load are both part of the model: name-based replacement is
// for redeclaring at the prompt, so it must not drop a file of the same load.
func TestFilesOfOneLoadDoNotReplaceEachOther(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.sysml"), "package Defs { part def Wheel; }\n")
	writeFile(t, filepath.Join(dir, "b.sysml"), "package Defs { part def Axle; }\n")

	s := NewSession()
	if _, err := s.LoadPaths([]string{dir}); err != nil {
		t.Fatalf("LoadPaths: %v", err)
	}
	got := s.List()
	if len(got) != 2 {
		t.Fatalf("a file of the load was dropped, want 2 snippets, got %v", got)
	}
	joined := strings.Join(got, "\n")
	if !strings.Contains(joined, "Wheel") || !strings.Contains(joined, "Axle") {
		t.Fatalf("both declarations should survive the load: %q", joined)
	}
}

// Retyping a declaration still supersedes what an earlier submission loaded.
func TestASubmissionStillReplacesAnEarlierLoad(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.sysml"), "package P { part def Wheel; }\n")

	s := NewSession()
	if _, err := s.LoadPaths([]string{dir}); err != nil {
		t.Fatalf("LoadPaths: %v", err)
	}
	s.Submit("package P { part def Axle; }")
	got := s.List()
	if len(got) != 1 {
		t.Fatalf("want the loaded package replaced, got %v", got)
	}
	if strings.Contains(got[0], "Wheel") {
		t.Fatalf("stale declaration survived: %q", got[0])
	}
}

// A diagnostic in the second file of a load is reported against that file, at
// the line it has in it — not at its line in the accumulated buffer.
func TestDiagnosticsOfALoadNameTheirFileAndItsLines(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.sysml"), "package A { }\n\n\n\npackage A2 { }\n")
	bad := writeFile(t, filepath.Join(dir, "b.sysml"), "package B {\n  import Missing::X;\n}\n")

	s := NewSession()
	out, err := s.LoadPaths([]string{dir})
	if err != nil {
		t.Fatalf("LoadPaths: %v", err)
	}
	joined := strings.Join(out, "\n")
	if !strings.Contains(joined, bad+":2:") {
		t.Fatalf("want a diagnostic reported at %s:2, got:\n%s", bad, joined)
	}
}

// A submission typed at the prompt comes from no file, and is still numbered
// from the line the user typed.
func TestPromptDiagnosticsAreNotAttributedToAFile(t *testing.T) {
	s := NewSession()
	s.Submit("package First { }")
	r := s.Submit("namespace N { import Missing::X; }")
	out := renderResult(r, VerbosityNormal)
	joined := strings.Join(out, "\n")
	if !strings.Contains(joined, "1:") {
		t.Fatalf("want the diagnostic on line 1 of the submission, got:\n%s", joined)
	}
	if strings.Contains(joined, ".sysml:") {
		t.Fatalf("prompt diagnostic should name no file:\n%s", joined)
	}
}

// A comment-only file is a file of the load, not documentation for the next one:
// it keeps its own snippet, and a diagnostic in a later file is still that
// file's.
func TestACommentOnlyFileKeepsItsIdentity(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a-notes.sysml"), "// notes about this project\n")
	decl := writeFile(t, filepath.Join(dir, "b-defs.sysml"), "package P {\n  import Missing::X;\n}\n")

	s := NewSession()
	out, err := s.LoadPaths([]string{dir})
	if err != nil {
		t.Fatalf("LoadPaths: %v", err)
	}
	if got := s.List(); len(got) != 2 {
		t.Fatalf("the comment file was folded away, want 2 snippets, got %v", got)
	}
	if joined := strings.Join(out, "\n"); !strings.Contains(joined, decl+":2:") {
		t.Fatalf("want a diagnostic reported at %s:2, got:\n%s", decl, joined)
	}
	// Retyping the declaration must not take the comment file with it.
	s.Submit("package P { }")
	if got := s.List(); len(got) != 2 || !strings.Contains(got[0], "notes about this project") {
		t.Fatalf("the comment file was deleted with the redeclaration: %v", got)
	}
}
