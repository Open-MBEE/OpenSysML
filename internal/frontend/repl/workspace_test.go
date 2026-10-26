package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const libSource = "package Lib { attribute a = 3; calc def Twice { in x; 2 * x } }\n"

// workspaceSession is a session over a directory holding lib.sysml.
func workspaceSession(t *testing.T) (*Session, string) {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "lib.sysml"), libSource)
	s := NewSession()
	if err := s.SetWorkspaceRoot(dir); err != nil {
		t.Fatal(err)
	}
	return s, dir
}

func unresolved(res Result) []string {
	var out []string
	for _, d := range res.Diagnostics {
		if d.Code == "unresolved" {
			out = append(out, d.Message)
		}
	}
	return out
}

func TestAnUnresolvedImportLoadsTheWorkspaceFileDeclaringItInTheSameSubmission(t *testing.T) {
	s, dir := workspaceSession(t)
	res := s.Submit("package Mine { private import Lib::*; attribute b = a + 1; }")
	if got := unresolved(res); len(got) != 0 {
		t.Fatalf("the import stayed unresolved: %v", got)
	}
	if got := strings.Join(res.Notices, "\n"); !strings.Contains(got, "note: loaded lib.sysml, which declares Lib") {
		t.Errorf("notices = %q", got)
	}
	if got := strings.Join(renderResult(res, VerbosityNormal), "\n"); !strings.Contains(got, "✓ package Lib") || !strings.Contains(got, "✓ package Mine") {
		t.Errorf("rendered = %q", got)
	}
	if got := s.Files(); len(got) != 1 || got[0] != filepath.Join(dir, "lib.sysml") {
		t.Errorf("files = %v", got)
	}
	ev, err := s.Evaluate("Mine::b")
	if err != nil || ev.Text != "4" {
		t.Errorf("Mine::b = %q, %v", ev.Text, err)
	}
}

func TestAnImportTypedAtTheRootLoadsTheFileAndItsValueEvaluates(t *testing.T) {
	s, _ := workspaceSession(t)
	if got := unresolved(s.Submit("private import Lib::*;")); len(got) != 0 {
		t.Fatalf("the import stayed unresolved: %v", got)
	}
	lines, err := s.EvalBare("a")
	if err != nil || !strings.Contains(strings.Join(lines, "\n"), "= 3") {
		t.Errorf("a = %v, %v", lines, err)
	}
}

func TestListShowsTheFileLoadedOnDemand(t *testing.T) {
	s, _ := workspaceSession(t)
	s.Submit("private import Lib::*;")
	if got := strings.Join(s.List(), "\n"); !strings.Contains(got, "package Lib { attribute a = 3;") {
		t.Errorf("%%list = %q", got)
	}
}

// saveTo saves the session to path and returns what was written.
func saveTo(t *testing.T, s *Session, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.RunMeta("%save " + path); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(saved)
}

func TestSaveBesideTheFileLoadedOnDemandLeavesItOut(t *testing.T) {
	s, dir := workspaceSession(t)
	s.Submit("private import Lib::*;")
	s.Submit("package Mine { attribute b = a + 1; }")
	got := saveTo(t, s, filepath.Join(dir, "session.sysml"))
	if !strings.Contains(got, "import Lib::*") || !strings.Contains(got, "package Mine") || strings.Contains(got, "package Lib") {
		t.Errorf("saved:\n%s", got)
	}
	// The saved file loads on its own: its import finds lib.sysml beside it.
	fresh := NewSession()
	if _, err := fresh.LoadPaths([]string{filepath.Join(dir, "session.sysml")}); err != nil {
		t.Fatal(err)
	}
	if diags := fresh.DiagnosticLines(); len(diags) != 0 {
		t.Errorf("the saved file did not load clean: %v", diags)
	}
}

func TestSaveWhereTheFileLoadedOnDemandIsOutOfReachWritesItsDeclarations(t *testing.T) {
	s, dir := workspaceSession(t)
	s.Submit("private import Lib::*;")
	for _, out := range []string{filepath.Join(dir, "out", "session.sysml"), filepath.Join(t.TempDir(), "session.sysml")} {
		got := saveTo(t, s, out)
		if !strings.Contains(got, "import Lib::*") || !strings.Contains(got, "package Lib") {
			t.Errorf("saved to %s:\n%s", out, got)
		}
		fresh := NewSession()
		if _, err := fresh.LoadPaths([]string{out}); err != nil {
			t.Fatal(err)
		}
		if diags := fresh.DiagnosticLines(); len(diags) != 0 {
			t.Errorf("%s did not load clean: %v", out, diags)
		}
	}
}

func TestSaveLeavesOutAFileLoadedOnDemandThatTheTypedTextNoLongerNames(t *testing.T) {
	s, _ := workspaceSession(t)
	if _, err := s.Evaluate("Lib::a"); err != nil {
		t.Fatal(err)
	}
	s.Submit("package Mine { attribute b = 1; }")
	if got := saveTo(t, s, filepath.Join(t.TempDir(), "session.sysml")); strings.Contains(got, "package Lib") || !strings.Contains(got, "package Mine") {
		t.Errorf("saved:\n%s", got)
	}
}

func TestSaveStillWritesAFileLoadedByName(t *testing.T) {
	s, dir := workspaceSession(t)
	if _, err := s.LoadPaths([]string{filepath.Join(dir, "lib.sysml")}); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out", "session.sysml")
	if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.RunMeta("%save " + out); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(saved), "package Lib") {
		t.Errorf("saved:\n%s", saved)
	}
}

func TestRedeclaringTheLoadedPackageReplacesTheFile(t *testing.T) {
	s, _ := workspaceSession(t)
	s.Submit("private import Lib::*;")
	res := s.Submit("package Lib { attribute a = 5; }")
	if got := strings.Join(res.Notices, "\n"); !strings.Contains(got, "replaced package Lib") {
		t.Errorf("notices = %q", got)
	}
	if got := s.Files(); len(got) != 0 {
		t.Errorf("files = %v, want the loaded file replaced", got)
	}
	if ev, err := s.Evaluate("Lib::a"); err != nil || ev.Text != "5" {
		t.Errorf("Lib::a = %q, %v", ev.Text, err)
	}
}

func TestClearDropsTheFileLoadedOnDemandAndKeepsTheRoot(t *testing.T) {
	s, dir := workspaceSession(t)
	s.Submit("private import Lib::*;")
	s.Clear()
	if got := s.Files(); len(got) != 0 {
		t.Errorf("files after %%clear = %v", got)
	}
	if got := s.List(); len(got) != 0 {
		t.Errorf("%%list after %%clear = %v", got)
	}
	if got := s.WorkspaceRoot(); got != dir {
		t.Errorf("root after %%clear = %q, want %q", got, dir)
	}
	if got := unresolved(s.Submit("private import Lib::*;")); len(got) != 0 {
		t.Errorf("the import after %%clear stayed unresolved: %v", got)
	}
}

func TestANameNoFileDeclaresStaysUnresolvedAndLoadsNothing(t *testing.T) {
	s, _ := workspaceSession(t)
	res := s.Submit("private import Nope::*;")
	if got := unresolved(res); len(got) != 1 || !strings.Contains(got[0], "unresolved reference: Nope") {
		t.Errorf("diagnostics = %v", got)
	}
	if len(res.Notices) != 0 || len(s.Files()) != 0 {
		t.Errorf("notices %v, files %v: want nothing loaded", res.Notices, s.Files())
	}
}

func TestAReferenceWithinALoadedPackageDoesNotLoadAnotherFile(t *testing.T) {
	s, dir := workspaceSession(t)
	writeFile(t, filepath.Join(dir, "other.sysml"), "package Other { attribute o = 1; }\n")
	res := s.Submit("package Mine { attribute b = Lib::nope; }")
	if got := unresolved(res); len(got) != 1 || !strings.Contains(got[0], "Lib::nope") {
		t.Errorf("diagnostics = %v", got)
	}
	if got := s.Files(); len(got) != 1 || filepath.Base(got[0]) != "lib.sysml" {
		t.Errorf("files = %v, want lib.sysml alone", got)
	}
}

func TestTwoFilesDeclaringTheNameLoadNeither(t *testing.T) {
	s, dir := workspaceSession(t)
	writeFile(t, filepath.Join(dir, "sub", "lib2.sysml"), "package Lib { attribute a = 4; }\n")
	res := s.Submit("private import Lib::*;")
	if got := unresolved(res); len(got) != 1 {
		t.Errorf("diagnostics = %v, want the import still unresolved", got)
	}
	want := "note: Lib is declared by more than one file under " + dir + ": lib.sysml, " + filepath.Join("sub", "lib2.sysml") + "; load the one meant with %load"
	if got := strings.Join(res.Notices, "\n"); !strings.Contains(got, want) {
		t.Errorf("notices = %q, want %q", got, want)
	}
	if got := s.Files(); len(got) != 0 {
		t.Errorf("files = %v, want none", got)
	}
}

func TestAMalformedCandidateIsReportedAgainstItsFile(t *testing.T) {
	s, dir := workspaceSession(t)
	writeFile(t, filepath.Join(dir, "lib.sysml"), "package Lib {\n    attribute a = ;\n}\n")
	res := s.Submit("private import Lib::*;")
	var located []Diagnostic
	for _, d := range s.Locate(res, res.Diagnostics) {
		if d.File != "" {
			located = append(located, d)
		}
	}
	if len(located) == 0 {
		t.Fatalf("no diagnostic against the file; diagnostics: %v", res.Diagnostics)
	}
	if got := located[0]; got.File != filepath.Join(dir, "lib.sysml") || got.Line != 2 {
		t.Errorf("located at %s:%d, want lib.sysml:2", got.File, got.Line)
	}
	if got := strings.Join(renderResult(res, VerbosityNormal), "\n"); !strings.Contains(got, "lib.sysml:2:") {
		t.Errorf("rendered = %q", got)
	}
}

func TestTheLoadedFilesOwnImportsAreFollowed(t *testing.T) {
	s, dir := workspaceSession(t)
	writeFile(t, filepath.Join(dir, "lib.sysml"), "package Lib { private import Util::*; attribute a = base + 1; }\n")
	writeFile(t, filepath.Join(dir, "util", "util.sysml"), "package Util { attribute base = 2; }\n")
	res := s.Submit("private import Lib::*;")
	if got := unresolved(res); len(got) != 0 {
		t.Errorf("diagnostics = %v", got)
	}
	if got := strings.Join(res.Notices, "\n"); !strings.Contains(got, "note: loaded "+filepath.Join("util", "util.sysml")+", which declares Util") {
		t.Errorf("notices = %q", got)
	}
	if ev, err := s.Evaluate("Lib::a"); err != nil || ev.Text != "3" {
		t.Errorf("Lib::a = %q, %v", ev.Text, err)
	}
}

func TestALoadedFileImportingATypedPackageLoadsNoSibling(t *testing.T) {
	s, dir := workspaceSession(t)
	writeFile(t, filepath.Join(dir, "lib.sysml"), "package Lib { private import Base::*; attribute a = base + 1; }\n")
	writeFile(t, filepath.Join(dir, "base.sysml"), "package Base { attribute base = 100; }\n")
	s.Submit("package Base { attribute base = 2; }")
	res := s.Submit("private import Lib::*;")
	if got := unresolved(res); len(got) != 0 {
		t.Errorf("diagnostics = %v", got)
	}
	if got := s.Files(); len(got) != 1 || filepath.Base(got[0]) != "lib.sysml" {
		t.Errorf("files = %v, want lib.sysml alone", got)
	}
	if ev, err := s.Evaluate("Lib::a"); err != nil || ev.Text != "3" {
		t.Errorf("Lib::a = %q, %v", ev.Text, err)
	}
}

func TestWithoutARootAnUnresolvedImportStaysUnresolved(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "lib.sysml"), libSource)
	s := NewSession()
	res := s.Submit("private import Lib::*;")
	if got := unresolved(res); len(got) != 1 {
		t.Errorf("diagnostics = %v", got)
	}
	if len(res.Notices) != 0 || len(s.Files()) != 0 {
		t.Errorf("notices %v, files %v", res.Notices, s.Files())
	}
	if got, _, _ := s.RunMeta("%workspace"); len(got) != 1 || !strings.HasPrefix(got[0], "no workspace root") {
		t.Errorf("%%workspace = %v", got)
	}
}

func TestEvalLoadsTheFileDeclaringTheQualifiedName(t *testing.T) {
	s, _ := workspaceSession(t)
	ev, err := s.Evaluate("Lib::a * 2")
	if err != nil || ev.Text != "6" {
		t.Fatalf("Lib::a * 2 = %q, %v", ev.Text, err)
	}
	if got := strings.Join(ev.Lines, "\n"); !strings.Contains(got, "note: loaded lib.sysml, which declares Lib") {
		t.Errorf("lines = %q", got)
	}
}

func TestEvalOfANameNoFileDeclaresFailsWithoutLoading(t *testing.T) {
	s, _ := workspaceSession(t)
	if _, err := s.Evaluate("Nope::a"); err == nil || !strings.Contains(err.Error(), "Nope") {
		t.Errorf("err = %v", err)
	}
	if got := s.Files(); len(got) != 0 {
		t.Errorf("files = %v", got)
	}
}

func TestCalcLoadsTheFileDeclaringIt(t *testing.T) {
	s, _ := workspaceSession(t)
	v := s.RunCalc("Lib::Twice(3)")
	if v.Status != VerdictHolds {
		t.Fatalf("verdict = %+v", v)
	}
	if got := strings.Join(v.Lines, "\n"); !strings.Contains(got, "note: loaded lib.sysml, which declares Lib") || !strings.Contains(got, "= [6]") {
		t.Errorf("lines = %q", got)
	}
}

func TestAChangedFileIsReadAgain(t *testing.T) {
	s, dir := workspaceSession(t)
	s.Submit("private import Lib::*;")
	s.Clear()
	writeFile(t, filepath.Join(dir, "lib.sysml"), "package Lib { attribute a = 30; }\n")
	s.Submit("private import Lib::*;")
	if ev, err := s.Evaluate("a"); err != nil || ev.Text != "30" {
		t.Errorf("a = %q, %v", ev.Text, err)
	}
}

func TestAHiddenDirectoryIsNotSearched(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".cache", "lib.sysml"), libSource)
	s := NewSession()
	if err := s.SetWorkspaceRoot(dir); err != nil {
		t.Fatal(err)
	}
	if got := unresolved(s.Submit("private import Lib::*;")); len(got) != 1 {
		t.Errorf("diagnostics = %v, want the import unresolved", got)
	}
}

func TestWorkspaceCommandShowsTheRoot(t *testing.T) {
	s, dir := workspaceSession(t)
	got, _, err := s.RunMeta("%workspace")
	if err != nil || len(got) == 0 || got[0] != "workspace root: "+dir {
		t.Errorf("%%workspace = %v, %v", got, err)
	}
	if help, _, _ := s.RunMeta("%help"); !strings.Contains(strings.Join(help, "\n"), "%workspace") {
		t.Error("%help does not list %workspace")
	}
}

func TestSetWorkspaceRootRefusesWhatIsNoDirectory(t *testing.T) {
	dir := t.TempDir()
	file := writeFile(t, filepath.Join(dir, "lib.sysml"), libSource)
	s := NewSession()
	if err := s.SetWorkspaceRoot(file); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("err = %v", err)
	}
	if err := s.SetWorkspaceRoot(filepath.Join(dir, "missing")); err == nil {
		t.Error("a missing directory was accepted")
	}
	if err := s.SetWorkspaceRoot(dir); err != nil {
		t.Fatal(err)
	}
	if err := s.SetWorkspaceRoot(""); err != nil || s.WorkspaceRoot() != "" {
		t.Errorf("clearing the root: %v, root %q", err, s.WorkspaceRoot())
	}
}

func TestTheProjectsTheRootUsesAreSearchedToo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".project.json"), `{"name": "App", "version": "1.0.0", "usage": [{"resource": "pkg:sysand/acme/gears"}]}`)
	writeFile(t, filepath.Join(dir, ".sysand", "lib", "gears", ".project.json"), `{"name": "Gears", "publisher": "acme", "version": "1.0.0"}`)
	gears := writeFile(t, filepath.Join(dir, ".sysand", "lib", "gears", "gears.sysml"), "package Gears { attribute teeth = 12; }\n")
	writeFile(t, filepath.Join(dir, ".sysand", "lib", "unused", ".project.json"), `{"name": "Unused", "publisher": "acme", "version": "1.0.0"}`)
	writeFile(t, filepath.Join(dir, ".sysand", "lib", "unused", "unused.sysml"), "package Unused { attribute u = 1; }\n")
	s := NewSession()
	if err := s.SetWorkspaceRoot(dir); err != nil {
		t.Fatal(err)
	}
	res := s.Submit("package Mine { private import Gears::*; attribute t = teeth; }")
	if got := unresolved(res); len(got) != 0 {
		t.Errorf("diagnostics = %v", got)
	}
	if got := s.Files(); len(got) != 1 || got[0] != gears {
		t.Errorf("files = %v, want %s", got, gears)
	}
	if got := unresolved(s.Submit("private import Unused::*;")); len(got) != 1 {
		t.Errorf("a project the root does not use was searched: %v", got)
	}
}

func TestAGlobalImportLoadsTheFileToo(t *testing.T) {
	s, _ := workspaceSession(t)
	if got := unresolved(s.Submit("private import $::Lib::*;")); len(got) != 0 {
		t.Errorf("the global import stayed unresolved: %v", got)
	}
	if ev, err := s.Evaluate("$::Lib::a"); err != nil || ev.Text != "3" {
		t.Errorf("$::Lib::a = %q, %v", ev.Text, err)
	}
}

func TestAFileReplacingAnEarlierSnippetKeepsTheSubmissionLocated(t *testing.T) {
	s, dir := workspaceSession(t)
	writeFile(t, filepath.Join(dir, "lib.sysml"), "package Other { attribute o = 2; }\npackage Lib {\n    attribute a = ;\n}\n")
	s.Submit("package Other {\n    attribute o = 1;\n    attribute p = 2;\n    attribute q = 3;\n    attribute r = 4;\n}")
	res := s.Submit("private import Lib::*;")
	if got := strings.Join(res.Notices, "\n"); !strings.Contains(got, "replaced package Other") {
		t.Errorf("notices = %q", got)
	}
	own := res.OwnDiagnostics(VerbosityNormal)
	if len(own) != 1 || !strings.Contains(own[0].Message, "expected an expression") {
		t.Errorf("own diagnostics = %v, want the file's syntax error", own)
	}
	if got := strings.Join(renderResult(res, VerbosityNormal), "\n"); !strings.Contains(got, "lib.sysml:3:") {
		t.Errorf("rendered = %q", got)
	}
}

func TestChangingTheLibraryRootsRebuildsTheIndex(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "ws")
	writeFile(t, filepath.Join(root, "main.sysml"), "package Main;\n")
	writeFile(t, filepath.Join(dir, "installed", "gears", ".project.json"), `{"name": "Gears", "publisher": "acme", "version": "1.0.0"}`)
	writeFile(t, filepath.Join(dir, "installed", "gears", "gears.sysml"), "package Gears { attribute teeth = 12; }\n")
	s := NewSession()
	s.SetLibraryRoots(nil)
	if err := s.SetWorkspaceRoot(root); err != nil {
		t.Fatal(err)
	}
	if got := unresolved(s.Submit("private import Gears::*;")); len(got) != 1 {
		t.Fatalf("diagnostics before the roots = %v", got)
	}
	s.SetLibraryRoots([]string{filepath.Join(dir, "installed")})
	if got := unresolved(s.Submit("private import Gears::*;")); len(got) != 0 {
		t.Errorf("diagnostics after the roots = %v", got)
	}
}

func TestSaveOutOfReachCarriesTheLoadedFilesOwnDependencies(t *testing.T) {
	s, dir := workspaceSession(t)
	writeFile(t, filepath.Join(dir, "lib.sysml"), "package Lib { private import Dep::*; attribute a = d + 1; }\n")
	writeFile(t, filepath.Join(dir, "dep.sysml"), "package Dep { attribute d = 2; }\n")
	s.Submit("private import Lib::*;")
	out := filepath.Join(t.TempDir(), "session.sysml")
	got := saveTo(t, s, out)
	if !strings.Contains(got, "package Lib") || !strings.Contains(got, "package Dep") {
		t.Errorf("saved:\n%s", got)
	}
	fresh := NewSession()
	if _, err := fresh.LoadPaths([]string{out}); err != nil {
		t.Fatal(err)
	}
	if diags := fresh.DiagnosticLines(); len(diags) != 0 {
		t.Errorf("%s did not load clean: %v", out, diags)
	}
	if ev, err := fresh.Evaluate("Lib::a"); err != nil || ev.Text != "3" {
		t.Errorf("Lib::a from the save = %q, %v", ev.Text, err)
	}
}

func TestEvalFollowsTheImportsOfTheFileItLoadedOnDemand(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "lib.sysml"), "package Lib {\n    private import Util::*;\n    attribute a = base * 3;\n}\n")
	writeFile(t, filepath.Join(dir, "util.sysml"), "package Util {\n    attribute base = 2;\n}\n")
	s := NewSession()
	if err := s.SetWorkspaceRoot(dir); err != nil {
		t.Fatal(err)
	}
	ev, err := s.Evaluate("Lib::a")
	if err != nil || ev.Text != "6" {
		t.Fatalf("Lib::a = %q, %v", ev.Text, err)
	}
	if got := strings.Join(ev.Lines, "\n"); !strings.Contains(got, "which declares Lib") || !strings.Contains(got, "which declares Util") {
		t.Errorf("lines = %q, want lib.sysml and util.sysml loaded", got)
	}
}
