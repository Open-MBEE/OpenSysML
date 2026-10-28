package export_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

const libraryUnitsModel = `package q {
 private import ISQ::*; private import SI::*; private import SIPrefixes::*;
 part def T { attribute period : DurationValue; }
 part t : T { :>> period = 20 [milli*s]; }
}`

// These six identities and stub keys are the v0.10.0 export of libraryUnitsModel.
var libraryUnitsReferences = []export.APIJSONLibraryReference{
	{ID: "0c539f7c-20a1-520d-bae0-f3f855517aea", Type: "AttributeUsage", QualifiedName: "SI::second"},
	{ID: "2a413ac9-972d-5182-afd3-46cdde4e0eba", Type: "LibraryPackage", QualifiedName: "SI"},
	{ID: "3fcc2ef4-a31c-522b-b69d-717d74bccfa6", Type: "LibraryPackage", QualifiedName: "ISQ"},
	{ID: "7e6dd4b3-e45b-5c71-8596-69062380bdc0", Type: "AttributeUsage", QualifiedName: "SIPrefixes::milli"},
	{ID: "c6b04dec-209b-5f14-ba38-451f64821004", Type: "LibraryPackage", QualifiedName: "SIPrefixes"},
	{ID: "ffba2d6a-c7fc-56f5-b6b1-9c8ba78a4568", Type: "AttributeDefinition", QualifiedName: "ISQBase::DurationValue"},
}

func TestAPIJSONLibraryReferencesAndLegacyStubs(t *testing.T) {
	graph, err := convert.SysMLToRDF("units.sysml", []byte(libraryUnitsModel))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := export.WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	var names []export.APIJSONLibraryReference
	withNames, err := export.WriteAPIJSONWithOptions(graph, export.APIJSONOptions{
		LibraryReferences: func(refs []export.APIJSONLibraryReference) { names = refs },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(plain, withNames) {
		t.Fatal("collecting library names changed the default element array")
	}
	if !reflect.DeepEqual(names, libraryUnitsReferences) {
		t.Fatalf("names = %#v", names)
	}
	if bytes.Contains(plain, []byte(`"isLibraryElement": true`)) {
		t.Fatal("default writes library roots")
	}
	legacy, err := export.WriteAPIJSONWithOptions(graph, export.APIJSONOptions{IncludeLibraryStubs: true})
	if err != nil {
		t.Fatal(err)
	}
	var elements, own []map[string]any
	if err := json.Unmarshal(legacy, &elements); err != nil {
		t.Fatal(err)
	}
	var stubs []map[string]any
	for _, element := range elements {
		if element["isLibraryElement"] == true {
			stubs = append(stubs, element)
		} else {
			own = append(own, element)
		}
	}
	declaredNames := []string{"second", "SI", "ISQ", "milli", "SIPrefixes", "DurationValue"}
	want := make([]map[string]any, len(libraryUnitsReferences))
	for i, ref := range libraryUnitsReferences {
		want[i] = map[string]any{"@id": ref.ID, "@type": ref.Type, "qualifiedName": ref.QualifiedName,
			"declaredName": declaredNames[i], "elementId": ref.ID, "isLibraryElement": true}
	}
	if !reflect.DeepEqual(stubs, want) {
		t.Fatalf("legacy stubs = %#v, want %#v", stubs, want)
	}
	var defaultElements []map[string]any
	if err := json.Unmarshal(plain, &defaultElements); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(own, defaultElements) {
		t.Fatal("stub mode changed model elements")
	}
	for _, omitDerived := range []bool{false, true} {
		compact, err := export.WriteAPIJSONCompact(graph, export.CompactAPIJSONOptions{OmitDerived: omitDerived})
		if err != nil {
			t.Fatal(err)
		}
		var document struct {
			LibraryReferences []export.APIJSONLibraryReference `json:"libraryReferences"`
		}
		if err := json.Unmarshal(compact, &document); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(document.LibraryReferences, libraryUnitsReferences) {
			t.Fatalf("compact table = %#v", document.LibraryReferences)
		}
	}
}
