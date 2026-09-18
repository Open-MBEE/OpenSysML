package repl

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

// A source that analyses to exactly one warning and no error.
const warningSrc = `package W { attribute flag = 1 == "one"; }`

func TestWarningSourceIsAWarning(t *testing.T) {
	r := NewSession().Submit(warningSrc)
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Severity != diag.SeverityWarning {
		t.Fatalf("fixture is not a single warning: %v", r.Diagnostics)
	}
}

// A submission that analyses clean is confirmed even when an earlier
// submission left an error in the buffer, and that error is not re-echoed.
func TestEarlierErrorDoesNotSuppressThisSubmission(t *testing.T) {
	s := NewSession()
	s.Submit("namespace N { private import Missing::X; }")
	got := strings.Join(renderResult(s.Submit("package P { }"), VerbosityNormal), "\n")

	wants(t, got, "package P")
	rejects(t, got, "Missing::X")
}

// A clean report is not a full check while an earlier error blocks the higher
// validation tiers, so the confirmation says so rather than reading as a pass.
func TestBlockedAnalysisIsNotReportedAsClean(t *testing.T) {
	s := NewSession()
	s.Submit("namespace N { private import Missing::X; }")
	got := strings.Join(renderResult(s.Submit("package P { }"), VerbosityNormal), "\n")
	// The blocking error is named by the buffer line it is on, which is what
	// makes it something to go and fix.
	wants(t, got, "deeper checks may not have run here", "buffer line 1")

	clean := strings.Join(renderResult(NewSession().Submit("package P { }"), VerbosityNormal), "\n")
	rejects(t, clean, "unresolved")
}

// The warning belongs to the error, not to every command typed after it: an
// error that stands is named once, and named again only when a different one
// starts blocking the checks.
func TestBlockingErrorIsNamedOnceNotOnEverySubmission(t *testing.T) {
	s := NewSession()
	s.Submit("namespace N { private import Missing::X; }")
	first := strings.Join(renderResult(s.Submit("package P { }"), VerbosityNormal), "\n")
	wants(t, first, "deeper checks may not have run here")

	again := strings.Join(renderResult(s.Submit("package Q { }"), VerbosityNormal), "\n")
	rejects(t, again, "deeper checks")

	// A second, different blocker is worth saying, and is counted with the first.
	s.Submit("namespace M { private import Absent::Y; }")
	third := strings.Join(renderResult(s.Submit("package R { }"), VerbosityNormal), "\n")
	wants(t, third, "deeper checks may not have run here", "1 error elsewhere in the buffer")
}

// A report that leaves the note out has said nothing to be quiet about: a
// submission rendered at debug verbosity, or one with errors of its own, does
// not use up the one warning the standing error gets.
func TestBlockingErrorIsStillNamedWhenTheNoteWasNotPrinted(t *testing.T) {
	s := NewSession()
	s.Submit("namespace N { private import Missing::X; }")

	debug := strings.Join(renderResult(s.Submit("package P { }"), VerbosityDebug), "\n")
	rejects(t, debug, "deeper checks may not have run here")

	named := strings.Join(renderResult(s.Submit("package Q { }"), VerbosityNormal), "\n")
	wants(t, named, "deeper checks may not have run here", "buffer line 1")

	// Nor does a submission reporting errors of its own, which is what there is
	// to read there instead of the note.
	other := NewSession()
	other.Submit("namespace N { private import Missing::X; }")
	res := other.Submit("package R { import Gone::Z; }")
	rejects(t, strings.Join(renderResult(res, VerbosityNormal), "\n"), "deeper checks may not have run here")
	if key := other.notedBlocker.reportedKey(); key != "" {
		t.Errorf("notedBlocker = %q after a report that left the note out", key)
	}
}

// The summary covers what this submission declared, not the whole buffer.
func TestSummaryCoversOnlyThisSubmission(t *testing.T) {
	s := NewSession()
	s.Submit("package Earlier { }")
	got := strings.Join(renderResult(s.Submit("package P { }"), VerbosityNormal), "\n")

	wants(t, got, "package P")
	rejects(t, got, "package Earlier")
}

// Positions are reported against what the user typed, not against the buffer
// the submission was appended to.
func TestDiagnosticLinesAreRelativeToTheSubmission(t *testing.T) {
	s := NewSession()
	s.Submit("package Earlier {\n}\n")
	r := s.Submit("namespace N {\n\tprivate import Missing::X;\n}")
	got := strings.Join(renderResult(r, VerbosityNormal), "\n")

	wants(t, got, "2:", "Missing::X")
	rejects(t, got, "5:")
}

func TestLocatedDiagnosticsAreRelativeToEachSnippet(t *testing.T) {
	s := NewSession()
	s.SubmitFiles([]SourceFile{
		{Name: "first.sysml", Text: "package First {\n\tprivate import Missing::X;\n}"},
		{Name: "second.sysml", Text: "package Second {\n\tprivate import Missing::Y;\n}"},
	})

	got := s.LocatedDiagnostics()
	if len(got) != 2 {
		t.Fatalf("LocatedDiagnostics() returned %d diagnostics, want 2: %v", len(got), got)
	}
	for _, d := range got {
		if d.Line != 2 || d.Column == 0 {
			t.Errorf("diagnostic location = %+v, want line 2 with a column", d)
		}
	}
}

// Quiet drops warnings; normal keeps them. Neither drops an error.
func TestVerbosityFiltersBySeverity(t *testing.T) {
	quiet := strings.Join(renderResult(NewSession().Submit(warningSrc), VerbosityQuiet), "\n")
	rejects(t, quiet, "warning:")
	wants(t, quiet, "package W")

	normal := strings.Join(renderResult(NewSession().Submit(warningSrc), VerbosityNormal), "\n")
	wants(t, normal, "warning:", "package W")

	errored := strings.Join(renderResult(NewSession().Submit("namespace N { private import Missing::X; }"), VerbosityQuiet), "\n")
	wants(t, errored, "error:")
}

// Debug reports the whole buffer, at buffer-absolute positions, naming the pass
// behind each diagnostic.
func TestDebugReportsTheWholeBuffer(t *testing.T) {
	s := NewSession()
	s.Submit("namespace N {\n\tprivate import Missing::X;\n}")
	got := strings.Join(renderResult(s.Submit("package P { }"), VerbosityDebug), "\n")

	wants(t, got, "[debug]", "Missing::X", "2:")
	if !strings.Contains(got, "[nameres") && !strings.Contains(got, "[name") {
		t.Errorf("debug output does not name the pass:\n%s", got)
	}
}

func TestVerbosityMetaCommand(t *testing.T) {
	s := NewSession()
	wants(t, run(t, s, "%verbosity"), "verbosity: normal")
	wants(t, run(t, s, "%verbosity debug"), "verbosity: debug")
	if s.Verbosity() != VerbosityDebug {
		t.Errorf("verbosity = %v, want debug", s.Verbosity())
	}
	wants(t, run(t, s, "%verbosity loud"), `error: unknown verbosity "loud"`)
	if s.Verbosity() != VerbosityDebug {
		t.Error("an unknown level changed the verbosity")
	}
}
