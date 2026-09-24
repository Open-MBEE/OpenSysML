package passes

import "testing"

// F69: an occurrence, item or part may be typed by an occurrence definition of
// any kind (SysML v2 §8.3.9.7), matching the pilot's
// "An occurrence, item or part must be typed by occurrence definitions."
func TestF69OccurrenceUsageTypedByOccurrenceDefKinds(t *testing.T) {
	for _, src := range []string{
		"item def I; part def Car { part p : I; }",
		"connection def C; part def Car { part p : C; }",
		"action def A; part def Car { part p : A; }",
		"occurrence def O; part def Car { part p : O; }",
		"part def P; item def Box { item i : P; }",
		"action def A; item def Box { item i : A; }",
		"port def Pt; part def Car { occurrence o : Pt; }",
		"item def I; part def Car { individual x : I; }",
	} {
		// An individual additionally needs an individual definition, which the
		// reference reports separately; W10BIndividualTypingPass covers it.
		if diags := except(typeDiags(t, src), "individual-typing"); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics, got %v", src, diags)
		}
	}
}

// F69 negative: the pilot still rejects a part typed by a data type
// ("An occurrence, item or part must be typed by occurrence definitions.").
func TestF69PartTypedByAttributeDefRejected(t *testing.T) {
	diags := typeDiags(t, "attribute def Mass; part def Car { part p : Mass; }")
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
	if diags[0].Message != "An occurrence, item or part must be typed by occurrence definitions." {
		t.Errorf("got %q", diags[0].Message)
	}
}

// F69: a case may be typed by a case definition of any kind (§8.3.24.4); a
// nested `use case` parses as a case usage, so a use case def must satisfy it.
func TestF69CaseTypedByCaseDefKinds(t *testing.T) {
	for _, src := range []string{
		"use case def UC; case c : UC;",
		"analysis def AD; case c : AD;",
		"verification def VD; case c : VD;",
	} {
		if diags := typeDiags(t, src); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics, got %v", src, diags)
		}
	}
}

// F69 negatives: the pilot keeps "A case must be typed by one case definition."
// and "A use case must be typed by one use case definition."
func TestF69CaseAndUseCaseRejections(t *testing.T) {
	for _, tt := range []struct{ src, want string }{
		{"part def P; case c : P;", "A case must be typed by one case definition."},
		{"case def C; use case u : C;", "A use case must be typed by one use case definition."},
		{"action def A; use case u : A;", "A use case must be typed by one use case definition."},
	} {
		diags := typeDiags(t, tt.src)
		if len(diags) != 1 {
			t.Fatalf("%s: expected one type diagnostic, got %v", tt.src, diags)
		}
		if diags[0].Message != tt.want {
			t.Errorf("%s: got %q", tt.src, diags[0].Message)
		}
	}
}

// F69: an action must be typed by Behaviors (§8.3.16.6), which every
// behavior-family definition is.
func TestF69ActionTypedByBehaviorKinds(t *testing.T) {
	for _, src := range []string{
		"calc def Total; action a : Total;",
		"state def S; action a : S;",
	} {
		if diags := typeDiags(t, src); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics, got %v", src, diags)
		}
	}
}

// F69 negatives: the pilot rejects an action typed by a usage or a data type
// ("An action must be typed by action definitions.").
func TestF69ActionRejections(t *testing.T) {
	for _, src := range []string{
		"attribute def Mass; action a : Mass;",
		"calc def Total; calc t : Total; action a : t;",
		"action def A; action b : A; action a : b;",
	} {
		diags := typeDiags(t, src)
		if len(diags) != 1 {
			t.Fatalf("%s: expected one type diagnostic, got %v", src, diags)
		}
	}
}

// F53: a succession types through a plain UsageDeclaration
// (SysML.xtext:1033), so a definition of any kind types it.
func TestF69SuccessionTypedByConnectionDefKinds(t *testing.T) {
	for _, src := range []string{
		"part def A; connection def CD; part p1 : A; part p2 : A; succession s : CD first p1 then p2;",
		"part def A; interface def ID; part p1 : A; part p2 : A; succession s : ID first p1 then p2;",
	} {
		if diags := typeDiags(t, src); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics, got %v", src, diags)
		}
	}
}

// Malformed typings must analyse without panicking.
func TestF69MalformedTypingNoPanic(t *testing.T) {
	for _, src := range []string{
		"part p : ;",
		"succession s : first then;",
		"use case u : { }",
		"case c : Missing::Def;",
		"action a : = 1;",
	} {
		typeDiags(t, src)
	}
}
