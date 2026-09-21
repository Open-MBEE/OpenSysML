package export_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// referenceFixture declares the shapes whose reference-valued properties the
// graph links: a type, a `first` start, a `then` source, a chain target, a
// referent, a function, a library metadata definition, both import kinds, and
// names nothing declares.
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
		"sysml:importedMembership elmt:Refs__Vehicle_om ;",
		// An import written through an alias imports the alias.
		"sysml:importedMembership elmt:Refs__Motor ;",
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
// references as name literals and imports as an abstract sysml:Import whose
// kind is a flag. Both read back as the notation that produced them.
func TestLegacyReferenceGraphsStillRead(t *testing.T) {
	graph, err := convert.Convert("refs.sysml", []byte(referenceFixture), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	legacy := string(withoutTriples(t, graph, "sysx:sourceText"))
	for _, edit := range [][2]string{
		{link("sysml:type", massValueID), `sysml:type "MassValue"`},
		{"sysml:type elmt:Refs__Engine", `sysml:type "Engine"`},
		{link("sysml:sourceFeature", actionStartID), `sysml:sourceFeature "start"`},
		{link("sysml:importedNamespace", scalarValuesID), `sysml:importedNamespace "ScalarValues"`},
		{link("sysml:importedMembership", massValueMembershipID), `sysml:importedNamespace "ISQ::MassValue"`},
		{"sysml:importedMembership elmt:Refs__Motor", `sysml:importedNamespace "Motor"`},
		{"a sysml:NamespaceImport ;", `a sysml:Import ;
    sysx:isNamespaceImport "true"^^xsd:boolean ;`},
		{"a sysml:MembershipImport ;", "a sysml:Import ;"},
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
