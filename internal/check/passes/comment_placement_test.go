package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A /* */ comment where no member may start is the parser's warning in the
// default mode and an error under strict conformance, reported once either way.
func TestMisplacedCommentEscalatesUnderStrictMode(t *testing.T) {
	const src = "state def S { state Idle; state Off; transition first Idle accept Stop /* c */ then Off; }"

	def := notationDiags(t, "a.sysml", src, diag.ConformanceDefault)
	if len(def) != 0 {
		t.Fatalf("the finding is the parser's own warning, not a pass finding: got %+v", def)
	}

	root, pd, idx := analyzeInputs(t, "a.sysml", src)
	var warnings int
	for _, d := range pd {
		if d.Severity == diag.SeverityWarning && d.Code == CodeNonstandardNotation {
			warnings++
		}
	}
	if warnings != 1 {
		t.Fatalf("parse diagnostics %+v: want one nonstandard-notation warning", pd)
	}

	got := []diag.Diagnostic{}
	for _, d := range AnalyzeWithOptions("a.sysml", source.KindOf("a.sysml"), root, pd, idx,
		Options{Conformance: diag.ConformanceStrict}) {
		if d.Code == CodeNonstandardNotation {
			got = append(got, d)
		}
	}
	if len(got) != 1 {
		t.Fatalf("strict analysis gave %d nonstandard-notation findings %+v, want 1", len(got), got)
	}
	if got[0].Severity != diag.SeverityError {
		t.Errorf("strict severity = %v, want error", got[0].Severity)
	}
}
