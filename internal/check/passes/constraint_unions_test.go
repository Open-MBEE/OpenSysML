package passes

import (
	"strings"
	"testing"
)

// A union's instances are exactly those of its unioning types, so a union
// conforms to every type all of them conform to (KerML 1.0 §8.3.3).
func TestConstraintRedefinitionConformsThroughUnion(t *testing.T) {
	const src = `
		classifier Wheel;
		classifier MyWheel1 specializes Wheel;
		classifier MyWheel2 specializes Wheel;
		classifier MyWheel unions MyWheel1, MyWheel2;
		classifier Vehicle { feature rollsOn : Wheel; }
		classifier MyVehicle specializes Vehicle {
			feature redefines rollsOn : MyWheel;
		}`
	for _, name := range []string{"a.kerml", "a.sysml"} {
		diags := diagsIn(t, name, src, "constraint")
		for _, d := range diags {
			if d.Code == "redefinition-type-mismatch" {
				t.Errorf("%s: unexpected diagnostic: %s", name, d.Message)
			}
		}
	}
}

// A union whose constituents do not all conform is still a mismatch: unioning
// widens conformance only as far as every unioning type reaches.
func TestConstraintRedefinitionUnionMemberDoesNotConform(t *testing.T) {
	const src = `
		classifier Wheel;
		classifier Sprocket;
		classifier MyWheel1 specializes Wheel;
		classifier MyWheel unions MyWheel1, Sprocket;
		classifier Vehicle { feature rollsOn : Wheel; }
		classifier MyVehicle specializes Vehicle {
			feature redefines rollsOn : MyWheel;
		}`
	diags := diagsIn(t, "a.kerml", src, "constraint")
	if !hasCode(diags, "redefinition-type-mismatch") {
		t.Fatalf("expected redefinition-type-mismatch, got %v", diags)
	}
	for _, d := range diags {
		if d.Code == "redefinition-type-mismatch" &&
			!strings.Contains(d.Message, "types do not conform") {
			t.Errorf("got %q", d.Message)
		}
	}
}

// A union that names itself terminates rather than recursing forever.
func TestConstraintRedefinitionUnionCycleTerminates(t *testing.T) {
	const src = `
		classifier Wheel;
		classifier MyWheel unions MyOtherWheel;
		classifier MyOtherWheel unions MyWheel;
		classifier Vehicle { feature rollsOn : Wheel; }
		classifier MyVehicle specializes Vehicle {
			feature redefines rollsOn : MyWheel;
		}`
	if !hasCode(diagsIn(t, "a.kerml", src, "constraint"), "redefinition-type-mismatch") {
		t.Fatal("expected redefinition-type-mismatch for a union that reaches no type")
	}
}
