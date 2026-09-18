package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// analyzeAll is diagnostics from every pass, since a residue is adjudicated by
// what the whole analysis says about a file rather than by one pass.
func analyzeAll(t *testing.T, name, src string) []diag.Diagnostic {
	t.Helper()
	root := parser.New(source.New(name, []byte(src))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument(name, root)
	return Analyze(name, root, nil, idx)
}

// TestW8GNodeBodiesAnalyseClean is F62's residue: a transition with an accept
// trigger and a body, and a send with a body, analyse clean, as the pinned
// validator also accepts them.
func TestW8GNodeBodiesAnalyseClean(t *testing.T) {
	diags := analyzeAll(t, "f62.sysml", `package F62 {
	part def Vehicle;
	port def P;
	item def Msg;
	action def SendM { in item m : Msg; }
	state def VehicleStates {
		entry; then off;
		state off;
		state on;
		transition off_to_on first off accept Msg then on {
			doc /* a transition body */
		}
	}
	action def Send {
		port p : P;
		part receiver : Vehicle;
		item msg : Msg;
		action a {
			send SendM(msg) via p to receiver {
				doc /* a send body */
			}
		}
	}
}`)
	for _, d := range diags {
		if d.Severity == diag.SeverityError {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

// TestW8GGuardedSuccessionEndpointsResolveAsActions pins action-body succession.
func TestW8GGuardedSuccessionEndpointsResolveAsActions(t *testing.T) {
	diags := analyzeAll(t, "guarded.sysml", `action def Decide {
	attribute x = 1;
	action A1;
	action A2;
	succession S first A1 if x == 0 then A2;
}`)
	for _, d := range diags {
		if d.Severity == diag.SeverityError {
			t.Errorf("action succession produced an error: %s", d.Message)
		}
	}
}

// TestW8GInterfaceConjugationStaysAWarning is the interface row's residue: the
// check is a one-sided warning, since the pinned validator reports nothing at
// all on an interface whose ends carry the same direction.
func TestW8GInterfaceConjugationStaysAWarning(t *testing.T) {
	diags := analyzeAll(t, "iface.sysml", `package IF {
	attribute def Level;
	port def Sensor { out attribute reading : Level; }
	port def Reader { out attribute reading : Level; }
	interface def Link {
		end supplier : Sensor;
		end consumer : Reader;
	}
}`)
	found := false
	for _, d := range diags {
		if !strings.Contains(d.Message, "not conjugate") {
			continue
		}
		found = true
		if d.Severity != diag.SeverityWarning {
			t.Errorf("conjugation mismatch is %v, want a warning", d.Severity)
		}
	}
	if !found {
		t.Error("no conjugation diagnostic")
	}
}

func TestW8GVerifyRedefinesInheritedObjective(t *testing.T) {
	diags := analyzeAll(t, "f66.sysml", `package F66 {
	requirement def MassReq;
	verification def MassVerification {
		objective { verify requirement massRequirement : MassReq; }
	}
	requirement vehicleMassRequirement : MassReq;
	verification vehicleMassVerification : MassVerification {
		objective { verify vehicleMassRequirement :>> massRequirement; }
	}
}`)
	var unresolved []string
	for _, d := range diags {
		if strings.Contains(d.Message, "unresolved reference") {
			unresolved = append(unresolved, d.Message)
		}
	}
	if len(unresolved) != 0 {
		t.Fatalf("inherited objective member is unresolved: %v", unresolved)
	}
}

func TestW8GObjectiveDoesNotInheritSubjectMembers(t *testing.T) {
	diags := analyzeAll(t, "role-negative.sysml", `package Roles {
	verification def Base {
		subject s { attribute subjectOnly; }
		objective o;
	}
	verification derived : Base {
		objective {
			attribute :>> subjectOnly;
		}
	}
}`)
	for _, d := range diags {
		if strings.Contains(d.Message, "unresolved reference: subjectOnly") {
			return
		}
	}
	t.Fatalf("expected subjectOnly to remain unresolved, got %v", diags)
}
