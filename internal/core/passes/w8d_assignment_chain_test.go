package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

// assignmentChainFindings collects every diagnostic reported for a chained
// assignment target, whatever its code.
func assignmentChainFindings(t *testing.T, src string, warm bool) []diag.Diagnostic {
	t.Helper()
	var out []diag.Diagnostic
	for _, d := range w9cLibraryDiags(t, src, warm) {
		if d.Severity == diag.SeverityError {
			out = append(out, d)
		}
	}
	return out
}

// The model that used to pass -validate and then fail to execute now analyses
// clean and runs (its execution is `assign_chain_state_entry_guard_reads`).
func TestAssignmentChainWritableTargetAnalysesClean(t *testing.T) {
	src := `package Test {
	part def Sensor { attribute reading : ScalarValues::Real = 0.0; }
	part def Rig {
		part s : Sensor;
		exhibit state run {
			entry; then go;
			state go {
				entry action set { assign s.reading := 4.5; }
			}
		}
	}
}`
	for _, warm := range []bool{false, true} {
		if got := assignmentChainFindings(t, src, warm); len(got) != 0 {
			t.Fatalf("warm=%v: got %v, want a writable chain target to analyse clean", warm, got)
		}
	}
}

// A calculation writes no feature of another object, so a chained target is an
// error of the body rather than a run-time surprise.
func TestAssignmentChainInCalcBodyIsReported(t *testing.T) {
	src := `package Test {
	part def Sensor { attribute reading : ScalarValues::Real = 0.0; }
	part s : Sensor;
	calc def Reset {
		assign s.reading := 1.0;
		2.0
	}
}`
	for _, warm := range []bool{false, true} {
		got := assignmentChainFindings(t, src, warm)
		if len(got) != 1 || got[0].Code != "assignment-chain-in-calc" {
			t.Fatalf("warm=%v: got %v, want the calculation chain-target diagnostic", warm, got)
		}
		if want := strings.Index(src, "s.reading := 1.0"); got[0].Span.Offset != want {
			t.Errorf("warm=%v: span offset %d, want %d", warm, got[0].Span.Offset, want)
		}
		if !strings.Contains(got[0].Message, "s.reading") {
			t.Errorf("warm=%v: message %q, want it to name the target", warm, got[0].Message)
		}
	}
}

// A step that may hold several objects names no one object to write, which the
// declared multiplicity already says.
func TestAssignmentChainStepHoldingManyIsReported(t *testing.T) {
	src := `package Test {
	part def Sensor { attribute reading : ScalarValues::Real = 0.0; }
	part def Rig {
		part bank : Sensor[3];
		exhibit state run {
			entry; then go;
			state go {
				entry action set { assign bank.reading := 4.5; }
			}
		}
	}
}`
	for _, warm := range []bool{false, true} {
		got := assignmentChainFindings(t, src, warm)
		if len(got) != 1 || got[0].Code != "assignment-chain-step-not-one-object" {
			t.Fatalf("warm=%v: got %v, want the many-objects diagnostic", warm, got)
		}
		if !strings.Contains(got[0].Message, "bank") {
			t.Errorf("warm=%v: message %q, want it to name the step", warm, got[0].Message)
		}
	}
}

// A final segment naming no feature of the type the chain reaches is reported by
// name resolution, so the chain check adds nothing and stays silent.
func TestAssignmentChainUnknownFinalSegmentIsReported(t *testing.T) {
	src := `package Test {
	part def Sensor { attribute reading : ScalarValues::Real = 0.0; }
	part def Rig {
		part s : Sensor;
		exhibit state run {
			entry; then go;
			state go {
				entry action set { assign s.nosuch := 4.5; }
			}
		}
	}
}`
	for _, warm := range []bool{false, true} {
		got := assignmentChainFindings(t, src, warm)
		if len(got) != 1 || !strings.Contains(got[0].Message, "nosuch") {
			t.Fatalf("warm=%v: got %v, want the unresolved member diagnostic", warm, got)
		}
	}
}
