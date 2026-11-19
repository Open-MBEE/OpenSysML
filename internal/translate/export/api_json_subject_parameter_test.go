package export

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A subject is a parameter, so its direction is in (KerML 8.3.4.6.4 parameterDirection) and a
// stated subject redefines the inherited one at its position (SysML 7.17.2).
func TestAPIJSONInheritedSubjectIsAnInputUntilRedefined(t *testing.T) {
	graph := duplicateGraph(t, `package Probe {
		part def V;
		requirement def Bare;
		requirement stated { subject v : V; }
		requirement def Wide { subject w : V; in limit : ScalarValues::Real; }
		requirement narrow : Wide { subject n : V; }
		use case def Outer {
			subject system : V;
			include use case inner { subject = system; }
		}
	}`, IDQualifiedName)
	document, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{InheritedMembers: true})
	if err != nil {
		t.Fatal(err)
	}
	elements := booleansElements(t, document)
	catalog := identity.LibraryCatalogWithOpenSysML(libs.SharedBase())
	subj, ok := catalog.ElementNamed("Requirements::RequirementCheck::subj")
	if !ok {
		t.Fatal("the library catalog has no Requirements::RequirementCheck::subj")
	}
	for _, property := range []string{"input", "directedFeature"} {
		if got := referenceIDs(t, elements["Probe__Bare"], property); !slices.Contains(got, subj.ID) {
			t.Errorf("Bare.%s = %v, want the inherited library subject", property, got)
		}
	}
	if got := referenceIDs(t, elements["Probe__Bare"], "output"); slices.Contains(got, subj.ID) {
		t.Errorf("Bare.output = %v, want no subject", got)
	}
	if got := referenceIDs(t, elements["Probe__stated"], "input"); !slices.Equal(got, []string{"Probe__stated__v"}) {
		t.Errorf("stated.input = %v, want its own subject only", got)
	}
	if got := referenceIDs(t, elements["Probe__stated"], "inheritedFeature"); slices.Contains(got, subj.ID) {
		t.Error("stated.inheritedFeature keeps the library subject its own subject redefines")
	}
	want := []string{"Probe__narrow__n", "Probe__Wide__limit"}
	if got := referenceIDs(t, elements["Probe__narrow"], "input"); !slices.Equal(got, want) {
		t.Errorf("narrow.input = %v, want %v", got, want)
	}
	// The implied base feature of a nested usage is a general too.
	if got := referenceIDs(t, elements["Probe__Outer__inner"], "input"); !slices.Equal(got, []string{"Probe__Outer__inner__subj"}) {
		t.Errorf("inner.input = %v, want its own subject only", got)
	}
}

// An objective redefines the objective of the case its owner specializes and inherits its
// members (SysML 8.3.21.9 checkRequirementUsageObjectiveRedefinition).
func TestAPIJSONObjectiveInheritsFromTheObjectiveItRedefines(t *testing.T) {
	graph := duplicateGraph(t, `package Probe {
		verification def Check { objective { doc /* goal */ } }
	}`, IDQualifiedName)
	document, err := WriteAPIJSONWithOptions(graph, APIJSONOptions{InheritedMembers: true})
	if err != nil {
		t.Fatal(err)
	}
	elements := booleansElements(t, document)
	catalog := identity.LibraryCatalogWithOpenSysML(libs.SharedBase())
	general, ok := catalog.ElementNamed("VerificationCases::VerificationCase::obj::subj")
	if !ok {
		t.Fatal("the library catalog has no VerificationCases::VerificationCase::obj::subj")
	}
	redefined, ok := catalog.ElementNamed("Requirements::RequirementCheck::subj")
	if !ok {
		t.Fatal("the library catalog has no Requirements::RequirementCheck::subj")
	}
	objective := elements["Probe__Check__obj"]
	if got := referenceIDs(t, objective, "input"); !slices.Equal(got, []string{general.ID}) {
		t.Errorf("objective.input = %v, want the subject of the objective it redefines, %s", got, general.ID)
	}
	if got := referenceIDs(t, objective, "inheritedFeature"); slices.Contains(got, redefined.ID) {
		t.Error("objective.inheritedFeature keeps Requirements::RequirementCheck::subj, which the inherited subject redefines")
	}
}
