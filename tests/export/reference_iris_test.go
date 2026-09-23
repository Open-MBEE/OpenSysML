package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// referenceFixture declares the shapes whose reference-valued properties the
// graph links: a type, a `first` start, a `then` source, a chain target, a
// referent, a function, a library metadata definition, both import kinds, both
// expose kinds, and names nothing declares.
const referenceFixture = `package Refs {
    private import ScalarValues::*;
    private import ISQ::MassValue;
    private import ParametersOfInterestMetadata::*;
    part def Engine { attribute power : Real; }
    part def Vehicle {
        attribute mass : MassValue;
        attribute count : Integer;
        part engine : Engine;
        attribute ratio : Real = engine.power;
        attribute unknown : Missing::Kind;
    }
    #moe attribute refresh : Real;
    action def Drive {
        action a1;
        first start then a1;
        then done;
    }
    alias Motor for Engine;
    package Wheels {
        private import Motor;
        private import Refs::Vehicle::**;
        private import Nowhere::Nothing;
        private import Nowhere::*;
    }
    view def Overview;
    view v : Overview {
        expose Vehicle::**;
        expose Wheels::*;
    }
}`

// The normative ids of the standard library elements the fixture names.
const (
	massValueID           = "9cd0e404-efee-50e5-a59b-681065bd188c"
	measureOfEffectiveID  = "cabf5c12-aaad-587e-bb44-ed0d4376e0b8"
	massValueMembershipID = "9a304301-2c85-5a21-8c03-2b345fc1c3d9"
	integerID             = "f2350199-2ab1-5258-8514-58812ef25dc6"
	realID                = "14c0aa22-5489-59b5-b438-ded26e83ba31"
	actionStartID         = "9a0d2905-0f9c-5bb4-af74-9780d6db1817"
	scalarValuesID        = "40bb440c-5036-58e1-8675-5afccb8b8f1d"
)

// link is the triple linking property to the element with id, in the form the
// Turtle writer gives it; an id it cannot prefix is written in full.
func link(property, id string) string {
	if id[0] >= '0' && id[0] <= '9' {
		return property + " <urn:sysmlv2:element:" + id + ">"
	}
	return property + " elmt:" + id
}

// TestReferencePropertiesLinkElements pins the reference rule: a name resolving
// to an element of this graph or of the standard library is linked by IRI, and
// only a name resolving to neither is carried as written.
func TestReferencePropertiesLinkElements(t *testing.T) {
	graph, err := convert.Convert("refs.sysml", []byte(referenceFixture), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	text := string(graph)
	for _, want := range []string{
		// Standard library elements by their normative ids.
		link("sysml:type", massValueID),
		link("sysml:type", integerID),
		link("sysml:type", realID),
		link("sysml:sourceFeature", actionStartID),
		link("sysml:type", measureOfEffectiveID),
		link("sysml:importedNamespace", scalarValuesID),
		link("sysml:importedMembership", massValueMembershipID),
		// Elements of this graph by their own ids.
		"sysml:type elmt:Refs__Engine",
		"sysml:targetFeature elmt:Refs__Drive__a1",
		"sysml:referent elmt:Refs__Vehicle__engine",
		"sysml:targetFeature elmt:Refs__Engine__power",
		"a sysml:NamespaceImport",
		"a sysml:MembershipImport",
		"a sysml:NamespaceExpose",
		"a sysml:MembershipExpose",
		"sysml:importedMembership elmt:Refs__Vehicle_om ;",
		"sysml:importedNamespace elmt:Refs__Wheels ;",
		// An import written through an alias imports the alias's membership.
		"sysml:importedMembership elmt:Refs__Motor_om ;",
		// Names nothing declares are carried as written.
		`sysml:type "Missing::Kind"`,
		`sysml:importedNamespace "Nowhere"`,
		`sysml:importedMembership "Nowhere::Nothing"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("graph does not record %q\n%s", want, text)
		}
	}
	for _, reject := range []string{
		"a sysml:Import ;",
		"sysx:isNamespaceImport",
		"sysx:isExpose",
		"sysml:importedMembership elmt:Refs__Motor ;",
		`sysml:type "MassValue"`,
		`sysml:type "Engine"`,
		`sysml:importedNamespace "ScalarValues"`,
		`sysml:sourceFeature "start"`,
	} {
		if strings.Contains(text, reject) {
			t.Errorf("graph still writes %q\n%s", reject, text)
		}
	}
	structuralRoundTrip(t, "refs", graph)
}

// TestLegacyReferenceGraphsStillRead covers the graphs earlier releases wrote:
// references as name literals and imports and exposes as an abstract
// sysml:Import whose kind is a flag. All read back as the notation that produced them.
func TestLegacyReferenceGraphsStillRead(t *testing.T) {
	graph, err := convert.Convert("refs.sysml", []byte(referenceFixture), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	legacy := string(withoutTriples(t, graph, "sysx:sourceText"))
	for _, edit := range [][2]string{
		{`sysml:declaredName "mass" ;` + "\n    " + link("sysml:type", massValueID), `sysml:declaredName "mass" ;` + "\n    " + `sysml:type "MassValue"`},
		{`sysml:declaredName "engine" ;` + "\n    " + `sysml:type elmt:Refs__Engine`, `sysml:declaredName "engine" ;` + "\n    " + `sysml:type "Engine"`},
		{link("sysml:sourceFeature", actionStartID), `sysml:sourceFeature "start"`},
		{link("sysml:importedNamespace", scalarValuesID), `sysml:importedNamespace "ScalarValues"`},
		{link("sysml:importedMembership", massValueMembershipID), `sysml:importedNamespace "ISQ::MassValue"`},
		{"sysml:importedMembership elmt:Refs__Motor_om", `sysml:importedNamespace "Motor"`},
		{"a sysml:NamespaceImport ;", `a sysml:Import ;
    sysx:isNamespaceImport "true"^^xsd:boolean ;`},
		{"a sysml:MembershipImport ;", "a sysml:Import ;"},
		{"a sysml:NamespaceExpose ;", `a sysml:Import ;
    sysx:isExpose "true"^^xsd:boolean ;
    sysx:isNamespaceImport "true"^^xsd:boolean ;`},
		{"a sysml:MembershipExpose ;", `a sysml:Import ;
    sysx:isExpose "true"^^xsd:boolean ;`},
	} {
		if !strings.Contains(legacy, edit[0]) {
			t.Fatalf("graph does not record %q\n%s", edit[0], legacy)
		}
		legacy = strings.ReplaceAll(legacy, edit[0], edit[1])
	}
	back, err := convert.Convert("legacy.ttl", []byte(legacy), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	for _, want := range []string{
		"attribute mass : MassValue;",
		"part engine : Engine;",
		"first start then a1;",
		"then done;",
		"private import ScalarValues::*;",
		"private import ISQ::MassValue;",
		"private import Motor;",
		"private import Vehicle::**;",
		"private import Nowhere::Nothing;",
		"private import Nowhere::*;",
		"expose Vehicle::**;",
		"expose Wheels::*;",
	} {
		if !strings.Contains(string(back), want) {
			t.Errorf("legacy graph does not read back %q\n%s", want, back)
		}
	}
	// The next hop writes today's graph: the legacy forms are read, not kept.
	second, err := convert.Convert("refs.sysml", back, convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle again: %v", err)
	}
	requireSameGraphBytes(t, back, graph, second)
}

// TestLegacyConnectorAsUsageStillReads covers the abstract metaclass older graphs
// typed a KerML `connector` with; it reads back as the connector it typed.
func TestLegacyConnectorAsUsageStillReads(t *testing.T) {
	const src = `package Links {
    class Tank;
    feature main : Tank;
    feature eng : Tank;
    connector link from eng to main;
}`
	graph, err := convert.Convert("links.kerml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	if !strings.Contains(string(graph), "a sysml:Connector ;") || strings.Contains(string(graph), "a sysml:ConnectorAsUsage ;") {
		t.Fatalf("a connector should be typed sysml:Connector\n%s", graph)
	}
	legacy := strings.ReplaceAll(string(withoutTriples(t, graph, "sysx:sourceText")), "a sysml:Connector ;", "a sysml:ConnectorAsUsage ;")
	back, err := convert.Convert("legacy.ttl", []byte(legacy), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "connector link from eng to main;") {
		t.Errorf("legacy graph does not read back the connector\n%s", back)
	}
}

// A metadata usage in a dependency's body links its type like any other, and
// the structure alone carries it back: the name must resolve from the body.
func TestDependencyBodyMetadataLinksItsType(t *testing.T) {
	const src = `package Deps {
    part def A;
    part def B;
    dependency A to B {
        @ModelingMetadata::Refinement;
    }
}
`
	turtle, err := convert.Convert("deps.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(turtle), "sysml:type <urn:sysmlv2:element:") {
		t.Fatalf("the metadata usage's type is not linked:\n%s", turtle)
	}
	back := structuralRoundTrip(t, "deps", turtle)
	if !strings.Contains(string(back), "@ModelingMetadata::Refinement;") {
		t.Errorf("the mapping alone did not bring the metadata usage back:\n%s", back)
	}
}

// TestImportNamingTwoElementsIsRefused pins that an import stating both an
// importedMembership and an importedNamespace is refused, not read as one of them.
func TestImportNamingTwoElementsIsRefused(t *testing.T) {
	graph, err := convert.Convert("refs.sysml", []byte(referenceFixture), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	const one = "sysml:importedMembership elmt:Refs__Motor_om ;"
	if !strings.Contains(string(graph), one) {
		t.Fatalf("graph does not record %q\n%s", one, graph)
	}
	two := strings.Replace(string(graph), one, one+"\n    sysml:importedNamespace elmt:Refs__Wheels ;", 1)
	_, err = convert.Convert("two.ttl", []byte(two), convert.FormatTurtle, convert.FormatSysML)
	if err == nil || !strings.Contains(err.Error(), "names one element") {
		t.Fatalf("an import naming two elements should be refused, got %v", err)
	}
}

// TestImportContradictingItsClassIsRefused pins that a concrete import class
// stating the other kind's target property is refused, not written as either kind.
func TestImportContradictingItsClassIsRefused(t *testing.T) {
	graph, err := convert.Convert("refs.sysml", []byte(referenceFixture), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	for _, c := range []struct{ target, class, other string }{
		{"sysml:importedMembership elmt:Refs__Motor_om ;", "sysml:MembershipImport", "sysml:NamespaceImport"},
		{"sysml:importedNamespace elmt:Refs__Wheels ;", "sysml:NamespaceImport", "sysml:MembershipImport"},
	} {
		i := strings.Index(string(graph), c.target)
		if i < 0 {
			t.Fatalf("graph does not record %q\n%s", c.target, graph)
		}
		head := strings.LastIndex(string(graph)[:i], "a "+c.class+" ;")
		if head < 0 {
			t.Fatalf("no %s before %q\n%s", c.class, c.target, graph)
		}
		swapped := string(graph)[:head] + "a " + c.other + " ;" + string(graph)[head+len("a "+c.class+" ;"):]
		_, err = convert.Convert("swapped.ttl", []byte(swapped), convert.FormatTurtle, convert.FormatSysML)
		if err == nil || !strings.Contains(err.Error(), "import a different set") {
			t.Errorf("a %s stating %s should be refused, got %v", c.other, c.target, err)
		}
	}
}

// TestLibraryIDCollisionIsRefused pins that an element declaring the id the
// norm fixes for a library element it refers to is refused, not merged with it.
func TestLibraryIDCollisionIsRefused(t *testing.T) {
	const src = `package Clash {
    private import ScalarValues::Real;
    part def Impostor { @IdentityMetadata::ElementId { id = "` + realID + `"; } }
    attribute x : Real;
}`
	_, err := convert.Convert("clash.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err == nil || !strings.Contains(err.Error(), "same IRI") {
		t.Fatalf("a document id landing on a library element's IRI should be refused, got %v", err)
	}
}
