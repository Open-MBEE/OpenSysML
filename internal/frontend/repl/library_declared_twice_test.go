package repl

import (
	"path/filepath"
	"strings"
	"testing"

	checkws "github.com/Open-MBEE/OpenSysML/internal/check/workspace"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// Two loaded files declaring one library package: the prompt reports the
// duplication once, in the file references do not resolve to, as a warning in
// every conformance mode — it is about the workspace, not the notation.
func TestLoadingALibraryDeclaredTwiceWarnsOnce(t *testing.T) {
	for _, strict := range []bool{false, true} {
		dir := t.TempDir()
		a := writeFile(t, filepath.Join(dir, "a.sysml"), "library package Foo {\n    part def A;\n}\n")
		b := writeFile(t, filepath.Join(dir, "b.sysml"), "library package Foo {\n    part def A;\n}\n")
		s := NewSession()
		if strict {
			meta(t, s, "%strict on")
		}
		if _, err := s.LoadFile(a); err != nil {
			t.Fatal(err)
		}
		if _, err := s.LoadFile(b); err != nil {
			t.Fatal(err)
		}
		var found []Diagnostic
		for _, d := range s.LocatedDiagnostics() {
			if d.Code == checkws.CodeLibraryDeclaredTwice {
				found = append(found, d)
			}
		}
		if len(found) != 1 {
			t.Fatalf("strict=%v: %d duplicate-library findings, want 1: %+v", strict, len(found), found)
		}
		d := found[0]
		if d.File != b || d.Line != 1 || d.Column != 17 || d.Severity != diag.SeverityWarning.String() || d.Pass != checkws.Source {
			t.Errorf("strict=%v: finding = %+v, want a workspace warning on %s:1:17", strict, d, b)
		}
		if !strings.Contains(d.Message, "both here and in "+a) {
			t.Errorf("strict=%v: message = %q, want it to name %s", strict, d.Message, a)
		}
		lines := strings.Join(s.DiagnosticLines(), "\n")
		if strings.Count(lines, "warning[library-declared-twice]") != 1 || !strings.Contains(lines, b+":1:17:") {
			t.Errorf("strict=%v: diagnostic lines report it other than once at %s:1:17:\n%s", strict, b, lines)
		}
		if s.HasErrors() {
			t.Errorf("strict=%v: a duplicated library is a warning, not an error", strict)
		}
	}
}

// One library declared once is quiet, however many files refer to it.
func TestLoadingALibraryDeclaredOnceIsQuiet(t *testing.T) {
	dir := t.TempDir()
	lib := writeFile(t, filepath.Join(dir, "lib.sysml"), "library package Foo {\n    part def A;\n}\n")
	m := writeFile(t, filepath.Join(dir, "m.sysml"), "package M {\n    part x : Foo::A;\n}\n")
	s := NewSession()
	for _, path := range []string{lib, m} {
		if _, err := s.LoadFile(path); err != nil {
			t.Fatal(err)
		}
	}
	if lines := strings.Join(s.DiagnosticLines(), "\n"); strings.Contains(lines, "library-declared-twice") {
		t.Errorf("a library declared once was reported:\n%s", lines)
	}
}
