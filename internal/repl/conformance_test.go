package repl

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/conformance"
	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
)

// stateExtension is notation of ours: a warning at the prompt by default, an
// error when the prompt is asked strictly.
const stateExtension = "attribute def Alarm; state def S { state a { defer Alarm; } }\n"

func TestStrictMetaCommandReportsAndSetsTheMode(t *testing.T) {
	s := NewSession()
	if got := meta(t, s, "%strict"); len(got) == 0 || !strings.Contains(got[0], "off") {
		t.Fatalf("%%strict = %v, want it to report off", got)
	}
	if got := meta(t, s, "%strict on"); len(got) == 0 || !strings.Contains(got[0], "on") {
		t.Fatalf("%%strict on = %v, want it to report on", got)
	}
	if s.ConformanceMode() != conformance.ModeStrict {
		t.Fatalf("mode = %v, want strict", s.ConformanceMode())
	}
	if got := meta(t, s, "%strict off"); len(got) == 0 || !strings.Contains(got[0], "off") {
		t.Fatalf("%%strict off = %v, want it to report off", got)
	}
	if s.ConformanceMode() != conformance.ModeDefault {
		t.Fatalf("mode = %v, want default", s.ConformanceMode())
	}
}

func TestStrictMetaCommandRejectsAnUnknownSetting(t *testing.T) {
	s := NewSession()
	got := meta(t, s, "%strict maybe")
	if len(got) != 1 || !strings.Contains(got[0], "error") {
		t.Fatalf("%%strict maybe = %v, want one error line", got)
	}
	if s.ConformanceMode() != conformance.ModeDefault {
		t.Fatal("a rejected setting must leave the mode alone")
	}
}

// The mode decides what the same submission weighs.
func TestStrictModeEscalatesNotationAtThePrompt(t *testing.T) {
	s := NewSession()
	s.Submit(stateExtension)
	if notationErrors(s.Diagnostics()) != 0 {
		t.Fatalf("default mode errored: %v", s.Diagnostics())
	}
	strict := NewSession()
	meta(t, strict, "%strict on")
	strict.Submit(stateExtension)
	if notationErrors(strict.Diagnostics()) == 0 {
		t.Fatalf("strict mode reported no notation error: %v", strict.Diagnostics())
	}
}

// Switching the mode re-reports the buffer instead of serving the other mode's
// verdict from the cache.
func TestStrictMetaCommandRepeatsTheDiagnostics(t *testing.T) {
	s := NewSession()
	s.Submit(stateExtension)
	out := strings.Join(meta(t, s, "%strict on"), "\n")
	if !strings.Contains(out, passes.CodeNonstandardNotation) && !strings.Contains(out, "defer") {
		t.Fatalf("%%strict on = %q, want the buffer's findings restated", out)
	}
}

// A bare import is an error about the writing, so the model it names still runs
// by default; asked strictly, the file is rejected and nothing runs.
func TestNotationErrorStopsTheRunOnlyWhenAskedStrictly(t *testing.T) {
	const bareImport = "package Q { part def A; }\npackage P { import Q::*; part def X; }\n"
	s := NewSession()
	s.Submit(bareImport)
	if !hasImportError(s.Diagnostics()) {
		t.Fatalf("the bare import reported no error: %v", s.Diagnostics())
	}
	if s.HasErrors() {
		t.Errorf("default mode refused to run a model whose only error is notation: %v", s.Diagnostics())
	}
	strict := NewSession()
	meta(t, strict, "%strict on")
	strict.Submit(bareImport)
	if !strict.HasErrors() {
		t.Errorf("strict mode ran a file it rejects: %v", strict.Diagnostics())
	}
}

func meta(t *testing.T, s *Session, line string) []string {
	t.Helper()
	out, quit, err := s.runMeta(line)
	if err != nil || quit {
		t.Fatalf("%s: err=%v quit=%v", line, err, quit)
	}
	return out
}

func hasImportError(diags []diag.Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == diag.SeverityError && d.Code == "import-visibility" {
			return true
		}
	}
	return false
}

func notationErrors(diags []diag.Diagnostic) int {
	var n int
	for _, d := range diags {
		if d.Severity == diag.SeverityError && d.Code == passes.CodeNonstandardNotation {
			n++
		}
	}
	return n
}
