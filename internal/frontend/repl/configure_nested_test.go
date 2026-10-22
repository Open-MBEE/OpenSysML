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
