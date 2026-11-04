package passes

import "testing"

// A union's instances are exactly those of its unioning types (KerML 1.1
// §7.3.2.3), so a union of types all disjoint from Wheel is disjoint from it.
func TestConstraintUnionOfDisjointTypesIsDisjoint(t *testing.T) {
	const src = `
		classifier Wheel;
		classifier Rock disjoint from Wheel;
		classifier Rock1 specializes Rock;
		classifier Rock2 specializes Rock;
		classifier Rocks unions Rock1, Rock2;
		classifier Vehicle { feature rollsOn : Wheel; }
		classifier MyVehicle specializes Vehicle {
			feature redefines rollsOn : Rocks;
		}`
	for _, name := range []string{"a.kerml", "a.sysml"} {
		wantDisjointReport(t, disjointReports(t, name, src), "rollsOn", "Rocks", "Wheel")
	}
}

// A union with one unioning type that may share instances with Wheel is not
// disjoint from it, whatever the others are.
func TestConstraintUnionWithSharingMemberIsNotDisjoint(t *testing.T) {
	const src = `
		classifier Wheel;
		classifier Rock disjoint from Wheel;
		classifier MyWheel1 specializes Wheel;
		classifier Mixed unions Rock, MyWheel1;
		classifier Vehicle { feature rollsOn : Wheel; }
		classifier MyVehicle specializes Vehicle {
			feature redefines rollsOn : Mixed;
		}`
	if diags := disjointReports(t, "a.kerml", src); len(diags) != 0 {
		t.Fatalf("expected no report, got %v", diags)
	}
}

// Unioning cycles terminate: a union that reaches no type of its own establishes
// nothing, so it is neither disjoint nor conforming.
func TestConstraintUnionCycleTerminates(t *testing.T) {
	const src = `
		classifier Wheel;
		classifier Rock disjoint from Wheel;
		classifier Rock1 specializes Rock;
		classifier Rocks unions Rock, Other;
		classifier Other unions Rocks, Rock1;
		classifier Vehicle { feature rollsOn : Wheel; }
		classifier MyVehicle specializes Vehicle {
			feature redefines rollsOn : Rocks;
		}`
	if diags := disjointReports(t, "a.kerml", src); len(diags) != 0 {
		t.Fatalf("expected no report, got %v", diags)
	}
}

// A disjoining from a union excludes every unioning type, since the union holds
// each one's instances.
func TestConstraintDisjoiningFromUnionExcludesItsMembers(t *testing.T) {
	const src = `
		class A; class B;
		class U unions A, B;
		class C disjoint from U;
		class A1 :> A;
		class X { feature f : A, C; }
		class Y { feature g : A1, C; }`
	diags := disjointReports(t, "a.kerml", src)
	if len(diags) != 2 {
		t.Fatalf("reports = %d, want f and g: %v", len(diags), diags)
	}
}

// A subtype of a union holds a subset of the union's instances, so it inherits
// the union's disjointness.
func TestConstraintSubtypeOfDisjointUnion(t *testing.T) {
	const src = `
		class Wheel;
		class Rock disjoint from Wheel;
		class Stone disjoint from Wheel;
		class Rocks unions Rock, Stone;
		class Pebbles specializes Rocks;
		class X { feature f : Pebbles, Wheel; }`
	wantDisjointReport(t, disjointReports(t, "a.kerml", src), "f", "Pebbles", "Wheel")
}
