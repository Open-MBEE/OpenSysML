package passes

import (
	"strings"
	"testing"
)

// An individual definition is an occurrence definition that individuates the
// definition it specializes (SysML v2 §7.9.4), so specializing an occurrence
// definition of any kind is well formed.
func TestTypeCheckIndividualDefSpecializesOccurrenceDefsOK(t *testing.T) {
	for _, src := range []string{
		"part def Vehicle; individual def TestVehicle :> Vehicle;",
		"occurrence def Flight; individual def Flight248 :> Flight;",
		"item def Fuel; individual def TankLoad :> Fuel;",
		"action def Fly; individual def Fly1 :> Fly;",
		"individual def Vehicle1; individual def Vehicle1Again :> Vehicle1;",
	} {
		if diags := typeDiags(t, src); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics, got %v", src, diags)
		}
	}
}

// An occurrence definition cannot specialize an attribute definition, because
// Occurrences::Occurrence is disjoint with Base::DataValues (SysML v2 §8.4.5.1).
func TestTypeCheckIndividualDefSpecializesDataTypeError(t *testing.T) {
	for _, src := range []string{
		"attribute def Mass; individual def Bad :> Mass;",
		"enum def Level; individual def AlsoBad :> Level;",
	} {
		diags := typeDiags(t, src)
		if len(diags) != 1 {
			t.Fatalf("%s: expected exactly one type diagnostic, got %v", src, diags)
		}
		if diags[0].Message != "Cannot specialize attribute definition" {
			t.Errorf("%s: unexpected message %q", src, diags[0].Message)
		}
	}
}

// A usage may be typed by an individual definition wherever it may be typed by
// an occurrence definition (SysML v2 §7.9.4).
func TestTypeCheckTypedByIndividualDefOK(t *testing.T) {
	src := `
		package P {
			part def Vehicle;
			individual def TestVehicle :> Vehicle;
			individual def TestSystem;
			part def Sys {
				item cargo : TestVehicle;
				occurrence o : TestVehicle;
				individual i : TestSystem;
				action collectData {
					in subject : TestVehicle;
					out result : TestVehicle;
				}
			}
		}
	`
	if diags := typeDiags(t, src); len(diags) != 0 {
		t.Fatalf("expected no type diagnostics for typing by an individual definition, got %v", diags)
	}
}

// Typing by an individual definition is allowed exactly where typing by an
// occurrence definition is: a port usage is still typed by a port definition
// only.
func TestTypeCheckPortTypedByIndividualDefError(t *testing.T) {
	src := `
		package P {
			part def Vehicle;
			individual def TestVehicle :> Vehicle;
			part def Sys {
				port p : TestVehicle;
			}
		}
	`
	diags := typeDiags(t, src)
	if len(diags) != 1 {
		t.Fatalf("expected exactly one type diagnostic, got %v", diags)
	}
	if diags[0].Message != "A port must be typed by port definitions." {
		t.Errorf("unexpected message %q", diags[0].Message)
	}
}

// The `individual` and `snapshot` modifiers make a usage an occurrence usage
// whatever kind keyword declares it (SysML v2 §8.3.9.11), so it may be typed by
// an occurrence definition of any kind — including one its kind keyword alone
// would reject.
func TestTypeCheckOccurrenceModifierWidensTypingOK(t *testing.T) {
	for _, src := range []string{
		"individual def TestSystem; individual testSystem : TestSystem;",
		"occurrence def Flight; snapshot occurrence takeoff : Flight;",
		"part def Vehicle; snapshot part vehicleAtTakeoff : Vehicle;",
		"action def Fly; individual flight : Fly;",
		"port def Sensing; snapshot occurrence sensing : Sensing;",
		"occurrence def Flight; action collect { in individual subject : Flight; }",
	} {
		// The separate individual-definition requirement is checked by
		// W10BIndividualTypingPass, as it is by a separate reference constraint.
		if diags := except(typeDiags(t, src), "individual-typing"); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics, got %v", src, diags)
		}
	}
}

// An occurrence usage may not be typed by a data type: Occurrences::Occurrence
// is disjoint with Base::DataValues (SysML v2 §8.4.5.1). The same declaration
// without the modifier is an attribute usage and is well formed, which is what
// makes the modifier observable.
func TestTypeCheckOccurrenceModifierRejectsDataType(t *testing.T) {
	for _, tt := range []struct {
		src  string
		want string
	}{
		{"attribute def Mass; individual m : Mass;", "individual usage cannot be typed by attributeDef"},
		{"enum def Level; individual l : Level;", "individual usage cannot be typed by enumDef"},
		{"attribute def Mass; snapshot occurrence m : Mass;", "snapshot usage cannot be typed by attributeDef"},
		{"attribute def Mass; action collect { in individual m : Mass; }", "individual usage cannot be typed by attributeDef"},
	} {
		diags := except(typeDiags(t, tt.src), "individual-typing")
		if len(diags) != 1 {
			t.Fatalf("%s: expected exactly one type diagnostic, got %v", tt.src, diags)
		}
		if !strings.Contains(diags[0].Message, tt.want) {
			t.Errorf("%s: unexpected message %q", tt.src, diags[0].Message)
		}
	}
}

// Without the modifier the same typings are accepted by the type checker, so
// the diagnostics above come from the modifier and not from the kind keyword.
// `occurrence m : Mass;` is rejected instead by W8DOccurrenceTypingPass, which
// the pilot reports too (OccurrenceUsage_invalid.sysml.xt).
func TestTypeCheckDataTypeTypingWithoutModifierOK(t *testing.T) {
	for _, src := range []string{
		"attribute def Mass; attribute m : Mass;",
		"enum def Level; attribute l : Level;",
		"attribute def Mass; action collect { in m : Mass; }",
	} {
		if diags := typeDiags(t, src); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics, got %v", src, diags)
		}
	}
}

// The shape that the OMG training corpus exercises in
// "34. Verification/Verification Case Usage Example": individual definitions of
// a part definition, and an individual usage of one of them that redefines a
// part usage.
func TestTypeCheckIndividualsVerificationCaseShape(t *testing.T) {
	src := `
		package P {
			part def Vehicle;
			part def MassVerificationSystem;
			part massVerificationSystem : MassVerificationSystem;

			individual def TestSystem :> MassVerificationSystem;
			individual def TestVehicle1 :> Vehicle;

			individual testSystem : TestSystem :> massVerificationSystem {
				timeslice test1 {
					action collectData {
						in individual :>> testVehicle : TestVehicle1;
					}
				}
			}
		}
	`
	if diags := typeDiags(t, src); len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}
