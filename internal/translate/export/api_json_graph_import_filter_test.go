package export

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// A condition reads an annotation's feature through a cast and compares it (KerML 7.4.9.2,
// 8.3.4.12.2); a guard settles `and`, and an unbound feature takes its declared default.
func TestAPIJSONGraphImportedMembershipFiltersOnAnnotationValues(t *testing.T) {
	elements := importProbeElements(t, `package Probe {
	enum def Kind { enum low; enum high; }
	metadata def Safety {
		attribute isMandatory : ScalarValues::Boolean;
		attribute level : ScalarValues::Natural = 1;
		attribute weight : ScalarValues::Real;
		attribute label : ScalarValues::String;
		attribute kind : Kind;
	}
	metadata def Strict :> Safety;
	package Lib {
		part a { @Safety { isMandatory = true; level = 3; weight = 2.5; label = "x"; kind = Kind::high; } }
		part b { @Safety { isMandatory = false; kind = Kind::low; } }
		part c { @Strict { isMandatory = true; level = 2; } }
		part d;
	}
	package Mandatory { import Lib::*; filter @Safety and (as Safety).isMandatory; }
	package Unguarded { import Lib::*; filter (as Safety).isMandatory; }
	package Above { import Lib::*; filter (as Safety).level > 1; }
	package Default { import Lib::*; filter (as Safety).level == 1; }
	package AtMost { import Lib::*; filter (as Safety).level <= 2; }
	package Heavy { import Lib::*; filter (as Safety).weight >= 2; }
	package Labelled { import Lib::*; filter (as Safety).label == "x"; }
	package Unlabelled { import Lib::*; filter @Safety and (as Safety).label != "x"; }
	package High { import Lib::*; filter Safety::kind == Kind::high; }
	package StrictOnly { import Lib::*; filter (as Strict).level == 2; }
}`)
	for _, want := range []struct {
		namespace string
		members   []string
	}{
		{"Probe__Mandatory", []string{"Probe__Lib__a_om", "Probe__Lib__c_om"}},
		{"Probe__Unguarded", []string{"Probe__Lib__a_om", "Probe__Lib__c_om"}},
		{"Probe__Above", []string{"Probe__Lib__a_om", "Probe__Lib__c_om"}},
		{"Probe__Default", []string{"Probe__Lib__b_om"}},
		{"Probe__AtMost", []string{"Probe__Lib__b_om", "Probe__Lib__c_om"}},
		{"Probe__Heavy", []string{"Probe__Lib__a_om"}},
		{"Probe__Labelled", []string{"Probe__Lib__a_om"}},
		{"Probe__Unlabelled", []string{"Probe__Lib__b_om", "Probe__Lib__c_om"}},
		{"Probe__High", []string{"Probe__Lib__a_om"}},
		{"Probe__StrictOnly", []string{"Probe__Lib__c_om"}},
	} {
		wantReferences(t, elements, want.namespace, "importedMembership", want.members...)
	}
}

// A metaclass feature is read from a candidate that instantiates the metaclass and has no
// value otherwise (KerML 8.4.4.9).
func TestAPIJSONGraphImportedMembershipFiltersOnMetaclassFeatures(t *testing.T) {
	elements := importProbeElements(t, `package Probe {
	private import KerML::*;
	package Lib {
		abstract part def System;
		part def Other;
		part system : System;
		package Nested;
	}
	package Named { import Lib::*; filter Element::name == "System"; }
	package Concrete { import Lib::*; filter not Type::isAbstract; }
	package Cast { import Lib::*; filter (as Type).isAbstract; }
	package Both { import Lib::*; filter Element::name == "System" and not Type::isAbstract or @SysML::PartUsage; }
}`)
	wantReferences(t, elements, "Probe__Named", "importedMembership", "Probe__Lib__System_om")
	wantReferences(t, elements, "Probe__Concrete", "importedMembership", "Probe__Lib__Other_om", "Probe__Lib__system_om")
	wantReferences(t, elements, "Probe__Cast", "importedMembership", "Probe__Lib__System_om")
	wantReferences(t, elements, "Probe__Both", "importedMembership", "Probe__Lib__system_om")
}

// A library candidate carries only the metadata the model states about it. The graph names
// rather than references the library element of such an annotation, so there it is withheld.
func TestAPIJSONGraphImportedMembershipFiltersLibraryCandidates(t *testing.T) {
	const model = `package Probe {
	metadata def Safety;
	part vehicle {
		part alarm;
		part seatBelt { @Safety; }
	}%s
	package Inherited { public import vehicle::*; filter @Safety; }
	package Library { import ScalarValues::*; filter @Safety; }
	package Own { import Inherited::*; filter @Safety; }
	package Plain { part p { @Safety; } part q; }
	package Local { import Plain::*; filter @Safety; }
}`
	elements := importProbeElements(t, fmt.Sprintf(model, ""))
	wantReferences(t, elements, "Probe__Inherited", "importedMembership", "Probe__vehicle__seatBelt_om")
	wantReferences(t, elements, "Probe__Own", "importedMembership", "Probe__vehicle__seatBelt_om")
	if _, stated := elements["Probe__Library"]["importedMembership"]; stated {
		t.Errorf("Library states importedMembership %v, want none imported", elements["Probe__Library"]["importedMembership"])
	}
	if _, stated := elements["Probe__Library"]["member"]; !stated {
		t.Error("Library states no member, want its derivation decided")
	}

	elements = importProbeElements(t, fmt.Sprintf(model, "\n\tmetadata marked : Safety about ScalarValues::Real;"))
	wantReferences(t, elements, "Probe__Local", "importedMembership", "Probe__Plain__p_om")
	for _, id := range []string{"Probe__Inherited", "Probe__Library", "Probe__Own"} {
		if _, stated := elements[id]["member"]; stated {
			t.Errorf("%s states member, want it withheld while an annotation is about an element outside the graph", id)
		}
	}
}

// A condition outside the model-level evaluable forms the structure decides leaves the
// imports withheld rather than emptied.
func TestAPIJSONGraphImportedMembershipWithheldForUndecidedFilter(t *testing.T) {
	elements := importProbeElements(t, `package Probe {
	metadata def Safety { attribute level : ScalarValues::Natural; }
	package Lib { part a { @Safety { level = 3; } } }
	package Sum { import Lib::*; filter (as Safety).level + 1 > 3; }
	package Mixed { import Lib::*; filter (as Safety).level == "3"; }
}`)
	for _, id := range []string{"Probe__Sum", "Probe__Mixed"} {
		element, ok := elements[id]
		if !ok {
			t.Fatalf("no element %s", id)
		}
		for _, property := range []string{"importedMembership", "membership", "member"} {
			if _, stated := element[property]; stated {
				t.Errorf("%s states %s for a condition the structure does not decide", id, property)
			}
		}
	}
}

// The member a feature chain reads from a cast is a member of the type cast to (KerML
// 8.2.5.8.1), so the element form references it rather than spelling its name.
func TestAPIJSONCastChainReferencesItsTargetFeature(t *testing.T) {
	elements := importProbeElements(t, `package Probe {
	metadata def Safety { attribute isMandatory : ScalarValues::Boolean; }
	package Lib { part a { @Safety { isMandatory = true; } } }
	package Mandatory { import Lib::*; filter (as Safety).isMandatory; }
}`)
	chains := 0
	for id, element := range elements {
		if element["@type"] != "FeatureChainExpression" {
			continue
		}
		chains++
		target, _ := element["targetFeature"].(map[string]any)
		if target["@id"] != "Probe__Safety__isMandatory" {
			t.Errorf("%s.targetFeature = %v, want a reference to Probe__Safety__isMandatory", id, element["targetFeature"])
		}
	}
	if chains != 1 {
		t.Errorf("%d feature chain expressions, want 1", chains)
	}
}

// The notation written back spells a member read past a cast as the member of the type cast
// to, the way the graph names it (KerML 8.2.5.8.1).
func TestCastChainSegmentIsWrittenBack(t *testing.T) {
	out, err := ToSysML(graphOfNotation(t, `package Probe {
	metadata def Safety { attribute isMandatory : ScalarValues::Boolean; }
	package Lib { part a { @Safety { isMandatory = true; } } }
	package Mandatory { import Lib::*; filter (as Safety).isMandatory; }
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "(as Safety).isMandatory") {
		t.Errorf("the notation written back loses the cast chain:\n%s", out)
	}
}

// A library alias a namespace imports is referenced by its own membership, which the graph
// names beside the element it aliases, and the notation reads back without it.
func TestImportedLibraryAliasMembershipIsAStub(t *testing.T) {
	const model = "package Probe {\n\tpackage Quantities { import ISQThermodynamics::*; }\n}\n"
	graph := graphOfNotation(t, model)
	aliases := 0
	for _, subject := range graph.Subjects() {
		path, ok := graph.Lexical(subject, rdf.OpenSysML+xLibraryPath)
		if !ok || !strings.HasPrefix(path, "ISQThermodynamics/Membership[") {
			continue
		}
		aliases++
		if !LibraryReference(graph, subject) || len(graph.Objects(subject, rdf.SysML+pMemberElement)) != 1 {
			t.Errorf("the alias membership %s is no library stub naming one member", path)
		}
	}
	if aliases == 0 {
		t.Fatal("the graph names no alias membership of ISQThermodynamics")
	}
	var warnings []string
	out, err := ToSysMLWarn(graph, func(w string) { warnings = append(warnings, w) })
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Errorf("reading the alias memberships back warns: %v", warnings[0])
	}
	if !strings.Contains(string(out), "import ISQThermodynamics::*;") || strings.Contains(string(out), "alias") {
		t.Errorf("the notation written back is\n%s", out)
	}
}

// An import of an alias imports the alias Membership itself (KerML 8.2.3.5, 8.3.2.4.4), not
// the owning membership of the element the alias names: the model's own alias and a library's.
func TestMembershipImportOfAnAliasImportsTheAliasMembership(t *testing.T) {
	const model = `package Probe {
	package Lib { part def Original; alias Nick for Original; }
	package Own { import Lib::Nick; }
	package Quantities { import ISQThermodynamics::TemperatureValue; }
}
`
	graph := graphOfNotation(t, model)
	var library []rdf.Term
	for _, subject := range graph.Subjects() {
		path, ok := graph.Lexical(subject, rdf.OpenSysML+xLibraryPath)
		if ok && strings.HasPrefix(path, "ISQThermodynamics/Membership[") {
			library = append(library, subject)
		}
	}
	if len(library) != 1 {
		t.Fatalf("the graph names the alias memberships %v of ISQThermodynamics, want the one imported", library)
	}
	imports := 0
	for _, subject := range graph.Subjects() {
		for _, imported := range graph.Objects(subject, rdf.SysML+pImportedMembership) {
			if imported == library[0] {
				imports++
			}
		}
	}
	if imports != 1 {
		t.Errorf("%d imports name the library alias membership, want 1", imports)
	}
	elements := importProbeElements(t, model)
	wantReferences(t, elements, "Probe__Own", "importedMembership", "Probe__Lib__Nick")
	wantReferences(t, elements, "Probe__Own", "member", "Probe__Lib__Original")
	quantities := referenceIDs(t, elements["Probe__Quantities"], "importedMembership")
	if want := rdf.LocalName(library[0].Value); len(quantities) != 1 || quantities[0] != want {
		t.Errorf("Probe__Quantities.importedMembership = %v, want [%s]", quantities, want)
	}
	var warnings []string
	out, err := ToSysMLWarn(graph, func(w string) { warnings = append(warnings, w) })
	if err != nil || len(warnings) != 0 {
		t.Fatalf("reading the imports back: %v %v", err, warnings)
	}
	for _, want := range []string{"import Lib::Nick;", "import ISQThermodynamics::TemperatureValue;"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("the notation written back lacks %q:\n%s", want, out)
		}
	}
}
