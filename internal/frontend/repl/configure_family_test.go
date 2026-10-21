package repl

import "testing"

// familyModel is a product line the spec's way: an anonymous assertion owned by the
// abstract family, a definition asserting over its own variation, and configurations
// that redefine the variation points to one variant each.
const familyModel = `
package Fam {
    part def E; part def T;
    variation part def EngineChoices :> E { variant part e4 : E; variant part e6 : E; }
    abstract part fam {
        part engine : EngineChoices[1];
        variation part trans : T { variant man : T; variant auto : T; }
        assert constraint { (engine == EngineChoices::e4 and trans == trans::man) xor (engine == EngineChoices::e6 and trans == trans::auto) }
    }
    part good :> fam {
        part redefines engine = EngineChoices::e4;
        part redefines trans = trans::man;
    }
    part bad :> fam {
        part redefines engine = EngineChoices::e6;
        part redefines trans = trans::man;
    }
    part def FamDef {
        part engine : EngineChoices[1];
        assert constraint { engine == EngineChoices::e6 }
    }
    part fromDef : FamDef;
    part plain { part engine : EngineChoices[1]; }
    part noVariation { attribute x : ScalarValues::Integer; assert constraint { x > 0 } }
}
`

func TestConfigureAFamilyOverItsAssertions(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, familyModel)
	all := run(t, s, "%configure Fam::fam all")
	wants(t, all, "✓ Part fam permits 2 selections of variants, which are all of them",
		"Fam::fam::engine = Fam::EngineChoices::e4", "Fam::fam::trans = Fam::fam::trans::man",
		"Fam::fam::engine = Fam::EngineChoices::e6", "Fam::fam::trans = Fam::fam::trans::auto")

	one := run(t, s, "%configure Fam::fam")
	wants(t, one, "✓ Part fam permits a selection of variants", "One witness: use %configure Fam::fam all")
}

func TestConfigureAFamilyNamesTheRefusingAssertion(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, familyModel)
	chosen := run(t, s, "%configure Fam::fam engine=e6 trans=man")
	wants(t, chosen, "✗ the chosen variants are not consistent with Part fam",
		"In the conflict: Fam::fam::engine = Fam::EngineChoices::e6, Fam::fam::trans = Fam::fam::trans::man",
		"Against: required condition { engine == EngineChoices::e4 and trans == trans::man xor engine == EngineChoices::e6 and trans == trans::auto } of fam at ")
}

func TestConfigureAConfigurationAgainstItsFamily(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, familyModel)
	bad := run(t, s, "%configure Fam::bad")
	wants(t, bad, "✗ Part bad permits no selection of variants consistent with the values it fixes",
		"Already fixed:", "Fam::bad::engine = Fam::EngineChoices::e6  (declared)", "Fam::bad::trans = Fam::fam::trans::man  (declared)",
		"In the conflict: Fam::bad::engine = Fam::EngineChoices::e6, Fam::bad::trans = Fam::fam::trans::man",
		"Against: required condition {", "of fam at ")

	good := run(t, s, "%configure Fam::good")
	wants(t, good, "✓ Part good permits a selection of variants", "Fam::good::engine = Fam::EngineChoices::e4  (declared)")

	all := run(t, s, "%configure Fam::bad all")
	wants(t, all, "✗ Part bad permits no selection of variants consistent with the values it fixes")
}

func TestConfigureAUsageOverItsDefinitionsAssertion(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, familyModel)
	out := run(t, s, "%configure Fam::fromDef all")
	wants(t, out, "✓ Part fromDef permits 1 selection of variants, which are all of them", "::engine = Fam::EngineChoices::e6")
	def := run(t, s, "%configure Fam::FamDef all")
	wants(t, def, "✓ Part def FamDef permits 1 selection of variants, which are all of them", "Fam::FamDef::engine = Fam::EngineChoices::e6")
}

func TestConfigureAPartAssertingNothing(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, familyModel)
	out := run(t, s, "%configure Fam::plain all")
	wants(t, out, "error: part plain: asserts nothing about its objects")
	rejects(t, out, "no satisfaction assertion")
}

func TestConfigureAPartReadingNoVariation(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, familyModel)
	out := run(t, s, "%configure Fam::noVariation")
	wants(t, out, "Part noVariation: ", "Use %check Fam::noVariation for satisfiability")
	checked := run(t, s, "%check Fam::noVariation")
	wants(t, checked, "✓ Part noVariation")
}
