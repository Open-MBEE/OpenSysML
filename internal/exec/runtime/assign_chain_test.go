package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
)

// A chained target whose last segment names no feature of the object the chain
// reaches is reported against that object rather than silently ignored.
func testAssignChainUnknownFinalFeature(t *testing.T) {
	err := runChainWrite(t, `
		part def Sensor { attribute reading : Real = 0.0; }
		part s : Sensor;
		state Machine {
			entry; then go;
			state go;
			transition first go do assign s.nosuch := 4.5 then done;
		}
	`)
	if !errors.Is(err, ErrNoSuchFeature) {
		t.Fatalf("err = %v; want ErrNoSuchFeature", err)
	}
	if !strings.Contains(err.Error(), "s.nosuch") {
		t.Errorf("err = %v; want it to name the target", err)
	}
}

// A step of a chain that holds a value rather than an object has no feature to
// write, and is reported naming the target.
func testAssignChainStepIsNotAnObject(t *testing.T) {
	err := runChainWrite(t, `
		state Machine {
			attribute level : Real = 1.0;
			entry; then go;
			state go;
			transition first go do assign level.reading := 4.5 then done;
		}
	`)
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("err = %v; want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "level.reading") {
		t.Errorf("err = %v; want it to name the target", err)
	}
}

// A step holding several objects names no one object to write on, so the write is
// reported rather than applied to an arbitrary element.
func testAssignChainStepHoldsManyObjects(t *testing.T) {
	err := runChainWrite(t, `
		part def Sensor { attribute reading : Real = 0.0; }
		part def Rig { part bank : Sensor[3]; }
		part rig : Rig;
		state Machine {
			entry; then go;
			state go;
			transition first go do assign rig.bank.reading := 4.5 then done;
		}
	`)
	if !errors.Is(err, ErrTypeMismatch) {
		t.Fatalf("err = %v; want ErrTypeMismatch", err)
	}
	if !strings.Contains(err.Error(), "rig.bank.reading") {
		t.Errorf("err = %v; want it to name the target", err)
	}
}

// A value the target's multiplicity does not admit is rejected exactly as a
// direct write of the same value is.
func testAssignChainMultiplicityViolation(t *testing.T) {
	err := runChainWrite(t, `
		part def Sensor { attribute pair : Real[2] = (1.0, 2.0); }
		part s : Sensor;
		state Machine {
			entry; then go;
			state go;
			transition first go do assign s.pair := (1.0, 2.0, 3.0) then done;
		}
	`)
	if !errors.Is(err, ErrMultiplicityViolation) {
		t.Fatalf("err = %v; want ErrMultiplicityViolation", err)
	}
	if !strings.Contains(err.Error(), "s.pair") {
		t.Errorf("err = %v; want it to name the target", err)
	}
}

// A chain step holding no value reaches no object, so the write reports the unset
// step rather than materializing one.
func testAssignChainUnsetStep(t *testing.T) {
	err := runChainWrite(t, `
		part def Sensor { attribute reading : Real = 0.0; }
		state Machine {
			attribute probe : Sensor = null;
			entry; then go;
			state go;
			transition first go do assign probe.reading := 4.5 then done;
		}
	`)
	if !errors.Is(err, ErrUninitializedFeatureValue) {
		t.Fatalf("err = %v; want ErrUninitializedFeatureValue", err)
	}
	if !strings.Contains(err.Error(), "probe.reading") {
		t.Errorf("err = %v; want it to name the target", err)
	}
}

// A chain starting from a name the body cannot reach is reported as the
// unresolved reference it is.
func testAssignChainUnreachableBase(t *testing.T) {
	err := runChainWrite(t, `
		state Machine {
			entry; then go;
			state go;
			transition first go do assign missingPart.reading := 4.5 then done;
		}
	`)
	if !errors.Is(err, ErrUnresolvedReference) {
		t.Fatalf("err = %v; want ErrUnresolvedReference", err)
	}
	if !strings.Contains(err.Error(), "missingPart") {
		t.Errorf("err = %v; want it to name the unreachable step", err)
	}
}

// A calculation writes nothing outside its own body, so a chained target is
// rejected as an assignment to any name it does not declare is.
func testAssignChainRejectedInCalcBody(t *testing.T) {
	ctx, idx := contextForSource(t, `package test {
		part def Sensor { attribute reading : Real = 0.0; }
		part s : Sensor;
		calc def Reset {
			assign s.reading := 1.0;
			2.0
		}
	}`)
	_, err := ctx.InvokeCalc(lookupOne(t, idx, "test::Reset"), nil, nil)
	if !errors.Is(err, ErrCalcExternalAssignment) {
		t.Fatalf("err = %v; want ErrCalcExternalAssignment", err)
	}
	if !strings.Contains(err.Error(), "s.reading") {
		t.Errorf("err = %v; want it to name the target", err)
	}
}

// runChainWrite runs a machine whose transition effect writes through a chain and
// returns what the write failed with.
func runChainWrite(t *testing.T, body string) error {
	t.Helper()
	exec := stateExecutorForSource(t, "Machine", "package test {\n"+body+"\n}")
	err := exec.RunToCompletion()
	if err == nil {
		t.Fatal("run to completion succeeded; want the chained write to be reported")
	}
	return err
}

// TestChainedAssignmentBindsNoCalcOutput locks that a chained target writing a
// feature of another object does not count as the calc computing its own output
// of that name.
func TestChainedAssignmentBindsNoCalcOutput(t *testing.T) {
	outputs := []calcOutput{{Name: "x"}}
	chained := []lower.Statement{lower.Assign{Target: "x", Chain: &lower.AssignTarget{Text: "foo.x"}}}
	if assigned := assignedOutputs(chained, outputs, nil); len(assigned) != 0 {
		t.Errorf("assignedOutputs(chained) = %v; want none, since foo.x is another object's feature", assigned)
	}
	plain := []lower.Statement{lower.Assign{Target: "x"}}
	if assigned := assignedOutputs(plain, outputs, nil); !assigned["x"] {
		t.Errorf("assignedOutputs(plain) = %v; want x, which the body does bind", assigned)
	}
}
