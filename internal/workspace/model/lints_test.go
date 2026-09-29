package model

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

const lintModel = `package P {
	state def Machine {
		entry; then idle;
		state idle; state busy;
		transition first idle when Strat then busy;
	}
}`

func lintCount(diags []diag.Diagnostic, code string) int {
	n := 0
	for _, d := range diags {
		if d.Code == code {
			n++
		}
	}
	return n
}

func TestWorkspaceDisablesALint(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte(lintModel), 1)
	if lintCount(ws.Diagnostics("a.sysml"), passes.CodeUndeclaredSignal) != 1 {
		t.Fatalf("want the lint on by default: %v", ws.Diagnostics("a.sysml"))
	}
	if err := ws.SetDisabledLints([]string{passes.CodeUndeclaredSignal}); err != nil {
		t.Fatal(err)
	}
	if n := lintCount(ws.Diagnostics("a.sysml"), passes.CodeUndeclaredSignal); n != 0 {
		t.Fatalf("Diagnostics kept %d disabled finding(s)", n)
	}
	if _, diags, _ := ws.AnalyzedContent("a.sysml"); lintCount(diags, passes.CodeUndeclaredSignal) != 0 {
		t.Fatal("AnalyzedContent kept a disabled finding")
	}
	if all := ws.DiagnosticsAll([]string{"a.sysml"}); lintCount(all[0], passes.CodeUndeclaredSignal) != 0 {
		t.Fatal("DiagnosticsAll kept a disabled finding")
	}
	if got := ws.DisabledLints(); len(got) != 1 || got[0] != passes.CodeUndeclaredSignal {
		t.Fatalf("DisabledLints = %v", got)
	}
	if err := ws.SetDisabledLints(nil); err != nil {
		t.Fatal(err)
	}
	if lintCount(ws.Diagnostics("a.sysml"), passes.CodeUndeclaredSignal) != 1 {
		t.Fatal("re-enabling the lint did not bring its finding back")
	}
}

func TestWorkspaceRejectsAnUnknownLint(t *testing.T) {
	ws := NewWorkspace(WithDisabledLints(passes.CodePortTypeMismatch))
	if err := ws.SetDisabledLints([]string{"no-such-lint"}); err == nil {
		t.Fatal("an unknown code was accepted")
	}
	if got := ws.DisabledLints(); len(got) != 1 || got[0] != passes.CodePortTypeMismatch {
		t.Fatalf("a rejected setting changed the lints: %v", got)
	}
}

// A signal another open document sends silences the lint, and closing that
// document brings it back: the sent names are a workspace-wide union.
func TestWorkspaceSignalLintFollowsOtherDocuments(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("a.sysml", []byte(lintModel), 1)
	if lintCount(ws.Diagnostics("a.sysml"), passes.CodeUndeclaredSignal) != 1 {
		t.Fatal("want the lint before anything sends Strat")
	}
	ws.Open("b.sysml", []byte(`package S { attribute Strat; part def Sender { action a { send Strat to self; } } }`), 1)
	if n := lintCount(ws.Diagnostics("a.sysml"), passes.CodeUndeclaredSignal); n != 0 {
		t.Fatalf("a sent signal kept %d finding(s)", n)
	}
	ws.Close("b.sysml")
	if lintCount(ws.Diagnostics("a.sysml"), passes.CodeUndeclaredSignal) != 1 {
		t.Fatal("closing the sender did not bring the finding back")
	}
}
