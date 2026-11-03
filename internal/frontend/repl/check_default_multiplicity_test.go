package repl

import (
	"strings"
	"testing"
)

// kermlUnstatedModel reads a KerML feature that declares no multiplicity, which
// the default `0..*` governs (KerML 1.1 §7.4.12), in an invariant.
const kermlUnstatedModel = `package Tanks {
	private import ScalarValues::Real;

	class Tank {
		feature level : Real;
		inv nonNegative { level >= 0.0 }
	}
}`

// sysmlImplicitOneModel reads an attribute usage a definition owns, which has
// the implicit [1..1] (SysML v2 §7.6.3), in a constraint.
const sysmlImplicitOneModel = `package Tanks {
	private import ScalarValues::Real;

	part def Tank {
		attribute level : Real;
		constraint nonNegative { level >= 0.0 }
	}
}`

// %check over a feature only the default multiplicity governs answers about the
// case where it holds one value, and says so; over an implicit [1..1] usage it
// assumes nothing.
func TestCheckReportsTheOneValueAssumption(t *testing.T) {
	requireSolver(t)
	s := NewSession()
	if res := s.SubmitFiles([]SourceFile{{Name: "tanks.kerml", Text: kermlUnstatedModel}}); len(res.Diagnostics) > 0 {
		t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
	}
	reports := s.CheckSolve("Tanks::Tank::nonNegative")
	if len(reports) != 1 || reports[0].Status != SolveSat {
		t.Fatalf("check = %+v, want one sat report", reports)
	}
	joined := strings.Join(reports[0].Lines, "\n")
	want := "Assuming Tanks::Tank::level holds one value: it declares no multiplicity, so the default [0..*] governs it"
	if !strings.Contains(joined, want) {
		t.Errorf("report lacks %q:\n%s", want, joined)
	}

	s = checkSession(t, sysmlImplicitOneModel)
	reports = s.CheckSolve("Tanks::Tank::nonNegative")
	if len(reports) != 1 || reports[0].Status != SolveSat {
		t.Fatalf("check = %+v, want one sat report", reports)
	}
	if joined := strings.Join(reports[0].Lines, "\n"); strings.Contains(joined, "Assuming") {
		t.Errorf("an implicit [1..1] usage is assumed about:\n%s", joined)
	}
}

// sysmlSubsettingPartModel holds a part with an owned subsetting, which §7.6.3
// leaves at the default `0..*`: a check of the part def walks its one
// representative object, as validation does, and says so once.
const sysmlSubsettingPartModel = `package Crafts {
	private import ScalarValues::Real;

	part def Tank {
		attribute level : Real;
		assert constraint nonNegative { level >= 0.0 }
	}

	part def Craft {
		part main : Tank;
		part spare : Tank :> main;
	}
}`

func TestCheckReportsTheOneObjectAssumptionOnce(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, sysmlSubsettingPartModel)
	reports := s.CheckSolve("Crafts::Craft")
	if len(reports) != 1 || reports[0].Status != SolveSat {
		t.Fatalf("check = %+v, want one sat report", reports)
	}
	joined := strings.Join(reports[0].Lines, "\n")
	want := "Assuming spare holds one object: it declares no multiplicity, so the default [0..*] governs it"
	if strings.Count(joined, "Assuming") != 1 || !strings.Contains(joined, want) {
		t.Errorf("report should carry %q once:\n%s", want, joined)
	}
}
