package workspace_test

import (
	"strings"
	"testing"

	checkws "github.com/Open-MBEE/OpenSysML/internal/check/workspace"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

const scalarValues = "Kernel Libraries/Kernel Data Type Library/ScalarValues.kerml"

// The bundled library alone, a user library package declared once and a plain
// package opened by two files are not duplications.
func TestDuplicateLibrariesIsQuietWhenNothingIsDeclaredTwice(t *testing.T) {
	ws := model.NewWorkspace()
	if got := ws.WorkspaceFindings(); len(got) != 0 {
		t.Fatalf("bundled library alone: %d findings, want none: %+v", len(got), got)
	}
	ws.Open("/ws/lib.sysml", []byte("library package Vendored { part def A; }"), 1)
	ws.Open("/ws/one.sysml", []byte("package P { part def X; }"), 1)
	ws.Open("/ws/two.sysml", []byte("package P { part def Y; }"), 1)
	if got := ws.WorkspaceFindings(); len(got) != 0 {
		t.Fatalf("one user library and a reopened plain package: %d findings, want none: %+v", len(got), got)
	}
}

// Two documents declaring the same `library package` are one warning, on the
// copy references do not resolve to, naming the one they do.
func TestDuplicateLibrariesReportsAUserLibraryDeclaredTwice(t *testing.T) {
	ws := model.NewWorkspace()
	ws.Open("/ws/a.sysml", []byte("library package Foo { part def A; }"), 1)
	ws.Open("/ws/b.sysml", []byte("library package Foo { part def B; }"), 1)
	ws.Open("/ws/m.sysml", []byte("package M { part x : Foo::A; part y : Foo::B; }"), 1)
	got := ws.WorkspaceFindings()
	if len(got) != 1 {
		t.Fatalf("%d findings, want 1: %+v", len(got), got)
	}
	f := got[0]
	if f.Document != "/ws/b.sysml" || f.Used != "/ws/a.sysml" || f.Library {
		t.Errorf("finding on %q (used %q, library %v), want on /ws/b.sysml with /ws/a.sysml used", f.Document, f.Used, f.Library)
	}
	d := f.Diagnostic
	if d.Code != checkws.CodeLibraryDeclaredTwice || d.Severity != diag.SeverityWarning || d.Source != checkws.Source {
		t.Errorf("diagnostic = %+v, want a %s warning from %s", d, checkws.CodeLibraryDeclaredTwice, checkws.Source)
	}
	if want := "library package Foo is declared both here and in /ws/a.sysml; references resolve to the declaration in /ws/a.sysml, and this one is unused"; d.Message != want {
		t.Errorf("message = %q, want %q", d.Message, want)
	}
	if d.Span.Offset != len("library package ") || d.Span.Len != len("Foo") {
		t.Errorf("span = %+v, want the package name", d.Span)
	}
	// The copy the finding names as used is the one the references reach.
	var unresolved []string
	for _, d := range ws.Diagnostics("/ws/m.sysml") {
		if d.Code == "unresolved" {
			unresolved = append(unresolved, d.Message)
		}
	}
	if len(unresolved) != 1 || !strings.Contains(unresolved[0], "Foo::B") {
		t.Errorf("m.sysml unresolved = %v, want Foo::B alone (Foo::A resolves to a.sysml)", unresolved)
	}
	if ws.FindingsOn("/ws/a.sysml") != nil || len(ws.FindingsOn("/ws/b.sysml")) != 1 {
		t.Error("the finding is attached to b.sysml alone")
	}
	ws.Remove("/ws/b.sysml")
	if got := ws.WorkspaceFindings(); len(got) != 0 {
		t.Fatalf("after removing the copy: %d findings, want none", len(got))
	}
}

// Two copies of a standard library file under the workspace both stand in for
// the bundled one; that is one warning too, and one copy alone is none.
func TestDuplicateLibrariesReportsTwoCopiesOfAStandardLibraryFile(t *testing.T) {
	ws := model.NewWorkspace()
	lib := ws.LibraryDocument(scalarValues)
	if lib == nil {
		t.Fatalf("%s not bundled", scalarValues)
	}
	ws.Open("/ws/one/"+scalarValues, lib.Content, 1)
	if got := ws.WorkspaceFindings(); len(got) != 0 {
		t.Fatalf("one copy standing in for the bundled file: %d findings, want none: %+v", len(got), got)
	}
	ws.Open("/ws/two/"+scalarValues, lib.Content, 1)
	got := ws.WorkspaceFindings()
	if len(got) != 1 {
		t.Fatalf("%d findings, want 1: %+v", len(got), got)
	}
	f := got[0]
	if f.Document != "/ws/two/"+scalarValues || f.Used != "/ws/one/"+scalarValues {
		t.Errorf("finding on %q (used %q), want the second copy with the first used", f.Document, f.Used)
	}
	if !strings.Contains(f.Diagnostic.Message, "library package ScalarValues is declared both here and in /ws/one/"+scalarValues) {
		t.Errorf("message = %q", f.Diagnostic.Message)
	}
}

// Two trees each declaring many library packages are one warning naming the trees.
func TestDuplicateLibrariesGroupsATreeDeclaredTwice(t *testing.T) {
	ws := model.NewWorkspace()
	for _, tree := range []string{"/ws/vendor/lib", "/ws/lib"} {
		ws.Open(tree+"/Shapes.sysml", []byte("library package Shapes { part def Box; }"), 1)
		ws.Open(tree+"/Units.sysml", []byte("library package Units { attribute def Metre; }"), 1)
		ws.Open(tree+"/sub/Colors.sysml", []byte("library package Colors { attribute def Red; }"), 1)
		ws.Open(tree+"/sub/Fonts.sysml", []byte("library package Fonts { attribute def Serif; }"), 1)
	}
	got := ws.WorkspaceFindings()
	if len(got) != 1 {
		t.Fatalf("%d findings, want 1: %+v", len(got), got)
	}
	f := got[0]
	if f.Document != "/ws/vendor/lib/Shapes.sysml" || f.Used != "/ws/lib/Shapes.sysml" {
		t.Errorf("finding on %q (used %q), want the first file of the vendored tree", f.Document, f.Used)
	}
	if want := "4 library packages (Colors, Fonts, Shapes, …) are declared both under /ws/vendor/lib and under /ws/lib; references resolve to the declarations under /ws/lib, and the ones under /ws/vendor/lib are unused"; f.Diagnostic.Message != want {
		t.Errorf("message = %q,\nwant      %q", f.Diagnostic.Message, want)
	}
	if want := []string{"Colors", "Fonts", "Shapes", "Units"}; strings.Join(f.Packages, ",") != strings.Join(want, ",") {
		t.Errorf("packages = %v, want %v", f.Packages, want)
	}
}

// Two user library files loaded again from their interface records — which carry
// no declaration — are still a duplication: the record keeps the `library` mark.
func TestDuplicateLibrariesSurvivesAnInterfaceRecordReload(t *testing.T) {
	src := []byte("library package Foo { part def A; }")
	ws := model.NewWorkspace()
	for _, name := range []string{"/ws/a.sysml", "/ws/b.sysml"} {
		// A record is written from a workspace holding the document alone.
		loaded := model.NewWorkspace()
		loaded.Open(name, src, 1)
		loaded.Diagnostics(name)
		rec, err := loaded.InterfaceRecord(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := ws.OpenRecorded(rec, src); err != nil {
			t.Fatal(err)
		}
	}
	got := ws.WorkspaceFindings()
	if len(got) != 1 || got[0].Document != "/ws/b.sysml" || got[0].Used != "/ws/a.sysml" {
		t.Fatalf("findings over the records = %+v, want one on /ws/b.sysml with /ws/a.sysml used", got)
	}
	plain := model.NewWorkspace()
	for _, name := range []string{"/ws/p.sysml", "/ws/q.sysml"} {
		plain.Open(name, []byte("package P { part def A; }"), 1)
	}
	if got := plain.WorkspaceFindings(); len(got) != 0 {
		t.Fatalf("a plain package opened twice: %d findings, want none", len(got))
	}
}
