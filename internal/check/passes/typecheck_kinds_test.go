package passes

import "testing"

func TestTypeCheckNewKindSpecializeSameKindOK(t *testing.T) {
	diags := typeDiags(t, "item def Base; item def Sub specializes Base;")
	if len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

// Occurrences are disjoint with data values (SysML v2 §8.4.5.1), so a part
// definition specializing an attribute definition is a cross-kind error.
func TestTypeCheckNewKindSpecializeCrossKindError(t *testing.T) {
	diags := typeDiags(t, "attribute def A; part def P specializes A;")
	if len(diags) != 1 {
		t.Fatalf("expected exactly one type diagnostic, got %v", diags)
	}
}

// A part definition is an item definition (SysML v2 §8.3.9.2), so either kind
// may specialize the other: `individual item def Alice :> Person` in the OMG
// training corpus names a part definition.
func TestTypeCheckSpecializeComparableKindOK(t *testing.T) {
	if diags := typeDiags(t, "part def P; item def I specializes P;"); len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
	if diags := typeDiags(t, "item def I; part def P specializes I;"); len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

func TestTypeCheckNewKindTypingWantsMatchingDef(t *testing.T) {
	// A port usage typed by a part def is a cross-kind error.
	diags := typeDiags(t, "part def Q; part def Sys { port p : Q; }")
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
}

func TestTypeCheckNewKindTypingMatchingDefOK(t *testing.T) {
	diags := typeDiags(t, "port def Q; part def Sys { port p : Q; }")
	if len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

// An objective is the ownedObjectiveRequirement of an ObjectiveMembership and so
// is a RequirementUsage (SysML v2 §8.3.22.4): a requirement definition and its
// specializations type it, structural definitions do not.
func TestTypeCheckObjectiveTypedByRequirementDefOK(t *testing.T) {
	diags := typeDiags(t, "requirement def MaximizeObjective; analysis def A { objective : MaximizeObjective; }")
	if len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

func TestTypeCheckObjectiveTypedByConcernDefOK(t *testing.T) {
	diags := typeDiags(t, "concern def C; analysis def A { objective o : C; }")
	if len(diags) != 0 {
		t.Fatalf("expected no type diagnostics, got %v", diags)
	}
}

func TestTypeCheckObjectiveTypedByPartDefError(t *testing.T) {
	diags := typeDiags(t, "part def Vehicle; analysis def A { objective o : Vehicle; }")
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
}

func TestTypeCheckObjectiveTypedByActionDefError(t *testing.T) {
	diags := typeDiags(t, "action def Move; analysis def A { objective o : Move; }")
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
}

// A subject is an unconstrained Usage (SysML v2 §8.3.21), so a definition of any
// kind types it. The OMG training models subject a `port def` and an `action def`
// (`32. Requirements/Requirement Definitions.sysml`).
func TestTypeCheckSubjectTypedByAnyDefKindOK(t *testing.T) {
	for _, src := range []string{
		"part def Vehicle; analysis def A { subject v : Vehicle; }",
		"port def ClutchPort; requirement def R { subject clutchPort : ClutchPort; }",
		"action def GenerateTorque; requirement def R { subject t : GenerateTorque; }",
		"requirement def R2; analysis def A { subject s : R2; }",
		"item def I; requirement def R { subject i : I; }",
	} {
		if diags := typeDiags(t, src); len(diags) != 0 {
			t.Errorf("%s: expected no type diagnostics, got %v", src, diags)
		}
	}
}

// A subject is still a usage, so its type must be a definition.
func TestTypeCheckSubjectTypedByUsageError(t *testing.T) {
	diags := typeDiags(t, "part def V; part v : V; requirement def R { subject s : v; }")
	if len(diags) != 1 {
		t.Fatalf("expected one type diagnostic, got %v", diags)
	}
}
