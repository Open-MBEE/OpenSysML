package export_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
)

const libraryNamesModel = `package L {
    private import ScalarValues::Real;
    private import ISQ::*;
    part def Car {
        attribute speed : Real;
        attribute weight : MassValue;
        attribute count : ScalarValues::Integer;
    }
}
`

// A converted model names each standard library element it references, marked
// isLibraryElement (KerML Element::isLibraryElement), so every @id the output
// references is an element of it; the library itself is not exported, and the
// document's root namespace does not take the library elements (#731).
func TestConvertNamesTheLibraryElementsItReferences(t *testing.T) {
	out, err := convert.Convert("l.sysml", []byte(libraryNamesModel), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(out, &elements); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	library := map[string]map[string]any{}
	for _, e := range elements {
		id, _ := e["@id"].(string)
		ids[id] = true
		if e["isLibraryElement"] == true {
			library[id] = e
		}
	}
	var dangling []string
	var walk func(any)
	walk = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			if id, ok := v["@id"].(string); ok && len(v) == 1 && !ids[id] {
				dangling = append(dangling, id)
			}
			for _, x := range v {
				walk(x)
			}
		case []any:
			for _, x := range v {
				walk(x)
			}
		}
	}
	for _, e := range elements {
		walk(e)
	}
	if len(dangling) > 0 {
		t.Errorf("the output references ids it does not hold: %v", dangling)
	}
	want := map[string]string{
		"ScalarValues::Real":    "DataType",
		"ScalarValues::Integer": "DataType",
		"ISQBase::MassValue":    "AttributeDefinition",
		"ISQ":                   "LibraryPackage",
	}
	named := map[string]string{}
	memberships := 0
	for _, e := range library {
		for _, owned := range []string{"owner", "owningRelationship", "owningNamespace", "owningMembership"} {
			if _, ok := e[owned]; ok {
				t.Errorf("the library element %v states an owner (%s) in the graph", e["qualifiedName"], owned)
			}
		}
		if e["@type"] == "OwningMembership" {
			memberships++
			continue
		}
		name, _ := e["qualifiedName"].(string)
		named[name], _ = e["@type"].(string)
	}
	for name, typ := range want {
		if named[name] != typ {
			t.Errorf("%s is named as %q, want a %s", name, named[name], typ)
		}
	}
	if memberships != 1 {
		t.Errorf("the membership import of Real names %d library memberships, want 1", memberships)
	}
}

// The names are a record, not declarations: the graph reads back, source text
// stripped, as the notation that produced it, in Turtle and in the API's JSON.
func TestLibraryNamesReadBackAsReferences(t *testing.T) {
	_, back := graphOnlyRoundTrip(t, "l.sysml", []byte(libraryNamesModel))
	if string(back) != libraryNamesModel {
		t.Errorf("the notation read back changed:\n%s", back)
	}
	doc, err := convert.Convert("l.sysml", []byte(libraryNamesModel), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatal(err)
	}
	fromJSON, err := convert.Convert("l.json", doc, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("the API JSON did not read back: %v", err)
	}
	for _, want := range []string{"attribute speed : Real;", "attribute count : ScalarValues::Integer;"} {
		if !strings.Contains(string(fromJSON), want) {
			t.Errorf("the notation should contain %q:\n%s", want, fromJSON)
		}
	}
	if strings.Contains(string(fromJSON), "datatype") || strings.Contains(string(fromJSON), "library package") {
		t.Errorf("a library element was written back as a declaration:\n%s", fromJSON)
	}
}

// A library element named otherwise than the bundled library names its id is
// refused rather than trusted.
func TestLibraryNameThatDisagreesIsRefused(t *testing.T) {
	turtle, err := convert.Convert("l.sysml", []byte(libraryNamesModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatal(err)
	}
	stated := `sysml:qualifiedName "ScalarValues::Real" ;`
	if !strings.Contains(string(turtle), stated) {
		t.Fatalf("the graph no longer names %q", stated)
	}
	renamed := strings.Replace(string(turtle), stated, `sysml:qualifiedName "ScalarValues::Rational" ;`, 1)
	if back, err := convert.Convert("l.ttl", []byte(renamed), convert.FormatTurtle, convert.FormatSysML); err == nil {
		t.Errorf("a misnamed library element was converted:\n%s", back)
	} else if !strings.Contains(err.Error(), "library") {
		t.Errorf("refused for another reason: %v", err)
	}
}

// A library membership owns the element the library says its id owns: one
// whose memberElement is another library element is refused, not read as an
// import of either.
func TestLibraryMembershipOwningAnotherElementIsRefused(t *testing.T) {
	turtle, err := convert.Convert("l.sysml", []byte(libraryNamesModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatal(err)
	}
	real := "sysml:memberElement <urn:sysmlv2:element:14c0aa22-5489-59b5-b438-ded26e83ba31> ;\n    sysml:isLibraryElement"
	if !strings.Contains(string(turtle), real) {
		t.Fatalf("the graph no longer states the library membership of Real")
	}
	integer := strings.Index(string(turtle), `sysml:qualifiedName "ScalarValues::Integer"`)
	if integer < 0 {
		t.Fatal("the graph no longer names ScalarValues::Integer")
	}
	start := strings.LastIndex(string(turtle[:integer]), "\n\n") + 2
	subject := strings.SplitN(string(turtle[start:]), "\n", 2)[0]
	swapped := strings.Replace(string(turtle), real, "sysml:memberElement "+subject+" ;\n    sysml:isLibraryElement", 1)
	if back, err := convert.Convert("l.ttl", []byte(swapped), convert.FormatTurtle, convert.FormatSysML); err == nil {
		t.Errorf("a library membership owning another element was converted:\n%s", back)
	} else if !strings.Contains(err.Error(), "library membership") {
		t.Errorf("refused for another reason: %v", err)
	}
}

// isLibraryElement is an xsd:boolean, so "1" marks a library element as
// "true" does: the element is still a reference, not a declaration.
func TestLibraryMarkerReadsEitherBooleanSpelling(t *testing.T) {
	turtle, err := convert.Convert("l.sysml", []byte(libraryNamesModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatal(err)
	}
	marked := `sysml:isLibraryElement "true"^^xsd:boolean`
	if !strings.Contains(string(turtle), marked) {
		t.Fatalf("the graph no longer marks library elements")
	}
	ones := strings.ReplaceAll(string(turtle), marked, `sysml:isLibraryElement "1"^^xsd:boolean`)
	back, err := convert.Convert("l.ttl", []byte(ones), convert.FormatTurtle, convert.FormatSysML)
	if err != nil {
		t.Fatalf("a graph marking library elements with 1 was refused: %v", err)
	}
	if strings.Contains(string(back), "datatype") || strings.Contains(string(back), "library package") {
		t.Errorf("a library element marked with 1 was written back as a declaration:\n%s", back)
	}
}
