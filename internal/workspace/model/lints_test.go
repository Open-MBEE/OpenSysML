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

// A sender held as its interface record still silences the lint: the record
// keeps the names its body sends.
func TestWorkspaceSignalLintReadsRecordedSenders(t *testing.T) {
	sender := []byte(`package S { attribute Strat; part def Sender { action a { send Strat to self; } } }`)
	loaded := NewWorkspace()
	loaded.OpenAll([]Input{{Name: "b.sysml", Content: sender, Version: 1}})
	loaded.DiagnosticsAll([]string{"b.sysml"})
	rec, err := loaded.InterfaceRecord("b.sysml")
	if err != nil {
		t.Fatal(err)
	}
	if got := rec.Scope.Gathered.SentSignals; len(got) != 1 || got[0] != "Strat" {
		t.Fatalf("the record keeps sent signals %v, want [Strat]", got)
	}

	ws := NewWorkspace()
	if err := ws.OpenRecorded(rec, sender); err != nil {
		t.Fatal(err)
	}
	ws.Open("a.sysml", []byte(lintModel), 1)
	if n := lintCount(ws.Diagnostics("a.sysml"), passes.CodeUndeclaredSignal); n != 0 {
		t.Fatalf("a recorded sender's signal kept %d finding(s)", n)
	}
	if !ws.Recorded("b.sysml") {
		t.Fatal("b.sysml was hydrated: the fixture does not exercise a recorded sender")
	}
}

const portLintDefs = `package D {
	item def Power; item def Fuel;
	port def PowerOut { out item p : Power; }
	port def FuelIn { in item f : Fuel; }
	part def Source { port power : PowerOut; }
	part def Sink { port fuel : FuelIn; }
	part def Return { port back : ~PowerOut; }
	part def Frame { port src : PowerOut; }
	interface def Mount { end a : PowerOut; end b : ~PowerOut; }
}`

// portLintCount is the port-type-mismatch findings on a user of portLintDefs,
// the definitions held parsed, or as their interface record when recorded.
func portLintCount(t *testing.T, user string, recorded bool) int {
	t.Helper()
	defs := []byte(portLintDefs)
	ws := NewWorkspace()
	if recorded {
		loaded := NewWorkspace()
		loaded.OpenAll([]Input{{Name: "d.sysml", Content: defs, Version: 1}})
		loaded.DiagnosticsAll([]string{"d.sysml"})
		rec, err := loaded.InterfaceRecord("d.sysml")
		if err != nil {
			t.Fatal(err)
		}
		if err := ws.OpenRecorded(rec, defs); err != nil {
			t.Fatal(err)
		}
	} else {
		ws.Open("d.sysml", defs, 1)
	}
	ws.Open("u.sysml", []byte(user), 1)
	n := lintCount(ws.Diagnostics("u.sysml"), passes.CodePortTypeMismatch)
	if recorded && !ws.Recorded("d.sysml") {
		t.Fatal("d.sysml was hydrated: the fixture does not exercise recorded definitions")
	}
	return n
}

// The port lint judges a connection the same whether the definitions it
// reaches are parsed or held as their interface record.
func TestWorkspacePortLintReadsRecordedDefinitions(t *testing.T) {
	cases := []struct {
		name, body string
		want       int
	}{
		{"unrelated ports", `connect s.power to k.fuel;`, 1},
		{"typed by an interface with port ends", `interface : Mount connect s.power to k.fuel;`, 0},
		{"a conjugated port", `part r : Return; connect s.power to r.back;`, 0},
		{"a redefinition inheriting an unrelated type", `part d : Frame { port :>> src; port q : FuelIn; connect src to q; }`, 1},
		{"a redefinition inheriting a conjugate type", `part d : Frame { port :>> src; port q : ~PowerOut; connect src to q; }`, 0},
	}
	for _, tc := range cases {
		user := "package U { private import D::*; part sys { part s : Source; part k : Sink; " + tc.body + " } }"
		for _, recorded := range []bool{false, true} {
			if n := portLintCount(t, user, recorded); n != tc.want {
				t.Errorf("%s (recorded %v): %d finding(s), want %d", tc.name, recorded, n, tc.want)
			}
		}
	}
}
