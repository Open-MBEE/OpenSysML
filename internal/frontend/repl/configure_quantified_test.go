package repl

import "testing"

// quantifiedModel quantifies over variation points: the wheels hold two objects,
// the spare may hold none.
const quantifiedModel = `
package Quantified {
    private import ControlFunctions::*;
    part def Wheel; part def NarrowWheel :> Wheel; part def WideWheel :> Wheel; part def Engine;
    part def Car { part engine : Engine; part wheels : Wheel[2]; part spare : Wheel[0..1]; }
    part fam : Car {
        variation part engineChoice :>> engine { variant part small : Engine; variant part big : Engine; }
        variation part wheelChoice :>> wheels { variant part narrow : NarrowWheel; variant part wide : WideWheel; }
        assert constraint {
            (engineChoice == engineChoice::small and wheelChoice->forAll { in ref w; w == wheelChoice::narrow }) xor
            (engineChoice == engineChoice::big and wheelChoice->exists { in ref w; w == wheelChoice::wide })
        }
    }
    part spared : Car {
        variation part spareChoice :>> spare { variant part narrowSpare : NarrowWheel; variant part wideSpare : WideWheel; }
        assert constraint { spareChoice->forAll { in ref w; w == spareChoice::narrowSpare } }
    }
}
`

// A quantifier over a variation point holding at least one object is the point's
// selection, so the whole configuration is one query and nothing is left out.
func TestConfigureTranslatesAQuantifierOverAVariationPoint(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, quantifiedModel)
	all := run(t, s, "%configure Quantified::fam all")
	wants(t, all, "✓ Part fam permits 2 selections of variants, which are all of them",
		"Quantified::fam::wheelChoice = Quantified::fam::wheelChoice::narrow",
		"Quantified::fam::wheelChoice = Quantified::fam::wheelChoice::wide")
	rejects(t, all, "Not in the query", "outside the subset")

	refused := run(t, s, "%configure Quantified::fam engineChoice=small wheelChoice=wide")
	wants(t, refused, "✗ the chosen variants are not consistent with Part fam",
		"In the conflict: Quantified::fam::engineChoice = Quantified::fam::engineChoice::small, Quantified::fam::wheelChoice = Quantified::fam::wheelChoice::wide")
}

// A point that may hold no object says nothing about its selection, so the
// quantifier stays refused, with that reason.
func TestConfigureRefusesAQuantifierOverAnOptionalPoint(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, quantifiedModel)
	wants(t, run(t, s, "%configure Quantified::spared all"),
		"error:", "`->forAll` expression", "spareChoice may hold no object, so the quantifier says nothing about the selection")
}
