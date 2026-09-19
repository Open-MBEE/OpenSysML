package repl

import (
	"strings"
	"testing"
)

const standaloneBodyModel = `package P {
	private import ScalarValues::*;
	action def Touch {
		action step { assign touched := touched + 1; }
		first step;
	}
	part def Host {
		attribute touched : Integer = 0;
		perform action t : Touch;
	}
	part host : Host;
}`

const inlineBodyModel = `package Q {
	private import ScalarValues::*;
	part def Host {
		attribute touched : Integer = 0;
		perform action t {
			action step { assign this.touched := this.touched + 1; }
			first step;
		}
	}
	part host : Host;
}`

// What the session reports on submission and what it reports on execution are
// one verdict: a name only the performing object declares is refused by both.
func TestStandaloneBodyNamingThePerformersFeatureIsRefusedBySubmitAndByExecution(t *testing.T) {
	s := NewSession()
	diags := errorDiagnostics(s.Submit(standaloneBodyModel).Diagnostics)
	if len(diags) == 0 {
		t.Fatalf("submit reported no error, want an unresolved reference for touched")
	}
	for _, d := range diags {
		if !strings.Contains(d.Message, "touched") {
			t.Errorf("diagnostic %q, want it to name touched", d.Message)
		}
	}

	out, _, err := s.RunMeta("%instantiate P::host")
	report := strings.Join(out, "\n")
	if err != nil {
		report = err.Error()
	}
	if !strings.Contains(report, "touched") {
		t.Errorf("instantiate reported %q, want the same name refused at run time", report)
	}
}

// A body written in the part performing it is accepted by both.
func TestInlineBodyNamingThePerformerThroughThisIsAcceptedBySubmitAndByExecution(t *testing.T) {
	s := NewSession()
	if diags := errorDiagnostics(s.Submit(inlineBodyModel).Diagnostics); len(diags) != 0 {
		t.Fatalf("submit reported %v, want no errors", diags)
	}
	if out := run(t, s, "%instantiate Q::host"); !strings.Contains(out, "Created instance") {
		t.Fatalf("instantiate output = %q, want the instance created", out)
	}
	wants(t, run(t, s, "%features Q::host"), "touched = 1")
}
