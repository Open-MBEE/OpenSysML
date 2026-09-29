package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

// payloadModel declares the port and feature definitions the payload tests' flow
// and message ends resolve against.
const payloadModel = `package P {
	port def FuelPort { out feature fuel : Fuel; }
	port def I { out feature i : Prio; }
	port def J { in feature j : Prio; }
	attribute def Prio;
	item def Fuel;
	part a { port p : FuelPort; }
	part b { port p : FuelPort; }
	part x { port i : I; }
	part y { port j : J; }
	message m;
	%s
}
`

// apiJSONTypes indexes an api-json document's elements by their @id.
func apiJSONTypes(t *testing.T, document []byte) map[string]map[string]any {
	t.Helper()
	var elements []map[string]any
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]any{}
	for _, el := range elements {
		if id, ok := el["@id"].(string); ok {
			out[id] = el
		}
	}
	return out
}

// apiJSONRef returns the @id a single-valued member names, or "".
func apiJSONRef(v any) string {
	if m, ok := v.(map[string]any); ok {
		if id, ok := m["@id"].(string); ok {
			return id
		}
	}
	return ""
}

// apiJSONRefs returns the @ids a list-valued member names, in order.
func apiJSONRefs(v any) []string {
	var out []string
	if list, ok := v.([]any); ok {
		for _, item := range list {
			out = append(out, apiJSONRef(item))
		}
	}
	return out
}

// The payload a flow's `of` clause names is a PayloadFeature the flow owns
// through a FeatureMembership ahead of its two end memberships — the order the
// grammar states them in — typed by the feature the clause references.
func TestFlowPayloadIsAFeatureMembershipOwnedPayloadFeature(t *testing.T) {
	src := strings.Replace(payloadModel, "%s", "\tflow of Fuel from a.p.fuel to b.p.fuel;\n", 1)
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	for _, want := range []string{
		"a sysml:PayloadFeature ;",
		"a sysml:FeatureMembership ;",
		"sysml:ownedFeatureMembership expr:P___4010_ppayload_om, expr:P___4010_pend0_om, expr:P___4010_pend1_om",
		"sysml:memberElement expr:P___4010_ppayload",
		"sysml:owningMembership expr:P___4010_ppayload_om",
		"a sysml:FeatureTyping ;",
		"sysml:type elmt:P__Fuel",
	} {
		if !strings.Contains(string(turtle), want) {
			t.Errorf("the graph does not state %q:\n%s", want, turtle)
		}
	}
	for _, reject := range []string{
		"sysx:payload",
		"expr:P___4010_ppayload\n    a sysml:FeatureReferenceExpression ;",
		"sysml:ownedFeatureMembership expr:P___4010_pend0_om, expr:P___4010_pend1_om, expr:P___4010_ppayload_om",
	} {
		if strings.Contains(string(turtle), reject) {
			t.Errorf("the graph states %q:\n%s", reject, turtle)
		}
	}
	// The api-json element form carries the same shape.
	document, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	elements := apiJSONTypes(t, document)
	payload, ok := elements["P___4010_ppayload"]
	if !ok || payload["@type"] != "PayloadFeature" {
		t.Fatalf("no PayloadFeature P___4010_ppayload in:\n%s", document)
	}
	membership, ok := elements["P___4010_ppayload_om"]
	if !ok || membership["@type"] != "FeatureMembership" {
		t.Fatalf("no FeatureMembership P___4010_ppayload_om in:\n%s", document)
	}
	if apiJSONRef(membership["memberElement"]) != "P___4010_ppayload" {
		t.Errorf("the membership does not own the payload feature: %v", membership)
	}
	flow := elements["P___4010"]
	if got := apiJSONRefs(flow["ownedFeatureMembership"]); len(got) != 3 ||
		got[0] != "P___4010_ppayload_om" || got[1] != "P___4010_pend0_om" || got[2] != "P___4010_pend1_om" {
		t.Errorf("the payload membership does not precede the end memberships: %v", got)
	}
	if _, bad := payload["FeatureReferenceExpression"]; bad {
		t.Errorf("the payload is a reference expression: %v", payload)
	}
	typing, ok := elements["P___4010_ppayload_ft0"]
	if !ok || typing["@type"] != "FeatureTyping" || apiJSONRef(typing["type"]) != "P__Fuel" {
		t.Errorf("the payload is not typed by Fuel: %v", typing)
	}
}

// A declared `of name : Type` payload is the declaration itself, the
// PayloadFeature element it names, owned by the flow's FeatureMembership.
func TestFlowDeclaredPayloadIsThePayloadFeature(t *testing.T) {
	src := strings.Replace(payloadModel, "%s",
		"\tflow of fuel : Fuel from x.i to y.j {\n\t\tattribute w : Prio;\n\t}\n", 1)
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	for _, want := range []string{
		"a sysml:PayloadFeature ;",
		"sysml:qualifiedName \"P::@10::fuel\"",
		"sysml:declaredName \"fuel\"",
		"sysml:owningMembership elmt:",
	} {
		if !strings.Contains(string(turtle), want) {
			t.Errorf("the graph does not state %q:\n%s", want, turtle)
		}
	}
	if strings.Contains(string(turtle), "sysx:payload") {
		t.Errorf("the graph still states sysx:payload:\n%s", turtle)
	}
	_, back := graphOnlyRoundTrip(t, "m.sysml", []byte(src))
	if !strings.Contains(string(back), "flow of fuel : Fuel from x.i to y.j {") {
		t.Errorf("the declared payload did not come back from the graph alone:\n%s", back)
	}
	if !strings.Contains(string(back), "attribute w : Prio;") {
		t.Errorf("the flow's body member did not come back:\n%s", back)
	}
}

// A message's payload is the same PayloadFeature member, and a value the
// payload declares reads back with it.
func TestMessagePayloadFeature(t *testing.T) {
	for _, tc := range []struct {
		line, want string
	}{
		{"\tmessage m1 of cmd : Prio from x.i to y.j;\n", "m1 of cmd : Prio"},
		{"\tmessage of cmd : Prio = m.cmd from x.i to y.j;\n", "of cmd : Prio = m.cmd"},
	} {
		src := strings.Replace(payloadModel, "%s", tc.line, 1)
		turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
		if err != nil {
			t.Fatalf("%s: to turtle: %v", tc.line, err)
		}
		if !strings.Contains(string(turtle), "a sysml:PayloadFeature ;") ||
			strings.Contains(string(turtle), "sysx:payload") {
			t.Errorf("%s: the payload is no FeatureMembership-owned PayloadFeature:\n%s", tc.line, turtle)
		}
		_, back := graphOnlyRoundTrip(t, "m.sysml", []byte(src))
		if !strings.Contains(string(back), tc.want) {
			t.Errorf("%s: the payload did not come back from the graph alone:\n%s", tc.line, back)
		}
	}
}

// The payload form survives a graph stripped of the layout text, over the
// Turtle and the api-json hops alike.
func TestPayloadFeatureGraphOnlyRoundTrip(t *testing.T) {
	for _, tc := range []struct{ line, want string }{
		{"\tflow of Fuel from a.p.fuel to b.p.fuel;\n", "of Fuel from a.p.fuel to b.p.fuel"},
		{"\tflow of Fuel[1] from x.i to y.j;\n", "of Fuel[1] from x.i to y.j"},
		{"\tflow of x.i from x.i to y.j;\n", "of x.i from x.i to y.j"},
		{"\tflow f of Fuel;\n", "flow f of Fuel"},
	} {
		src := strings.Replace(payloadModel, "%s", tc.line, 1)
		_, back := graphOnlyRoundTrip(t, "m.sysml", []byte(src))
		if !strings.Contains(string(back), tc.want) {
			t.Errorf("%s: did not come back as %q from the graph alone:\n%s", tc.line, tc.want, back)
		}
		// The api-json element form reads the same back.
		document, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON)
		if err != nil {
			t.Fatalf("%s: to api-json: %v", tc.line, err)
		}
		jsonBack, err := convert.Convert("m.json", document, convert.FormatAPIJSON, convert.FormatSysML)
		if err != nil {
			t.Fatalf("%s: back to notation from api-json: %v", tc.line, err)
		}
		if !strings.Contains(string(jsonBack), tc.want) {
			t.Errorf("%s: did not come back as %q via api-json:\n%s", tc.line, tc.want, jsonBack)
		}
	}
}

// An older graph's sysx:payload expression still reads as the `of` clause.
func TestFlowLegacyPayloadExpressionReadsBack(t *testing.T) {
	turtle := `@prefix elmt: <urn:sysmlv2:element:> .
@prefix expr: <urn:opensysml:expr:> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix sysx: <urn:opensysml:sysml:> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

elmt:P
    a sysml:Package ;
    sysml:qualifiedName "P" ;
    sysml:elementId "P" ;
    sysx:memberIndex "0"^^xsd:integer ;
    sysx:sourceLanguage "sysml" ;
    sysml:declaredName "P" ;
    sysx:hasBody "true"^^xsd:boolean ;
    sysml:ownedMember elmt:P__Fuel, elmt:P___401 ;
    sysml:ownedMembership elmt:P__Fuel_om, elmt:P___401_om ;
    sysml:ownedRelationship elmt:P__Fuel_om, elmt:P___401_om .

elmt:P__Fuel
    a sysml:ItemDefinition ;
    sysml:qualifiedName "P::Fuel" ;
    sysml:elementId "P__Fuel" ;
    sysx:memberIndex "0"^^xsd:integer ;
    sysml:owningNamespace elmt:P ;
    sysml:owner elmt:P ;
    sysml:owningRelationship elmt:P__Fuel_om ;
    sysml:owningMembership elmt:P__Fuel_om ;
    sysml:declaredName "Fuel" ;
    sysx:hasBody "false"^^xsd:boolean .

elmt:P__Fuel_om
    a sysml:OwningMembership ;
    sysml:elementId "P__Fuel_om" ;
    sysml:owner elmt:P ;
    sysml:memberElement elmt:P__Fuel ;
    sysml:ownedMemberElement elmt:P__Fuel ;
    sysml:ownedRelatedElement elmt:P__Fuel ;
    sysml:owningRelatedElement elmt:P ;
    sysml:membershipOwningNamespace elmt:P .

elmt:P___401
    a sysml:FlowUsage ;
    sysml:qualifiedName "P::@1" ;
    sysml:elementId "P___401" ;
    sysx:memberIndex "1"^^xsd:integer ;
    sysml:owner elmt:P ;
    sysml:owningRelationship elmt:P___401_om ;
    sysml:owningMembership elmt:P___401_om ;
    sysx:declaredKeyword "flow" ;
    sysx:endForm "fromTo" ;
    sysml:connectorEnd expr:P___401_pend0, expr:P___401_pend1 ;
    sysml:ownedRelationship expr:P___401_pend0_om, expr:P___401_pend1_om, expr:P___401_ppayload_om ;
    sysml:ownedMembership expr:P___401_pend0_om, expr:P___401_pend1_om, expr:P___401_ppayload_om ;
    sysml:ownedFeatureMembership expr:P___401_pend0_om, expr:P___401_pend1_om, expr:P___401_ppayload_om ;
    sysml:ownedFeature expr:P___401_pend0, expr:P___401_pend1 ;
    sysml:ownedEndFeature expr:P___401_pend0, expr:P___401_pend1 ;
    sysml:relatedFeature "a.p.fuel", "b.p.fuel" ;
    sysml:sourceFeature "a.p.fuel" ;
    sysml:targetFeature "b.p.fuel" ;
    sysx:payload expr:P___401_ppayload ;
    sysx:hasBody "false"^^xsd:boolean .

elmt:P___401_om
    a sysml:OwningMembership ;
    sysml:elementId "P___401_om" ;
    sysml:owner elmt:P ;
    sysml:memberElement elmt:P___401 ;
    sysml:ownedMemberElement elmt:P___401 ;
    sysml:ownedRelatedElement elmt:P___401 ;
    sysml:owningRelatedElement elmt:P ;
    sysml:membershipOwningNamespace elmt:P .

expr:P___401_pend0
    a sysml:ReferenceUsage ;
    sysml:elementId "P___401_pend0" ;
    sysml:isEnd "true"^^xsd:boolean ;
    sysml:owner elmt:P___401 ;
    sysml:owningRelationship expr:P___401_pend0_om ;
    sysml:owningMembership expr:P___401_pend0_om ;
    sysx:sourceText "a.p.fuel" ;
    sysml:ownedReferenceSubsetting expr:P___401_pend0_prs ;
    sysml:ownedSubsetting expr:P___401_pend0_prs ;
    sysml:ownedSpecialization expr:P___401_pend0_prs ;
    sysml:ownedRelationship expr:P___401_pend0_prs .

expr:P___401_pend0_om
    a sysml:EndFeatureMembership ;
    sysml:elementId "P___401_pend0_om" ;
    sysml:owner elmt:P___401 ;
    sysml:memberElement expr:P___401_pend0 ;
    sysml:ownedMemberElement expr:P___401_pend0 ;
    sysml:ownedRelatedElement expr:P___401_pend0 ;
    sysml:owningRelatedElement elmt:P___401 ;
    sysml:membershipOwningNamespace elmt:P___401 .

expr:P___401_pend0_prs
    a sysml:ReferenceSubsetting ;
    sysml:elementId "P___401_pend0_prs" ;
    sysml:referencingFeature expr:P___401_pend0 ;
    sysml:subsettingFeature expr:P___401_pend0 ;
    sysml:owningFeature expr:P___401_pend0 ;
    sysml:specific expr:P___401_pend0 ;
    sysml:source expr:P___401_pend0 ;
    sysml:owningRelatedElement expr:P___401_pend0 ;
    sysml:referencedFeature "a.p.fuel" ;
    sysml:subsettedFeature "a.p.fuel" ;
    sysml:general "a.p.fuel" ;
    sysml:target "a.p.fuel" ;
    sysml:owner expr:P___401_pend0 ;
    sysml:relatedElement expr:P___401_pend0, "a.p.fuel" .


expr:P___401_pend1
    a sysml:ReferenceUsage ;
    sysml:elementId "P___401_pend1" ;
    sysml:isEnd "true"^^xsd:boolean ;
    sysml:owner elmt:P___401 ;
    sysml:owningRelationship expr:P___401_pend1_om ;
    sysml:owningMembership expr:P___401_pend1_om ;
    sysx:sourceText "b.p.fuel" ;
    sysml:ownedReferenceSubsetting expr:P___401_pend1_prs ;
    sysml:ownedSubsetting expr:P___401_pend1_prs ;
    sysml:ownedSpecialization expr:P___401_pend1_prs ;
    sysml:ownedRelationship expr:P___401_pend1_prs .

expr:P___401_pend1_om
    a sysml:EndFeatureMembership ;
    sysml:elementId "P___401_pend1_om" ;
    sysml:owner elmt:P___401 ;
    sysml:memberElement expr:P___401_pend1 ;
    sysml:ownedMemberElement expr:P___401_pend1 ;
    sysml:ownedRelatedElement expr:P___401_pend1 ;
    sysml:owningRelatedElement elmt:P___401 ;
    sysml:membershipOwningNamespace elmt:P___401 .

expr:P___401_pend1_prs
    a sysml:ReferenceSubsetting ;
    sysml:elementId "P___401_pend1_prs" ;
    sysml:referencingFeature expr:P___401_pend1 ;
    sysml:subsettingFeature expr:P___401_pend1 ;
    sysml:owningFeature expr:P___401_pend1 ;
    sysml:specific expr:P___401_pend1 ;
    sysml:source expr:P___401_pend1 ;
    sysml:owningRelatedElement expr:P___401_pend1 ;
    sysml:referencedFeature "b.p.fuel" ;
    sysml:subsettedFeature "b.p.fuel" ;
    sysml:general "b.p.fuel" ;
    sysml:target "b.p.fuel" ;
    sysml:owner expr:P___401_pend1 ;
    sysml:relatedElement expr:P___401_pend1, "b.p.fuel" .


expr:P___401_ppayload
    a sysml:FeatureReferenceExpression ;
    sysml:elementId "P___401_ppayload" ;
    sysx:sourceText "Fuel" ;
    sysml:referent elmt:P__Fuel ;
    sysml:owner elmt:P___401 ;
    sysml:owningRelationship expr:P___401_ppayload_om ;
    sysml:owningMembership expr:P___401_ppayload_om .

expr:P___401_ppayload_om
    a sysml:OwningMembership ;
    sysml:elementId "P___401_ppayload_om" ;
    sysml:owner elmt:P___401 ;
    sysml:memberElement expr:P___401_ppayload ;
    sysml:ownedMemberElement expr:P___401_ppayload ;
    sysml:ownedRelatedElement expr:P___401_ppayload ;
    sysml:owningRelatedElement elmt:P___401 ;
    sysml:membershipOwningNamespace elmt:P___401 .
`
	back, err := convert.Convert("m.ttl", []byte(turtle), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "flow of Fuel from a.p.fuel to b.p.fuel;") {
		t.Errorf("the legacy payload did not come back:\n%s", back)
	}
}

// A `#` prefix ahead of a dependency is owned through an Annotation; ahead of
// any other declaration it is the usual OwningMembership.
func TestDependencyPrefixIsAnAnnotation(t *testing.T) {
	src := `package D {
	metadata def Tag;
	metadata def Other;
	part a; part b;
	#Tag dependency x to y;
	#Tag #Other dependency d from a to b;
	#Tag part z;
}
`
	turtle, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatalf("to turtle: %v", err)
	}
	for _, want := range []string{
		"a sysml:Annotation ;",
		"sysml:ownedAnnotation elmt:",
		"sysml:annotatingElement elmt:",
		"sysml:annotatedElement elmt:",
		"sysml:owningAnnotatedElement elmt:",
	} {
		if !strings.Contains(string(turtle), want) {
			t.Errorf("the graph does not state %q:\n%s", want, turtle)
		}
	}
	for _, reject := range []string{
		"a sysml:OwningMembership ;\n    sysml:elementId \"D___404___400_om\"",
		"a sysml:OwningMembership ;\n    sysml:elementId \"D__d___400_om\"",
	} {
		if strings.Contains(string(turtle), reject) {
			t.Errorf("a dependency's prefix is no membership, yet the graph states %q:\n%s", reject, turtle)
		}
	}
	// The metadata usages keep their type, keyword and identity.
	for _, want := range []string{
		"a sysml:MetadataUsage ;",
		"sysx:declaredKeyword \"#\"",
		"sysml:type elmt:D__Tag",
		"sysml:type elmt:D__Other",
	} {
		if !strings.Contains(string(turtle), want) {
			t.Errorf("the metadata usage lost %q:\n%s", want, turtle)
		}
	}
	// The control declaration's prefix is still an OwningMembership.
	if !strings.Contains(string(turtle), "a sysml:OwningMembership ;\n    sysml:elementId \"D__z___400_om\"") {
		t.Errorf("#Tag part z lost its OwningMembership:\n%s", turtle)
	}
	// The api-json element form carries the annotation.
	document, err := convert.Convert("m.sysml", []byte(src), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatalf("to api-json: %v", err)
	}
	elements := apiJSONTypes(t, document)
	var annotations int
	for _, el := range elements {
		if el["@type"] == "Annotation" {
			annotations++
			if apiJSONRef(el["annotatingElement"]) == "" || apiJSONRef(el["annotatedElement"]) == "" {
				t.Errorf("the annotation does not relate its ends: %v", el)
			}
		}
	}
	if annotations != 3 {
		t.Errorf("expected 3 annotations (two dependencies' three prefixes), got %d:\n%s", annotations, document)
	}
	// Both prefixes read back.
	_, back := graphOnlyRoundTrip(t, "m.sysml", []byte(src))
	for _, want := range []string{"#Tag dependency from x to y;", "#Tag #Other dependency d from a to b;", "#Tag part z;"} {
		if !strings.Contains(string(back), want) {
			t.Errorf("%q did not come back from the graph alone:\n%s", want, back)
		}
	}
}

// An older graph that owns a dependency's prefix metadata through an
// OwningMembership still reads it as a `#` prefix.
func TestDependencyLegacyPrefixMembershipReadsBack(t *testing.T) {
	turtle := `@prefix elmt: <urn:sysmlv2:element:> .
@prefix json: <urn:sysmlv2:annotation:json:> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix sysx: <urn:opensysml:sysml:> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

elmt:D
    a sysml:Package ;
    sysml:qualifiedName "D" ;
    sysml:elementId "D" ;
    sysx:memberIndex "0"^^xsd:integer ;
    sysx:sourceLanguage "sysml" ;
    sysml:declaredName "D" ;
    sysx:hasBody "true"^^xsd:boolean ;
    sysml:ownedMember elmt:D__Tag, elmt:D__a, elmt:D__b, elmt:D___403 ;
    sysml:ownedMembership elmt:D__Tag_om, elmt:D__a_om, elmt:D__b_om, elmt:D___403_om ;
    sysml:ownedRelationship elmt:D__Tag_om, elmt:D__a_om, elmt:D__b_om, elmt:D___403_om ;
    sysx:sourceText "package D {\n\tmetadata def Tag;\n\tpart a; part b;\n\t#Tag dependency x to y;\n}\n" ;
    json:ownedMember "[{\"@id\":\"D__Tag\"},{\"@id\":\"D__a\"},{\"@id\":\"D__b\"},{\"@id\":\"D___403\"}]" ;
    json:ownedMembership "[{\"@id\":\"D__Tag_om\"},{\"@id\":\"D__a_om\"},{\"@id\":\"D__b_om\"},{\"@id\":\"D___403_om\"}]" ;
    json:ownedRelationship "[{\"@id\":\"D__Tag_om\"},{\"@id\":\"D__a_om\"},{\"@id\":\"D__b_om\"},{\"@id\":\"D___403_om\"}]" .

elmt:D__Tag
    a sysml:MetadataDefinition ;
    sysml:qualifiedName "D::Tag" ;
    sysml:elementId "D__Tag" ;
    sysx:memberIndex "0"^^xsd:integer ;
    sysml:owningNamespace elmt:D ;
    sysml:owner elmt:D ;
    sysml:owningRelationship elmt:D__Tag_om ;
    sysml:owningMembership elmt:D__Tag_om ;
    sysml:declaredName "Tag" ;
    sysx:hasBody "false"^^xsd:boolean .

elmt:D__Tag_om
    a sysml:OwningMembership ;
    sysml:elementId "D__Tag_om" ;
    sysml:owner elmt:D ;
    sysml:memberElement elmt:D__Tag ;
    sysml:ownedMemberElement elmt:D__Tag ;
    sysml:ownedRelatedElement elmt:D__Tag ;
    sysml:owningRelatedElement elmt:D ;
    sysml:membershipOwningNamespace elmt:D .

elmt:D__a
    a sysml:PartUsage ;
    sysml:qualifiedName "D::a" ;
    sysml:elementId "D__a" ;
    sysx:memberIndex "1"^^xsd:integer ;
    sysml:owningNamespace elmt:D ;
    sysml:owner elmt:D ;
    sysml:owningRelationship elmt:D__a_om ;
    sysml:owningMembership elmt:D__a_om ;
    sysml:declaredName "a" ;
    sysx:hasBody "false"^^xsd:boolean .

elmt:D__a_om
    a sysml:OwningMembership ;
    sysml:elementId "D__a_om" ;
    sysml:owner elmt:D ;
    sysml:memberElement elmt:D__a ;
    sysml:ownedMemberElement elmt:D__a ;
    sysml:ownedRelatedElement elmt:D__a ;
    sysml:owningRelatedElement elmt:D ;
    sysml:membershipOwningNamespace elmt:D .

elmt:D__b
    a sysml:PartUsage ;
    sysml:qualifiedName "D::b" ;
    sysml:elementId "D__b" ;
    sysx:memberIndex "2"^^xsd:integer ;
    sysml:owningNamespace elmt:D ;
    sysml:owner elmt:D ;
    sysml:owningRelationship elmt:D__b_om ;
    sysml:owningMembership elmt:D__b_om ;
    sysml:declaredName "b" ;
    sysx:hasBody "false"^^xsd:boolean .

elmt:D__b_om
    a sysml:OwningMembership ;
    sysml:elementId "D__b_om" ;
    sysml:owner elmt:D ;
    sysml:memberElement elmt:D__b ;
    sysml:ownedMemberElement elmt:D__b ;
    sysml:ownedRelatedElement elmt:D__b ;
    sysml:owningRelatedElement elmt:D ;
    sysml:membershipOwningNamespace elmt:D .

elmt:D___403
    a sysml:Dependency ;
    sysml:qualifiedName "D::@3" ;
    sysml:elementId "D___403" ;
    sysx:memberIndex "3"^^xsd:integer ;
    sysml:owningNamespace elmt:D ;
    sysml:owner elmt:D ;
    sysml:owningRelationship elmt:D___403_om ;
    sysml:owningMembership elmt:D___403_om ;
    sysml:client "x" ;
    sysml:supplier "y" ;
    sysml:ownedRelationship elmt:D___403___400_om ;
    sysx:hasBody "false"^^xsd:boolean .

elmt:D___403_om
    a sysml:OwningMembership ;
    sysml:elementId "D___403_om" ;
    sysml:owner elmt:D ;
    sysml:memberElement elmt:D___403 ;
    sysml:ownedMemberElement elmt:D___403 ;
    sysml:ownedRelatedElement elmt:D___403 ;
    sysml:owningRelatedElement elmt:D ;
    sysml:membershipOwningNamespace elmt:D .

elmt:D___403___400
    a sysml:MetadataUsage ;
    sysml:qualifiedName "D::@3::@0" ;
    sysml:elementId "D___403___400" ;
    sysx:memberIndex "0"^^xsd:integer ;
    sysml:owner elmt:D___403 ;
    sysml:owningRelationship elmt:D___403___400_om ;
    sysml:owningMembership elmt:D___403___400_om ;
    sysx:declaredKeyword "#" ;
    sysml:type elmt:D__Tag ;
    sysx:hasBody "false"^^xsd:boolean ;
    sysml:ownedTyping elmt:D___403___400_ft0 ;
    sysml:ownedSpecialization elmt:D___403___400_ft0 ;
    sysml:ownedRelationship elmt:D___403___400_ft0 .

elmt:D___403___400_om
    a sysml:OwningMembership ;
    sysml:elementId "D___403___400_om" ;
    sysml:owner elmt:D___403 ;
    sysml:memberElement elmt:D___403___400 ;
    sysml:ownedMemberElement elmt:D___403___400 ;
    sysml:ownedRelatedElement elmt:D___403___400 ;
    sysml:owningRelatedElement elmt:D___403 .

elmt:D___403___400_ft0
    a sysml:FeatureTyping ;
    sysml:elementId "D___403___400_ft0" ;
    sysml:typedFeature elmt:D___403___400 ;
    sysml:owningFeature elmt:D___403___400 ;
    sysml:specific elmt:D___403___400 ;
    sysml:source elmt:D___403___400 ;
    sysml:owningRelatedElement elmt:D___403___400 ;
    sysml:type elmt:D__Tag ;
    sysml:general elmt:D__Tag ;
    sysml:target elmt:D__Tag ;
    sysml:owner elmt:D___403___400 ;
    sysml:relatedElement elmt:D___403___400, elmt:D__Tag ;
    json:relatedElement "[{\"@id\":\"D___403___400\"},{\"@id\":\"D__Tag\"}]" .
`
	back, err := convert.Convert("m.ttl", []byte(turtle), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("back to notation: %v", err)
	}
	if !strings.Contains(string(back), "#Tag dependency") {
		t.Errorf("the legacy prefix membership did not come back:\n%s", back)
	}
}
