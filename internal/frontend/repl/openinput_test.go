package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// hasSyntaxError reports whether a result carries a syntax error, which is what
// an unreadable submission is reported as.
func hasSyntaxError(res Result) bool {
	for _, d := range res.Diagnostics {
		if d.Source == "syntax" && d.Severity == diag.SeverityError {
			return true
		}
	}
	return false
}

// tempFile writes src to a file of its own and returns its path.
func tempFile(t *testing.T, name, src string) string {
	t.Helper()
	return writeFile(t, filepath.Join(t.TempDir(), name), src)
}

// The poison case: a loaded file the parser cannot close absorbs whatever is
// submitted next, so the next declaration must be read on its own terms.
func TestLoadedOpenFileDoesNotPoisonTheNextSubmission(t *testing.T) {
	s := NewSession()
	bad := tempFile(t, "bad.sysml", "package Broken {\n    part def A\n")

	if _, err := s.LoadFile(bad); err != nil {
		t.Fatal(err)
	}
	res := s.Submit("package Good { part def B; attribute n : ScalarValues::Real = 2.0; }")

	if len(res.Declared) != 1 || res.Declared[0] != "Good" {
		t.Fatalf("declared = %v, want [Good]", res.Declared)
	}
	for _, name := range []string{"Good", "Good::B", "Good::n"} {
		if _, _, err := s.lookupSymbol(name); err != nil {
			t.Errorf("%s did not resolve after the bad load: %v", name, err)
		}
	}
	// The good declaration is a namespace of its own, not a member the open
	// package swallowed.
	if lines, err := s.EvalExpr("Good::n"); err != nil {
		t.Errorf("%%eval Good::n after the bad load: %v", err)
	} else if !strings.Contains(strings.Join(lines, "\n"), "2.0") {
		t.Errorf("%%eval Good::n = %v, want the declared value", lines)
	}
	if out, _, err := s.runMeta("%instantiate Good::B"); err != nil {
		t.Errorf("%%instantiate after the bad load: %v", err)
	} else if strings.Contains(strings.Join(out, "\n"), "error") {
		t.Errorf("%%instantiate after the bad load: %v", out)
	}
}

// The reason must be visible: a load whose file does not parse says so, in the
// prompt's own formatting, and leaves the session in error for a non-interactive
// run to exit on.
func TestLoadReportsSyntaxDiagnostics(t *testing.T) {
	s := NewSession()
	bad := tempFile(t, "bad.sysml", "package Broken {\n    part def A\n")

	out, err := s.LoadFile(bad)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(out, "\n"), "error:") {
		t.Errorf("load printed no diagnostics: %v", out)
	}
	if !s.HasErrors() {
		t.Error("HasErrors should be true after loading a file that does not parse")
	}
	lines := strings.Join(s.DiagnosticLines(), "\n")
	if !strings.Contains(lines, "bad.sysml") || !strings.Contains(lines, "error:") {
		t.Errorf("diagnostic lines do not name the file and the error:\n%s", lines)
	}
	located := s.LocatedDiagnostics()
	if len(located) == 0 {
		t.Fatal("located diagnostics are empty")
	}
	for _, d := range located {
		if !strings.HasSuffix(d.File, "bad.sysml") || d.Line == 0 {
			t.Errorf("diagnostic is not placed in the loaded file: %+v", d)
		}
	}
}

// The summary surface is the one a non-interactive load prints; it must not
// swallow the reason either.
func TestLoadFileSummaryReportsSyntaxDiagnostics(t *testing.T) {
	s := NewSession()
	bad := tempFile(t, "bad.sysml", "package Broken {\n    part def A\n")

	out, err := s.LoadFileSummary(bad)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(out, "\n"), "error:") {
		t.Errorf("summary load printed no diagnostics: %v", out)
	}
	if !s.HasErrors() {
		t.Error("HasErrors should be true after a summary load of a file that does not parse")
	}
}

// A summary load defers the analysis, so a syntax warning belongs to the report
// that analysis makes rather than being printed by the load as well.
func TestLoadFileSummaryDefersSyntaxWarnings(t *testing.T) {
	s := NewSession()
	path := tempFile(t, "keyword.sysml", "package P {\n    action def flow;\n}\n")

	out, err := s.LoadFileSummary(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(out, "\n"); strings.Contains(got, "warning:") {
		t.Errorf("the summary load reported the warning the analysis reports:\n%s", got)
	}
	if lines := strings.Join(s.DiagnosticLines(), "\n"); !strings.Contains(lines, "reserved keyword") {
		t.Errorf("the analysis did not report the warning:\n%s", lines)
	}
}

// A bare import is an error about the notation, not a reason the file could not be
// read, so the load defers it to the analysis rather than reporting it twice.
func TestLoadFileSummaryDefersNotationErrors(t *testing.T) {
	s := NewSession()
	path := tempFile(t, "bare.sysml", "package Q { part def A; }\npackage P { import Q::*; }\n")

	out, err := s.LoadFileSummary(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(out, "\n"); strings.Contains(got, "visibility indicator") {
		t.Errorf("the summary load reported the notation error the analysis reports:\n%s", got)
	}
	if lines := strings.Join(s.DiagnosticLines(), "\n"); !strings.Contains(lines, "visibility indicator") {
		t.Errorf("the analysis did not report the notation error:\n%s", lines)
	}
}

// A load of several files at once prints, file by file, what loading each on
// its own would have: its syntax errors, the notices accepting it raised and its
// summary. The analysis then reports the same findings against the same files.
// The KerML file and the file opening with a comment sit where a document's last
// member runs on past its file, which must not credit it to the next one.
func TestLoadFilesSummaryMatchesLoadingEachFile(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		writeFile(t, filepath.Join(dir, "k.kerml"), "package K { class A; }\n"),
		writeFile(t, filepath.Join(dir, "a.sysml"), "package A { part def X; }\npackage Shared { part def S; }\n"),
		writeFile(t, filepath.Join(dir, "bad.sysml"), "package Broken {\n    part def A\n"),
		writeFile(t, filepath.Join(dir, "b.sysml"), "// b\npackage B { part x : Missing; part y : A::X; }\n"),
		writeFile(t, filepath.Join(dir, "c.sysml"), "package Shared { part def T; }\n"),
	}

	one := NewSession()
	var want []string
	for _, path := range paths {
		out, err := one.LoadFileSummary(path)
		if err != nil {
			t.Fatal(err)
		}
		want = append(want, out...)
	}

	all := NewSession()
	got, err := all.LoadFilesSummary(paths)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("loading the files at once printed:\n%s\nwant, as loading each did:\n%s",
			strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if !strings.Contains(strings.Join(got, "\n"), "error:") {
		t.Errorf("the file that does not parse was not reported with the load:\n%s", strings.Join(got, "\n"))
	}
	var summaries []string
	for _, line := range got {
		if strings.HasPrefix(line, "✓ ") {
			summaries = append(summaries, line)
		}
	}
	wantSummaries := []string{"✓ package K", "✓ package A", "✓ package Shared", "✓ package B", "✓ package Shared"}
	if strings.Join(summaries, "\n") != strings.Join(wantSummaries, "\n") {
		t.Errorf("each file's summary should name its own declarations once, got:\n%s", strings.Join(summaries, "\n"))
	}
	if g, w := strings.Join(all.DiagnosticLines(), "\n"), strings.Join(one.DiagnosticLines(), "\n"); g != w {
		t.Errorf("analysis after loading at once reported:\n%s\nwant:\n%s", g, w)
	}
	located := all.LocatedDiagnostics()
	if len(located) == 0 {
		t.Fatal("located diagnostics are empty")
	}
	for _, d := range located {
		if filepath.Dir(d.File) != dir || d.Line == 0 {
			t.Errorf("diagnostic is not placed in a loaded file: %+v", d)
		}
	}
}

// A file that parses but whose analysis fails is a different case: it is accepted
// and reported by the deeper tiers, and the load still counts as an error.
func TestLoadReportsUnresolvedReference(t *testing.T) {
	s := NewSession()
	path := tempFile(t, "unresolved.sysml", "package P { part a : Missing; }\n")

	out, err := s.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(out, "\n"), "unresolved reference") {
		t.Errorf("load printed no unresolved reference: %v", out)
	}
	if !s.HasErrors() {
		t.Error("HasErrors should be true for a file with an unresolved reference")
	}
}

// Reloading the file once it is fixed clears the report: the masked submission is
// superseded like any other reading of the same file.
func TestReloadingAFixedFileClearsItsSyntaxError(t *testing.T) {
	s := NewSession()
	path := filepath.Join(t.TempDir(), "work.sysml")
	if err := os.WriteFile(path, []byte("package Work {\n    part def A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LoadFile(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package Work { part def A; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LoadFile(path); err != nil {
		t.Fatal(err)
	}
	if s.HasErrors() {
		t.Errorf("the fixed file should leave no errors: %v", s.DiagnosticLines())
	}
	if _, _, err := s.lookupSymbol("Work::A"); err != nil {
		t.Errorf("Work::A did not resolve after the reload: %v", err)
	}
}

// Typed input that leaves an enclosure open is masked the same way, and the
// declarations already in the buffer are untouched.
func TestOpenTypedSubmissionKeepsTheBuffer(t *testing.T) {
	s := NewSession()
	s.Submit("package Kept { part def A; }")
	res := s.Submit("/* oops")
	if !hasSyntaxError(res) {
		t.Errorf("an unterminated comment should be reported: %v", res.Diagnostics)
	}
	if _, _, err := s.lookupSymbol("Kept::A"); err != nil {
		t.Errorf("Kept::A did not survive the open submission: %v", err)
	}
	next := s.Submit("package Later { part def B; }")
	if len(next.Declared) != 1 || next.Declared[0] != "Later" {
		t.Errorf("declared = %v, want [Later]", next.Declared)
	}
	if _, _, err := s.lookupSymbol("Later::B"); err != nil {
		t.Errorf("Later::B did not resolve after the open submission: %v", err)
	}
	// The text is still the user's to save.
	if !strings.Contains(strings.Join(s.List(), "\n"), "/* oops") {
		t.Errorf("the open submission should still be listed: %v", s.List())
	}
}

// A masked submission is not allowed to silence the rest of the session: what
// the deeper tiers find in the submissions that did parse is still reported.
func TestOpenSubmissionKeepsReportingTheRestOfTheBuffer(t *testing.T) {
	s := NewSession()
	s.Submit("/* oops")
	res := s.Submit("package P { part a : Missing; }")

	if !strings.Contains(strings.Join(renderDiagnostics(res.Diagnostics, res.Source, res.diagLocation, false), "\n"), "unresolved reference") {
		t.Errorf("the unresolved reference should still be reported: %v", res.Diagnostics)
	}
	var syntax, deeper int
	for _, d := range s.Diagnostics() {
		if d.Source == "syntax" {
			syntax++
			continue
		}
		deeper++
	}
	if syntax == 0 || deeper == 0 {
		t.Errorf("diagnostics = %d syntax, %d deeper; want both", syntax, deeper)
	}
}

// The report for a masked submission quotes the line it is about: masking is for
// the analysis, not for what the user is shown.
func TestOpenSubmissionEchoesItsOwnLine(t *testing.T) {
	s := NewSession()
	res := s.Submit("package P { part x = ; }\n/* open")

	out := strings.Join(renderResult(res, VerbosityNormal), "\n")
	for _, want := range []string{"package P { part x = ; }", "/* open"} {
		if !strings.Contains(out, want) {
			t.Errorf("the report does not echo %q:\n%s", want, out)
		}
	}
	bad := tempFile(t, "bad.sysml", "package Broken { part x = ;\n")
	lines, err := s.LoadFileSummary(bad)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(lines, "\n"); !strings.Contains(got, "package Broken { part x = ;") {
		t.Errorf("the load does not echo the offending line:\n%s", got)
	}
}

// Re-typing a namespace does not fold a masked submission back into the buffer:
// the text the parser could not read stays out of it, so later declarations are
// still declared where they were typed.
func TestRetypingANamespaceDoesNotMergeAMaskedSubmission(t *testing.T) {
	s := NewSession()
	s.Submit("package P { part def A; }\npackage Q {")
	s.Submit("package P { part def B; }")
	s.Submit("package Later { part def C; }")

	for _, sn := range s.snippets {
		if !sn.open && strings.Contains(sn.src, "package Q {") {
			t.Fatalf("the masked submission was merged back into the buffer: %q", sn.src)
		}
	}
	if _, _, err := s.lookupSymbol("Later::C"); err != nil {
		t.Errorf("Later::C did not resolve after the namespace was re-typed: %v", err)
	}
}

// A masked submission gated no validation tier, so it is not reported as the
// reason the deeper checks may not have run over a later clean submission.
func TestMaskedSubmissionIsNotReportedAsBlockingTheChecks(t *testing.T) {
	s := NewSession()
	s.Submit("/* oops")
	res := s.Submit("package Clean { part def A; }")

	if note := res.Blocked.note(); note != "" {
		t.Errorf("a masked submission should block nothing: %s", note)
	}
	// A real error elsewhere in the buffer is still named.
	s.Submit("package Bad { part a : Missing; }")
	res = s.Submit("package Also { part def B; }")
	if res.Blocked.note() == "" {
		t.Error("the unresolved reference in the buffer should still be named as blocking")
	}
}

// A masked submission's warnings stay warnings, with the code the parser gave
// them, as they would in any file the workspace analyzes.
func TestOpenSubmissionKeepsWarningSeverity(t *testing.T) {
	s := NewSession()
	res := s.Submit("package Open { part def in\n")

	var warned bool
	for _, d := range res.Diagnostics {
		if d.Severity == diag.SeverityWarning && d.Code != "syntax" {
			warned = true
		}
	}
	if !warned {
		t.Errorf("the reserved-name warning should survive masking as a warning: %+v", res.Diagnostics)
	}
	if !hasSyntaxError(res) {
		t.Errorf("the unreadable submission should still be an error: %+v", res.Diagnostics)
	}
}

// Robustness: input the parser cannot read is answered with a typed error rather
// than a panic or a hang, whichever surface it reaches.
func TestOpenSubmissionSurfacesStayTyped(t *testing.T) {
	for _, src := range []string{
		"package P {",
		"/* unterminated",
		"part def 'unterminated",
		"package P { part x : ",
		"}}}",
		"package P { state def S { entry; then ",
	} {
		s := NewSession()
		res := s.Submit(src)
		if len(res.Diagnostics) == 0 {
			t.Errorf("%q produced no diagnostics", src)
		}
		// Every command must answer, not panic, over a session holding it.
		for _, cmd := range []string{"%list", "%symbols", "%check", "%eval 1 + 1", "%view P"} {
			if _, _, err := s.runMeta(cmd); err != nil && strings.Contains(err.Error(), "panic") {
				t.Errorf("%q over %q: %v", cmd, src, err)
			}
		}
		if _, _, err := s.lookupSymbol("Nothing"); err == nil {
			t.Errorf("%q: an undeclared name should not resolve", src)
		}
	}
}
