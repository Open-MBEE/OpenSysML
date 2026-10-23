package repl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rejectedCalc and rejectedLine are the issue's session: a calc, and the typed
// `add(10 [m], 20 [mm]);`, which no grammar of the prompt accepts whole.
const rejectedCalc = "calc add { in x; in y; x + y }"

const rejectedLine = "add(10 [m], 20 [mm]);"

func sessionWithCalc(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	for _, src := range []string{rejectedCalc, "private import SI::*;"} {
		if res := s.Submit(src); len(res.Diagnostics) > 0 {
			t.Fatalf("Submit(%q) diagnostics = %v", src, res.Diagnostics)
		}
	}
	return s
}

// hasDiagnostic reports whether a result carries a diagnostic mentioning fragment.
func hasDiagnostic(res Result, fragment string) bool {
	for _, d := range res.Diagnostics {
		if strings.Contains(d.Message, fragment) {
			return true
		}
	}
	return false
}

// A submission that declares nothing and does not parse is reported once,
// against the buffer it was typed into, and then leaves no trace: it is not
// listed, not saved, and does not fail what is evaluated or submitted after it.
func TestRejectedNamelessSubmissionLeavesNoTrace(t *testing.T) {
	s := sessionWithCalc(t)
	res := s.Submit(rejectedLine)
	if !hasSyntaxError(res) || !res.Rejected() {
		t.Fatalf("Submit(%q) = %+v, want the syntax error reported", rejectedLine, res.Diagnostics)
	}
	if !strings.Contains(res.Source, rejectedLine) {
		t.Errorf("the rejected text is not in the source its diagnostic is reported against:\n%s", res.Source)
	}

	if list := strings.Join(s.List(), "\n"); strings.Contains(list, rejectedLine) {
		t.Errorf("%%list still shows the rejected submission:\n%s", list)
	}
	path := filepath.Join(t.TempDir(), "out.sysml")
	saved := run(t, s, "%save "+path)
	rejects(t, saved, "warning:", "syntax error(s)")
	if data, err := os.ReadFile(path); err != nil || strings.Contains(string(data), rejectedLine) {
		t.Errorf("%%save wrote the rejected submission (%v):\n%s", err, data)
	}
	lines, err := s.EvalBare("add(10 [m], 20 [mm])")
	if err != nil {
		t.Fatalf("EvalBare after the rejected submission: %v", err)
	}
	wants(t, strings.Join(lines, "\n"), "10.02 [m]")

	next := s.Submit("part def Spare;")
	if strings.Contains(next.Source, rejectedLine) {
		t.Errorf("the next submission's source still carries the rejected text:\n%s", next.Source)
	}
	if next.Failed(VerbosityDebug) || hasDiagnostic(next, "expected a namespace member") {
		t.Errorf("the next submission reports the stale error: %v", next.Diagnostics)
	}
}

// The commands that append the expression to the transcript answer after a
// rejected submission, with no note about an unresolved error in the buffer.
func TestEvalAndCalcAnswerAfterARejectedSubmission(t *testing.T) {
	s := sessionWithCalc(t)
	s.Submit("add(1, 2);")
	for _, cmd := range []string{"%eval add(1, 2)", "%calc add(1, 2)"} {
		got := run(t, s, cmd)
		wants(t, got, "= [3]")
		rejects(t, got, "deeper checks", "unresolved", "expected a namespace member")
	}
}

// At debug verbosity every error in the buffer fails a submission, so the
// rejected text must be gone before the next one is reported.
func TestRejectedSubmissionDoesNotFailLaterSubmissionsAtDebugVerbosity(t *testing.T) {
	s := sessionWithCalc(t)
	s.SetVerbosity(VerbosityDebug)
	if res := s.Submit("add(1, 2);"); !res.Failed(VerbosityDebug) {
		t.Fatalf("the rejected submission did not fail: %v", res.Diagnostics)
	}
	next := s.Submit("part def Spare;")
	if next.Failed(VerbosityDebug) {
		t.Errorf("the submission after a rejected one fails at debug verbosity: %v", next.Diagnostics)
	}
	rejects(t, strings.Join(renderResult(next, VerbosityDebug), "\n"), "expected a namespace member")
}

// A submission that declares nothing but parses is part of the session: an
// import brings its names in and a trailing comment is kept where it was typed.
func TestNamelessValidSubmissionsStayInTheBuffer(t *testing.T) {
	s := NewSession()
	for _, src := range []string{"private import SI::*;", "part def A;", "// a thought"} {
		if res := s.Submit(src); len(res.Diagnostics) > 0 {
			t.Fatalf("Submit(%q) diagnostics = %v", src, res.Diagnostics)
		}
	}
	list := strings.Join(s.List(), "\n")
	wants(t, list, "private import SI::*;", "// a thought", "part def A;")
	if got := len(s.List()); got != 3 {
		t.Errorf("want 3 snippets, got %d:\n%s", got, list)
	}
}

// A submission that does not parse but declares a name stays, reported, until
// the name is declared again: the redeclaration is what replaces it.
func TestBrokenDeclarationStaysUntilRedeclared(t *testing.T) {
	s := NewSession()
	broken := "part def A { part x : ; }"
	if res := s.Submit(broken); !hasSyntaxError(res) {
		t.Fatalf("Submit(%q) = %v, want a syntax error", broken, res.Diagnostics)
	}
	wants(t, strings.Join(s.List(), "\n"), broken)

	res := s.Submit("part def A;")
	if !hasNotice(res, "replaced part def A") {
		t.Errorf("notices = %v, want the broken declaration replaced", res.Notices)
	}
	list := strings.Join(s.List(), "\n")
	rejects(t, list, broken)
	wants(t, list, "part def A;")
}

// The prompt answers the corrected expression after rejecting the one typed
// with a trailing semicolon, and lists nothing of the rejected line.
func TestLoopRecoversFromARejectedExpression(t *testing.T) {
	script := []string{
		rejectedCalc,
		"private import SI::*;",
		"add(10 [m], 20 [mm])",
		rejectedLine,
		"add(10 [m], 20 [mm])",
		"%list",
	}
	var out strings.Builder
	if err := Loop(&scriptReader{lines: script}, &out, NewSession()); err != nil {
		t.Fatalf("Loop error: %v", err)
	}
	got := out.String()
	if n := strings.Count(got, "= [10.02 [m]]"); n != 2 {
		t.Errorf("want the expression answered twice, got %d:\n%s", n, got)
	}
	wants(t, got, "expected a namespace member")
	rejects(t, got, "parse failed")
	if n := strings.Count(got, rejectedLine); n != 1 {
		t.Errorf("want the rejected line echoed once by its diagnostic and never by %%list, got %d:\n%s", n, got)
	}
}
