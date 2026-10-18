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

// The API element form carries the model's own elements only: a standard
// library element it references stays a bare `{"@id"}` of its normative id,
// as the OMG API JSON shape has it, since an ownerless element there is a root
// a reader would declare anew. The graph (Turtle) still names each one (#731).
func TestAPIJSONCarriesNoLibraryElements(t *testing.T) {
	out, err := convert.Convert("l.sysml", []byte(libraryNamesModel), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(out, &elements); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, e := range elements {
		id, _ := e["@id"].(string)
		ids[id] = true
		if e["isLibraryElement"] == true {
			t.Errorf("the element form holds the library element %v", e["qualifiedName"])
		}
		if _, owned := e["owner"]; !owned && e["@type"] != "Namespace" {
			t.Errorf("the element form holds an ownerless %v %v, which a reader takes for a root", e["@type"], id)
		}
	}
	unheld := map[string]bool{}
	var walk func(any)
	walk = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			if id, ok := v["@id"].(string); ok && len(v) == 1 && !ids[id] {
				unheld[id] = true
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
	turtle, err := convert.Convert("l.sysml", []byte(libraryNamesModel), convert.FormatSysML, convert.FormatTurtle)
	if err != nil {
		t.Fatal(err)
	}
	// Each id the element form leaves unheld is one the graph names as a library element.
	named := 0
	for id := range unheld {
		if !strings.Contains(string(turtle), "<urn:sysmlv2:element:"+id+">\n    a sysml:") &&
			!strings.Contains(string(turtle), "\nelmt:"+id+"\n    a sysml:") {
			t.Errorf("the element form references %s, which is no library element the graph names", id)
		}
		named++
	}
	// Real, Integer, MassValue, the ISQ package and the membership import of Real.
	if named != 5 {
		t.Errorf("the element form leaves %d library ids unheld, want 5", named)
	}
	for _, want := range []string{
		`sysml:qualifiedName "ScalarValues::Real" ;`, `sysml:qualifiedName "ISQBase::MassValue" ;`,
		`sysml:qualifiedName "ISQ" ;`, `sysml:isLibraryElement "true"^^xsd:boolean`,
	} {
		if !strings.Contains(string(turtle), want) {
			t.Errorf("the graph should still name the library element: %s", want)
		}
	}
}

// An older element form that named its library elements as ownerless
// isLibraryElement stubs still reads: the stubs are references, not declarations.
func TestAPIJSONWithLibraryStubsStillReads(t *testing.T) {
	out, err := convert.Convert("l.sysml", []byte(libraryNamesModel), convert.FormatSysML, convert.FormatAPIJSON)
	if err != nil {
		t.Fatal(err)
	}
	var elements []json.RawMessage
	if err := json.Unmarshal(out, &elements); err != nil {
		t.Fatal(err)
	}
	for _, stub := range []string{
		`{"@type":"DataType","@id":"14c0aa22-5489-59b5-b438-ded26e83ba31","elementId":"14c0aa22-5489-59b5-b438-ded26e83ba31","qualifiedName":"ScalarValues::Real","declaredName":"Real","isLibraryElement":true}`,
		`{"@type":"OwningMembership","@id":"ab72a695-5fe9-58a3-9d48-9e9a8711862d","elementId":"ab72a695-5fe9-58a3-9d48-9e9a8711862d","memberElement":{"@id":"14c0aa22-5489-59b5-b438-ded26e83ba31"},"isLibraryElement":true}`,
	} {
		elements = append(elements, json.RawMessage(stub))
	}
	older, err := json.Marshal(elements)
	if err != nil {
		t.Fatal(err)
	}
	back, err := convert.Convert("l.json", older, convert.FormatAPIJSON, convert.FormatSysML)
	if err != nil {
		t.Fatalf("the older element form did not read back: %v", err)
	}
	if string(back) != libraryNamesModel {
		t.Errorf("the notation read back changed:\n%s", back)
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
	realMembership := "sysml:memberElement <urn:sysmlv2:element:14c0aa22-5489-59b5-b438-ded26e83ba31> ;\n    sysml:isLibraryElement"
	if !strings.Contains(string(turtle), realMembership) {
		t.Fatalf("the graph no longer states the library membership of Real")
	}
	integer := strings.Index(string(turtle), `sysml:qualifiedName "ScalarValues::Integer"`)
	if integer < 0 {
		t.Fatal("the graph no longer names ScalarValues::Integer")
	}
	start := strings.LastIndex(string(turtle[:integer]), "\n\n") + 2
	subject := strings.SplitN(string(turtle[start:]), "\n", 2)[0]
	swapped := strings.Replace(string(turtle), realMembership, "sysml:memberElement "+subject+" ;\n    sysml:isLibraryElement", 1)
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
