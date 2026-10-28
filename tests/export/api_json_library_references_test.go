package export_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/convert"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
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
		read, err := export.ReadAPIJSON(compact)
		if err != nil {
			t.Fatal(err)
		}
		array, err := export.WriteAPIJSON(read)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(array, []byte(`"isLibraryElement": true`)) {
			t.Fatal("compact metadata became model roots")
		}
		again, err := export.WriteAPIJSONCompact(read, export.CompactAPIJSONOptions{OmitDerived: omitDerived})
		if err != nil {
			t.Fatal(err)
		}
		var before, after any
		if err := json.Unmarshal(compact, &before); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(again, &after); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatal("compact read/write changed the document")
		}
	}
}

func TestAPIJSONLibraryStubRejectsIncompleteGraph(t *testing.T) {
	for _, typeTriple := range []string{"", ` a <https://example.org/Wrong> ;`, ` a sysml:Class, sysml:Feature ;`} {
		graph, err := rdf.ParseTurtle([]byte(`@prefix sysml: <` + rdf.SysML + `> . <urn:sysmlv2:element:L>` + typeTriple + ` sysml:isLibraryElement "true"^^<` + rdf.XSD + `boolean> .`))
		if err != nil {
			t.Fatal(err)
		}
		_, err = export.WriteAPIJSONWithOptions(graph, export.APIJSONOptions{IncludeLibraryStubs: true})
		var unsupported *export.UnsupportedError
		if !errors.As(err, &unsupported) {
			t.Fatalf("type %q: got %v, want UnsupportedError", typeTriple, err)
		}
	}
}

func TestAPIJSONCompactLibraryTableIsLiteralMetadata(t *testing.T) {
	graph, err := convert.SysMLToRDF("units.sysml", []byte(libraryUnitsModel))
	if err != nil {
		t.Fatal(err)
	}
	inner, outer := 0, 0
	compact, err := convert.FromGraphWith(graph, convert.FormatAPIJSON, convert.Options{
		Compact:           &export.CompactAPIJSONOptions{IncludeLibraryStubs: true, LibraryReferences: func([]export.APIJSONLibraryReference) { inner++ }},
		LibraryReferences: func([]export.APIJSONLibraryReference) { outer++ },
	})
	if err != nil {
		t.Fatal(err)
	}
	if inner != 1 || outer != 1 || !bytes.Contains(compact, []byte(`"isLibraryElement":true`)) {
		t.Fatal("conversion discarded compact options")
	}
	if _, err := export.ReadAPIJSON(compact); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{`[]`, `[{"@id":"literal-id","@type":"Class","qualifiedName":"Library::Class"}]`} {
		data := []byte(`{"format":"api-json-compact/1","ids":[],"elementCount":0,"referenceKeys":["@id","qualifiedName"],"elements":[],"libraryReferences":` + table + `}`)
		if _, err := export.ReadAPIJSON(data); err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range []string{`null`, `{}`, `[{"@id":0,"@type":"Class","qualifiedName":"C"}]`, `[{"@id":"C","@type":"Class"}]`, `[{"@id":"C","@id":"D","@type":"Class","qualifiedName":"C"}]`} {
		data := []byte(`{"format":"api-json-compact/1","ids":[],"elementCount":0,"referenceKeys":[],"elements":[],"libraryReferences":` + table + `}`)
		if _, err := export.ReadAPIJSON(data); err == nil {
			t.Fatalf("accepted invalid table %s", table)
		}
	}
	without := strings.Replace(string(compact), `,"libraryReferences":`, `,"unknownTable":`, 1)
	if _, err := export.ReadAPIJSON([]byte(without)); err == nil {
		t.Fatal("accepted unknown top-level key")
	}
}

func TestAPIJSONCompactLibraryMetadataPreservesLiteralIDsAndNames(t *testing.T) {
	data := []byte(`{"format":"api-json-compact/1","ids":["root","0"],"elementCount":1,"referenceKeys":["superclassifier","qualifiedName"],"elements":[{"@type":"Class","superclassifier":[1]}],"libraryReferences":[{"@id":"0","@type":"Class","qualifiedName":"0"}]}`)
	graph, err := export.ReadAPIJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	var got []export.APIJSONLibraryReference
	array, err := export.WriteAPIJSONWithOptions(graph, export.APIJSONOptions{LibraryReferences: func(refs []export.APIJSONLibraryReference) { got = refs }})
	if err != nil {
		t.Fatal(err)
	}
	want := []export.APIJSONLibraryReference{{ID: "0", Type: "Class", QualifiedName: "0"}}
	if !reflect.DeepEqual(got, want) || bytes.Contains(array, []byte(`"isLibraryElement": true`)) {
		t.Fatalf("metadata changed identity or became roots: %#v", got)
	}
}

func TestAPIJSONCompactLibraryMetadataRejectsElementConflicts(t *testing.T) {
	for _, element := range []string{
		`{"@type":"Class"}`,
		`{"@type":"Feature","isLibraryElement":true,"qualifiedName":"L"}`,
		`{"@type":"Class","isLibraryElement":true,"qualifiedName":"Other"}`,
	} {
		data := []byte(`{"format":"api-json-compact/1","ids":["L"],"elementCount":1,"referenceKeys":[],"elements":[` + element + `],"libraryReferences":[{"@id":"L","@type":"Class","qualifiedName":"L"}]}`)
		if _, err := export.ReadAPIJSON(data); err == nil {
			t.Fatalf("metadata redefined an element: %s", element)
		}
	}
}

func TestAPIJSONLibraryReferencesAcrossScopes(t *testing.T) {
	const libraryID = "14c0aa22-5489-59b5-b438-ded26e83ba31"
	graph := rdf.NewGraph()
	root, library := rdf.ElementIRIForID("root"), rdf.ElementIRIForID(libraryID)
	graph.Add(root, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("Class"))
	graph.Add(root, rdf.SysMLTerm("declaredName"), rdf.String("Root"))
	graph.Add(root, rdf.SysMLTerm("superclassifier"), library)
	graph.Add(library, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("DataType"))
	graph.Add(library, rdf.SysMLTerm("qualifiedName"), rdf.String("ScalarValues::Real"))
	graph.Add(library, rdf.SysMLTerm("isLibraryElement"), rdf.Bool(true))
	for _, scopeLibraries := range []bool{false, true} {
		scoped := rdf.NewGraph()
		rescope := func(term rdf.Term) rdf.Term {
			id, ok := rdf.SubjectID(term)
			if !ok || (!scopeLibraries && export.LibraryReference(graph, term)) {
				return term
			}
			return rdf.ScopedElementIRIForID("project", id)
		}
		for _, triple := range graph.Triples() {
			scoped.Add(rescope(triple.Subject), triple.Predicate, rescope(triple.Object))
		}
		for _, compact := range []bool{false, true} {
			var got []export.APIJSONLibraryReference
			opts := convert.Options{LibraryReferences: func(refs []export.APIJSONLibraryReference) { got = refs }}
			if compact {
				opts.Compact = &export.CompactAPIJSONOptions{}
			}
			data, err := convert.FromGraphWith(scoped, convert.FormatAPIJSON, opts)
			if err != nil {
				t.Fatal(err)
			}
			want := []export.APIJSONLibraryReference{{ID: libraryID, Type: "DataType", QualifiedName: "ScalarValues::Real"}}
			if scopeLibraries {
				for i := range want {
					want[i].ID = "project:" + want[i].ID
				}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("compact=%v scoped libraries=%v table=%v want=%v", compact, scopeLibraries, got, want)
			}
			if compact {
				read, err := export.ReadAPIJSON(data)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := convert.FromGraphWith(read, convert.FormatAPIJSON, opts); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("compact read/write changed scoped metadata: got %v want %v", got, want)
				}
			}
		}
	}
}

func TestAPIJSONUnnamedLibraryMembership(t *testing.T) {
	graph, err := convert.SysMLToRDF("import.sysml", []byte(`package P { private import ScalarValues::Real; }`))
	if err != nil {
		t.Fatal(err)
	}
	var refs []export.APIJSONLibraryReference
	compact, err := export.WriteAPIJSONCompact(graph, export.CompactAPIJSONOptions{LibraryReferences: func(got []export.APIJSONLibraryReference) { refs = got }})
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].Type != "OwningMembership" || refs[0].QualifiedName != "" {
		t.Fatalf("unnamed membership=%v", refs)
	}
	if _, err := export.ReadAPIJSON(compact); err != nil {
		t.Fatal(err)
	}
	legacy, err := export.WriteAPIJSONWithOptions(graph, export.APIJSONOptions{IncludeLibraryStubs: true})
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(legacy, &elements); err != nil {
		t.Fatal(err)
	}
	for _, element := range elements {
		if element["@id"] == refs[0].ID {
			if element["memberElement"] == nil {
				t.Fatal("legacy owning membership lost its memberElement")
			}
			if _, exists := element["qualifiedName"]; exists {
				t.Fatal("invented a name for an unnamed membership")
			}
		}
	}
}
