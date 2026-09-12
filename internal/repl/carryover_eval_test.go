package repl

import (
	"strings"
	"testing"
)

// writtenHolderModel is a part whose feature an action writes, so the object
// holds a value its declaration does not state.
const writtenHolderModel = `package Demo {
    private import ScalarValues::*;
    part def Holder {
        attribute n : Integer;
    }
    part holder : Holder;
    action Writer {
        action write { assign holder.n := 5; }
        first start;
        then write;
        then done;
    }
}`

// writtenGridModel writes an array into a feature of the holder, so a member of
// the array is reached through the carried object.
const writtenGridModel = `package Demo {
    private import ScalarValues::*;
    private import Collections::*;
    attribute def LabeledGrid :> Array { attribute label : String; }
    attribute grid : LabeledGrid { :>> dimensions = (2, 2); :>> elements = (1, 2, 3, 4); :>> label = "grid"; }
    part def Holder {
        attribute n : Integer;
        attribute cells : LabeledGrid;
    }
    part holder : Holder;
    action Writer {
        action write { assign holder.n := 5; assign holder.cells := grid; }
        first start;
        then write;
        then done;
    }
}`

// submitModel submits src, failing the test when it does not parse.
func submitModel(t *testing.T, s *Session, src string) {
	t.Helper()
	if res := s.Submit(src); hasSyntaxError(res) {
		t.Fatalf("model does not parse: %v", res.Diagnostics)
	}
}

// metaOK runs a meta-command that must succeed and returns what it printed.
func metaOK(t *testing.T, s *Session, cmd string) string {
	t.Helper()
	out, _, err := s.runMeta(cmd)
	if err != nil {
		t.Fatalf("%s: %v (%v)", cmd, err, out)
	}
	return strings.Join(out, "\n")
}

// evalOK evaluates expr at the prompt and returns what it printed.
func evalOK(t *testing.T, s *Session, expr string) string {
	t.Helper()
	lines, err := s.EvalExpr(expr)
	if err != nil {
		t.Fatalf("%%eval %s: %v", expr, err)
	}
	return strings.Join(lines, "\n")
}

// writeHolder loads model, materializes Demo::holder and runs Demo::Writer to
// completion, so the object holds what the action wrote.
func writeHolder(t *testing.T, s *Session, model string) {
	t.Helper()
	submitModel(t, s, model)
	for _, cmd := range []string{"%instantiate Demo::holder", "%action Demo::Writer", "%continue"} {
		metaOK(t, s, cmd)
	}
}

// A value an action wrote is the object's, so after an unrelated declaration
// re-analyzes the document, every surface reading the carried object reads it:
// %eval answers what %features lists, about the one object held before.
func TestEvalReadsTheWrittenValueOfACarriedObject(t *testing.T) {
	s := NewSession()
	writeHolder(t, s, writtenHolderModel)
	before, _ := s.heldObject("Demo::holder")
	if got := evalOK(t, s, "Demo::holder.n"); !strings.Contains(got, "= 5") {
		t.Fatalf("%%eval Demo::holder.n before the declaration = %q, want 5", got)
	}
	submitModel(t, s, "part def Widget;")
	after, ok := s.heldObject("Demo::holder")
	if !ok || after != before {
		t.Fatalf("Demo::holder is %p after the submission, was %p", after, before)
	}

	if got := metaOK(t, s, "%features Demo::holder"); !strings.Contains(got, "n = 5") {
		t.Fatalf("%%features Demo::holder = %q, want n = 5", got)
	}
	if got := evalOK(t, s, "Demo::holder.n"); !strings.Contains(got, "= 5") {
		t.Errorf("%%eval Demo::holder.n = %q, want the written value 5 that %%features lists", got)
	}
	if got := evalOK(t, s, "Demo::holder === Demo::holder"); !strings.Contains(got, "= true") {
		t.Errorf("Demo::holder === Demo::holder = %q, want true", got)
	}
	if s.heldObjects() != 1 {
		t.Errorf("the session holds %d objects, want the carried one alone", s.heldObjects())
	}
}

// A member of an object reached through a carried object's written value is read
// through the carried object too.
func TestEvalReachesThroughACarriedObjectsWrittenValue(t *testing.T) {
	s := NewSession()
	writeHolder(t, s, writtenGridModel)
	submitModel(t, s, "part def Widget;")

	if got := metaOK(t, s, "%features Demo::holder"); !strings.Contains(got, "cells = Array(2, 2)[1, 2, 3, 4]") {
		t.Fatalf("%%features Demo::holder = %q, want the written array", got)
	}
	for expr, want := range map[string]string{
		"Demo::holder.n":           "= 5",
		"Demo::holder.cells.rank":  "= 2",
		"Demo::holder.cells.label": `= "grid"`,
	} {
		if got := evalOK(t, s, expr); !strings.Contains(got, want) {
			t.Errorf("%%eval %s = %q, want %s", expr, got, want)
		}
	}
}

// An action stepped at the prompt writes to the same object every surface reads
// after a declaration re-analyzes the document under it.
func TestADebuggerWritesTheObjectEvalReadsAfterAReanalysis(t *testing.T) {
	s := NewSession()
	submitModel(t, s, writtenHolderModel)
	metaOK(t, s, "%instantiate Demo::holder")
	metaOK(t, s, "%action Demo::Writer")
	submitModel(t, s, "part def Widget;")
	metaOK(t, s, "%continue")

	if got := metaOK(t, s, "%features Demo::holder"); !strings.Contains(got, "n = 5") {
		t.Fatalf("%%features Demo::holder = %q, want n = 5", got)
	}
	if got := evalOK(t, s, "Demo::holder.n"); !strings.Contains(got, "= 5") {
		t.Errorf("%%eval Demo::holder.n = %q, want the 5 the debugger wrote", got)
	}
}

// A resubmission that changes the holder's declaration drops the object, so the
// written value is gone from every surface alike: %features says the object was
// dropped and %eval, back at model level, finds the declaration gives no value.
func TestSupersedingTheHolderDropsTheWrittenValueEverywhere(t *testing.T) {
	s := NewSession()
	writeHolder(t, s, writtenHolderModel)
	submitModel(t, s, strings.Replace(writtenHolderModel, "attribute n : Integer;", "attribute n : Integer; attribute m : Integer;", 1))

	if _, held := s.heldObject("Demo::holder"); held {
		t.Fatal("Demo::holder was carried over a change to its declaration")
	}
	if got := metaOK(t, s, "%features Demo::holder"); !strings.Contains(got, "1 instance was dropped when the declarations changed") {
		t.Errorf("%%features Demo::holder = %q, want the object reported dropped", got)
	}
	if got := evalOK(t, s, "Demo::holder.n"); !strings.Contains(got, "= <undetermined>") {
		t.Errorf("%%eval Demo::holder.n = %q, want <undetermined>", got)
	}
}
