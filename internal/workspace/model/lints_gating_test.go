package model

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// A syntax error elsewhere in the document does not silence the signal lint for
// a well-formed trigger: the lint gates on its own subject, not the whole document.
func TestSignalLintSurvivesAnErrorElsewhereInTheDocument(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte(lintModel+"\npackage Broken { part x : ; }\n"), 1)
	diags := ws.Diagnostics("a.sysml")
	blocking := 0
	for _, d := range diags {
		if d.Severity == diag.SeverityError {
			blocking++
		}
	}
	if blocking == 0 {
		t.Fatalf("the fixture has no error to gate on: %v", diags)
	}
	if n := lintCount(diags, passes.CodeUndeclaredSignal); n != 1 {
		t.Fatalf("undeclared-signal findings = %d, want 1: %v", n, diags)
	}
}
