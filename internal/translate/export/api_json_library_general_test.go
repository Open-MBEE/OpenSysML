package export

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

func libraryElementID(t *testing.T, name string) string {
	t.Helper()
	element, ok := identity.LibraryCatalogWithOpenSysML(libs.SharedBase()).ElementNamed(name)
	if !ok {
		t.Fatalf("the library catalog has no %s", name)
	}
	return element.ID
}

// A parameter that redefines a library feature leaves the types inheriting it their family:
// the library row the graph keeps for the redefined feature is read from the semantic model.
func TestAPIJSONInheritedParameterRedefiningALibraryFeatureCompletes(t *testing.T) {
	graph := duplicateGraph(t, `package Probe {
		calc def C specializes Calculations::Calculation {
			in ref a redefines Base::things;
			in ref b redefines Base::things;
			return ref result redefines Calculations::Calculation::result;
		}
		calc def C1 specializes C {
			in ref a redefines C::a;
			return ref result redefines C::result;
			in ref b redefines C::b;
		}
	}`, IDQualifiedName)
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	elements := booleansElements(t, document)
	want := []string{"Probe__C1__a", "Probe__C1__b"}
	if got := referenceIDs(t, elements["Probe__C1"], "input"); !slices.Equal(got, want) {
		t.Errorf("C1.input = %v, want %v", got, want)
	}
	inherited := referenceIDs(t, elements["Probe__C1"], "inheritedFeature")
	if len(inherited) == 0 {
		t.Fatal("C1.inheritedFeature is withheld or empty")
	}
	for _, redefined := range []string{"Probe__C__a", "Probe__C__b", "Probe__C__result"} {
		if slices.Contains(inherited, redefined) {
			t.Errorf("C1.inheritedFeature keeps the redefined %s", redefined)
		}
	}
}

// A feature chain's last feature is a supertype of the chain (KerML 8.3.3.3.4
// Feature::supertypes), so what reference-subsets the chain inherits that feature's members.
func TestAPIJSONFeatureChainInheritsFromItsTarget(t *testing.T) {
	graph := duplicateGraph(t, `package Probe {
		part def V;
		requirement spec {
			requirement economy { subject v : V; }
		}
		requirement top {
			require spec.economy;
		}
	}`, IDQualifiedName)
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	elements := booleansElements(t, document)
	want := []string{"Probe__spec__economy__v"}
	if got := referenceIDs(t, elements["Probe__top___400"], "input"); !slices.Equal(got, want) {
		t.Errorf("the required chain's input = %v, want %v", got, want)
	}
}

// `then done;` reaches the `done` its owner inherits, which a use case redefines
// (UseCases::UseCase::done), not the Actions::Action::done that one redefines.
func TestAPIJSONDoneOfAUseCaseIsTheUseCaseLibraryFeature(t *testing.T) {
	graph := duplicateGraph(t, `package Probe {
		use case def UC { first start; then done; }
		action def A { first start; then done; }
	}`, IDQualifiedName)
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	elements := booleansElements(t, document)
	for subsetting, name := range map[string]string{
		"Probe__UC___402_pend1_prs": "UseCases::UseCase::done",
		"Probe__A___402_pend1_prs":  "Actions::Action::done",
	} {
		reference, _ := elements[subsetting]["referencedFeature"].(map[string]any)
		if got, want := reference["@id"], libraryElementID(t, name); got != want {
			t.Errorf("%s references %v, want %s (%s)", subsetting, got, name, want)
		}
	}
}

// A performed feature chain's own parameters redefine those of the action the chain names
// by position (KerML 8.4.4.7 checkFeatureParameterRedefinition), so the named action's
// parameters are not inherited beside them.
func TestAPIJSONParametersOfAPerformedChainRedefineItsTargets(t *testing.T) {
	graph := duplicateGraph(t, `package Probe {
		action region {
			action produce { in cmd; out light; }
		}
		part lamp {
			perform region.produce { in cmd; out light; }
		}
	}`, IDQualifiedName)
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	elements := booleansElements(t, document)
	want := []string{"Probe__lamp__produce__cmd", "Probe__lamp__produce__light"}
	if got := referenceIDs(t, elements["Probe__lamp__produce"], "directedFeature"); !slices.Equal(got, want) {
		t.Errorf("the performed chain's directedFeature = %v, want %v", got, want)
	}
}

// A flow end's FlowFeature has the direction of the feature it redefines as the end inherits
// it (KerML.xtext FlowFeature, note 1), reversed through a conjugated port.
func TestAPIJSONFlowFeatureHasTheDirectionOfTheFeatureItRedefines(t *testing.T) {
	graph := duplicateGraph(t, `package Probe {
		item def Air;
		port def P { in ref air : Air; in ref other : Air; }
		part def H { port x : ~P; }
		part def C { port y : P; }
		part ctx {
			part h : H;
			part c : C;
			flow f from h.x.air to c.y.air;
		}
	}`, IDQualifiedName)
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	elements := booleansElements(t, document)
	for end, want := range map[string]string{"Probe__ctx__f_pend0": "out", "Probe__ctx__f_pend1": "in"} {
		// The feature the end inherits beside it takes the same path (KerML 8.3.3.1.10 directionOf).
		property := map[string]string{"out": "output", "in": "input"}[want]
		if got := referenceIDs(t, elements[end], property); !slices.Contains(got, "Probe__P__other") {
			t.Errorf("%s %s = %v, want the inherited Probe__P__other in it", end, property, got)
		}
		owned := referenceIDs(t, elements[end], "ownedFeature")
		if len(owned) != 1 {
			t.Fatalf("%s owns %v, want its FlowFeature alone", end, owned)
		}
		if got := elements[owned[0]]["direction"]; got != want {
			t.Errorf("the FlowFeature of %s has direction %v, want %s", end, got, want)
		}
		if got := referenceIDs(t, elements[end], "directedFeature"); !slices.Equal(got, append(owned, "Probe__P__other")) {
			t.Errorf("%s directedFeature = %v, want its FlowFeature and Probe__P__other", end, got)
		}
	}
}

// A port typed by a conjugated definition reverses the directions the original gives it, and a
// feature it declares itself keeps the direction it states (KerML 8.3.3.1.10 directionOf).
func TestAPIJSONConjugatedPortKeepsTheDirectionOfTheFeatureItDeclares(t *testing.T) {
	graph := duplicateGraph(t, `package Probe {
		item def Air;
		port def P { in ref air : Air; in ref other : Air; }
		part a { port x : ~P { out ref :>> air; } }
		part b { port y : ~P { in ref :>> air; } }
	}`, IDQualifiedName)
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	elements := booleansElements(t, document)
	for port, want := range map[string][2][]string{
		"Probe__a__x": {nil, {"Probe__a__x__air", "Probe__P__other"}},
		"Probe__b__y": {{"Probe__b__y__air"}, {"Probe__P__other"}},
	} {
		for i, property := range []string{"input", "output"} {
			var got []string
			if _, stated := elements[port][property]; stated {
				got = referenceIDs(t, elements[port], property)
			}
			if !slices.Equal(got, want[i]) {
				t.Errorf("%s %s = %v, want %v", port, property, got, want[i])
			}
		}
	}
}
