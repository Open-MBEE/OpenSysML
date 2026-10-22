package repl

import (
	"strings"
	"testing"
)

// nestedModel holds parts whose nested parts' objects carry assertions of their
// own: a variant owning one over its own variation (the shape of the OMG
// VehicleVariabilityModel), a nested part whose declared value refuses the
// family, one reading the enclosing part's variation, and one held many times.
const nestedModel = `
package Nested {
    private import ScalarValues::*;
    part def T;
    port def P;
    part def Engine {
        variation port autoPort : P { variant port p1 : P; variant port p2 : P; }
    }
    variation part def EngineChoices :> Engine {
        variant part small : Engine;
        variant part big : Engine { assert constraint { autoPort == autoPort::p2 } }
    }
    part family {
        variation part trans : T { variant man : T; variant auto : T; }
        part engine : EngineChoices;
        assert constraint { trans == trans::man }
    }

    part def Gear { attribute teeth : Integer; assert constraint { teeth > 10 } }
    part refused {
        variation part trans : T { variant man : T; variant auto : T; }
        part gear : Gear { attribute :>> teeth = 4; }
        assert constraint { trans == trans::man }
    }
    part deep {
        variation part trans : T { variant man : T; variant auto : T; }
        part box { part gear : Gear { attribute :>> teeth = 4; } }
        assert constraint { trans == trans::man }
    }
    part outer {
        variation part trans : T { variant man : T; variant auto : T; }
        part box { assert constraint { trans == trans::auto } }
    }
    part def Block { port autoPort; }
    variation part def Engines :> Block {
        variant part four : Block;
        variant part six : Block {
            variation port :>> autoPort { variant port a1; variant port a2; }
            assert constraint { autoPort == autoPort::a2 }
        }
    }
    part fleet {
        variation part trans : T { variant man : T; variant auto : T; }
        part engine : Engines;
        assert constraint { trans == trans::man }
    }
    part many {
        variation part trans : T { variant man : T; variant auto : T; }
        part gear : Gear[2];
        assert constraint { trans == trans::man }
    }
}
`

func TestConfigureAppliesAVariantsAssertionUnderItsVariant(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, nestedModel)
	out := run(t, s, "%configure Nested::family all")
	wants(t, out, "✓ Part family permits 3 selections of variants",
		"Nested::family::engine = Nested::EngineChoices::big",
		"Nested::family::'engine.autoPort' = Nested::Engine::autoPort::p1")
	rejects(t, out, "Not in the query")
	for _, sel := range strings.Split(out, "\n  ") {
		if strings.Contains(sel, "EngineChoices::big") && strings.Contains(sel, "autoPort::p1") {
			t.Errorf("big's assertion is not applied under big:\n%s", sel)
		}
	}
	wants(t, run(t, s, "%configure Nested::family engine=big engine.autoPort=p1"),
		"✗ the chosen variants are not consistent with Part family",
		"Against: required condition engine == big implies { autoPort == autoPort::p2 } of big at ")
}

func TestConfigureANestedPartsAssertionRefusesTheFamily(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, nestedModel)
	wants(t, run(t, s, "%configure Nested::refused all"),
		"✗ Part refused permits no selection of variants consistent with the value it fixes",
		"Nested::refused::'gear.teeth' = 4  (declared)",
		"Against: required condition { teeth > 10 } of Gear at ")
	wants(t, run(t, s, "%configure Nested::deep all"),
		"✗ Part deep permits no selection of variants consistent with the value it fixes",
		"Nested::deep::'box.gear.teeth' = 4  (declared)")
}

func TestConfigureANestedPartReadingTheEnclosingVariation(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, nestedModel)
	wants(t, run(t, s, "%configure Nested::outer all"),
		"✓ Part outer permits 1 selection of variants, which are all of them",
		"Nested::outer::trans = Nested::outer::trans::auto")
}

func TestConfigureRefusesAMultiValuedNestedPartByName(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, nestedModel)
	wants(t, run(t, s, "%configure Nested::many all"),
		"✓ Part many permits 1 selection of variants, which are all of them",
		"Not in the query: what gear asserts about its own objects, as it may hold more than one object; %instantiate and %validate an object to check it")
}

func TestConfigureSelectsAVariantsOwnVariationOnlyUnderIt(t *testing.T) {
	requireSolver(t)
	s := checkSession(t, nestedModel)
	out := run(t, s, "%configure Nested::fleet all")
	wants(t, out, "✓ Part fleet permits 2 selections of variants, which are all of them",
		"Nested::fleet::engine = Nested::Engines::four",
		"Nested::fleet::engine = Nested::Engines::six",
		"Nested::fleet::'engine.autoPort' = Nested::Engines::six::autoPort::a2")
	for _, sel := range strings.Split(out, "\n  ") {
		if strings.Contains(sel, "Engines::four") && strings.Contains(sel, "autoPort") {
			t.Errorf("four has no autoPort variation, yet its selection lists one:\n%s", sel)
		}
	}
	wants(t, run(t, s, "%configure Nested::fleet engine=four engine.autoPort=a1"),
		"engine.autoPort is a variation point only when engine selects six")
	wants(t, run(t, s, "%configure Nested::fleet engine.autoPort=a1"),
		"✗ the chosen variants are not consistent with Part fleet")
}
