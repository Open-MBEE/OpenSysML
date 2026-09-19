package passes

import "testing"

// Constraint-tier rules the pinned pilot reports and we do not, adjudicated as
// P6 in docs/project/pilot-differential.md (F20-F23).

// F20 validateSubsettingFeaturingTypes: `subsettingFeature.canAccess(subsettedFeature)`
// — a feature of a type is not reachable by `::` from outside it.
// The rule constrains a Subsetting, so a nested feature named from outside its
// type is reported where it is subsetted, not where it is merely referenced.
func TestConstraintSubsettingFeaturingTypes(t *testing.T) {
	const src = `package E {
		part def P { attribute n; }
		part def R { attribute m :> E::P::n; }
	}`
	if !hasCode(constraintDiags(t, src), "subsetting-featuring-types") {
		t.Fatalf("expected an inaccessible-feature diagnostic for E::P::n")
	}
	const referenceOnly = `package E {
		part def P { attribute n = 1; }
		package Q { filter E::P::n > 0; }
	}`
	if hasCode(constraintDiags(t, referenceOnly), "subsetting-featuring-types") {
		t.Fatalf("a filter condition is not a subsetting")
	}
}

// F21 validateFlowEndSubsetting: each flow end must name the feature the payload
// leaves from or arrives at, so it can subset it. Naming the part alone cannot.
func TestConstraintFlowEndSubsettingNotImplemented(t *testing.T) {
	const src = `package G {
	part def Fuel;
	part def Tank { out item fuelOut : Fuel; }
	part def Thruster { in item fuelIn : Fuel; }
	part def Sys {
		part tank : Tank;
		part thruster : Thruster;
		flow of Fuel from tank to thruster;
	}
}`
	n := 0
	for _, d := range constraintDiags(t, src) {
		if d.Code == "flow-end-subsetting" {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("expected one flow-end diagnostic per undotted end, got %d", n)
	}
}

// F21's accepted counterpart: dotted ends name features of the parts, so the
// flow is well formed — this must keep holding when F21 lands.
func TestConstraintDottedFlowEndsAreAccepted(t *testing.T) {
	const src = `package G {
	part def Fuel;
	part def Tank { out item fuelOut : Fuel; }
	part def Thruster { in item fuelIn : Fuel; }
	part def Sys {
		part tank : Tank;
		part thruster : Thruster;
		flow of Fuel from tank.fuelOut to thruster.fuelIn;
	}
}`
	if diags := constraintDiags(t, src); len(diags) != 0 {
		t.Fatalf("dotted flow ends are valid SysML v2, got %v", diags)
	}
}

// F22 validateElementFilterMembershipIsModelLevelEvaluable, our side too narrow:
// an invocation of a model-level-evaluable library function over literals is fine.
func TestFilterModelLevelEvaluableFalsePositive(t *testing.T) {
	const src = `package H { package Q { filter 1 + 2 > 0; } }`
	if diags := only(filterDiags(t, src), "filter-not-evaluable"); len(diags) != 0 {
		t.Fatalf("`1 + 2 > 0` is model-level evaluable, got %v", diags)
	}
}

// F22 the other way: a referent with a featuring type is not evaluable (a
// top-level `part p : P` has none, so `filter p.n > 1` is — featuring, not constancy).
func TestFilterModelLevelEvaluableFalseNegative(t *testing.T) {
	const src = `package ScalarValues { attribute def Integer; }
	package E {
		private import ScalarValues::*;
		part def P { attribute n : Integer = 1; }
		package Q { filter E::P::n > 0; }
	}`
	if diags := only(filterDiags(t, src), "filter-not-evaluable"); len(diags) != 1 {
		t.Fatalf("a reference to a featured feature is not evaluable, got %v", diags)
	}
}

// F23 validateInvocationExpressionInstantiatedType: what is invoked must be a
// Behavior, or a Feature typed by exactly one Behavior. A part definition is neither.
func TestTypeCheckInvocationInstantiatedTypeNotImplemented(t *testing.T) {
	const src = `package C {
		part def Widget;
		part w = Widget();
	}`
	if !hasCode(f23AllDiags(t, src), "invocation-not-behavior") {
		t.Fatalf("expected an invocation-target diagnostic for Widget()")
	}
}
