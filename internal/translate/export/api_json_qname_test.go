package export

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func TestAPIJSONQualifiedNameUsesEvaluator(t *testing.T) {
	file := source.New("duplicates.sysml", []byte(`package P {
    part def A;
    part def A;
    state def S;
    state running : S;
}`))
	graph, err := ToRDF(file, parser.New(file).ParseFile())
	if err != nil {
		t.Fatal(err)
	}
	defaults := mappingQualifiedNames(graphWithoutQualifiedNames(graph))
	standard, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(standard, &elements); err != nil {
		t.Fatal(err)
	}
	var duplicateNames []any
	for _, element := range elements {
		if element["declaredName"] == "A" {
			duplicateNames = append(duplicateNames, element["qualifiedName"])
		}
		if name, ok := element["qualifiedName"].(string); ok && strings.Contains(name, "::@") {
			t.Errorf("spec qualifiedName %q contains a positional mapping segment", name)
		}
	}
	if len(duplicateNames) != 2 || duplicateNames[0] != "P::A" || duplicateNames[1] != nil {
		t.Errorf("duplicate spec qualifiedNames = %#v, want [P::A null]", duplicateNames)
	}
	equal := 0
	for _, element := range elements {
		id, _ := element["@id"].(string)
		subject, found := subjectForID(graph, id)
		if !found {
			continue
		}
		graphName, hasGraphName := addressOf(graph, subject)
		mappingName, hasMappingName := element["sysx:mappingQualifiedName"].(string)
		defaultName, hasDefault := defaults[id]
		if hasGraphName && hasDefault && graphName == defaultName {
			equal++
			if hasMappingName {
				t.Errorf("%s redundantly carries mappingQualifiedName %q", id, mappingName)
			}
			continue
		}
		if hasGraphName {
			if !hasMappingName || mappingName != graphName {
				t.Errorf("%s mappingQualifiedName = %q, want %q", id, mappingName, graphName)
			}
		}
	}
	if equal == 0 {
		t.Fatalf("default names = %d; want at least one reconstructible name", equal)
	}

	compact, err := WriteAPIJSONCompact(graph, CompactAPIJSONOptions{OmitDerived: true})
	if err != nil {
		t.Fatal(err)
	}
	var compactDoc struct {
		IDs      []string         `json:"ids"`
		Elements []map[string]any `json:"elements"`
	}
	if err := json.Unmarshal(compact, &compactDoc); err != nil {
		t.Fatal(err)
	}
	for i, element := range compactDoc.Elements {
		subject, found := subjectForID(graph, compactDoc.IDs[i])
		if !found {
			continue
		}
		graphName, hasGraphName := addressOf(graph, subject)
		mappingName, hasMappingName := element["sysx:mappingQualifiedName"].(string)
		defaultName, hasDefault := defaults[compactDoc.IDs[i]]
		if hasGraphName && hasDefault && graphName == defaultName {
			if hasMappingName {
				t.Errorf("compact element %s redundantly carries mappingQualifiedName", compactDoc.IDs[i])
			}
		} else if hasGraphName && (!hasMappingName || mappingName != graphName) {
			t.Errorf("compact element %s mappingQualifiedName = %q, want %q", compactDoc.IDs[i], mappingName, graphName)
		}
	}

	partial := rdf.NewGraph()
	subject := rdf.ElementIRIForID("partial")
	partial.Add(subject, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("PartUsage"))
	partial.Add(subject, rdf.SysMLTerm("declaredName"), rdf.String("x"))
	partial.Add(subject, rdf.SysMLTerm("owningRelationship"), rdf.ElementIRIForID("missing"))
	partialElements, err := apiJSONElements(partial)
	if err != nil {
		t.Fatal(err)
	}
	for _, element := range partialElements {
		if element[1].value == "partial" && hasAPIJSONMember(element, "qualifiedName") {
			t.Fatalf("partial element has an undeterminable qualifiedName: %#v", element)
		}
	}
	evaluation := newAPIJSONEvaluation(partial)
	handle, ok := evaluation.structure.element(subject)
	if !ok {
		t.Fatal("partial graph subject has no evaluator handle")
	}
	if _, ok := evaluation.evaluator.Property(handle, "Element", "qualifiedName"); ok {
		t.Fatal("qualifiedName unexpectedly evaluated from incomplete structure")
	}
	reason, ok := evaluation.evaluator.Explain(handle, "Element", "qualifiedName")
	if !ok || reason.Category != metamodel.ReasonMissingStructure {
		t.Fatalf("qualifiedName reason = %#v, want missing structure", reason)
	}
}

func TestAPIJSONSpecQualifiedNameUsesMappingSpelling(t *testing.T) {
	for spec, want := range map[string]string{
		"'Fuel Pump'":       "Fuel Pump",
		"@2":                "'@2'",
		"@2::'Fuel Pump'":   "'@2'::Fuel Pump",
		"'Fuel Pump'::'@2'": "Fuel Pump::'@2'",
	} {
		if got := mappingQualifiedNameFromSpec(spec); got != want {
			t.Errorf("mapping spelling for %q = %q, want %q", spec, got, want)
		}
	}
}

func TestReadAPIJSONMapsSpecQualifiedNameSpelling(t *testing.T) {
	documents := map[string]string{
		"standard": `[
			{"@type":"PartDefinition","@id":"pump","declaredName":"Fuel Pump","qualifiedName":"'Fuel Pump'"},
			{"@type":"Package","@id":"at","declaredName":"@2","qualifiedName":"'@2'"}
		]`,
		"toolkit": `[
			{"@type":"PartDefinition","@id":"pump","declaredName":"Fuel Pump","qualifiedName":"'Fuel Pump'","isImpliedIncluded":false},
			{"@type":"Package","@id":"at","declaredName":"@2","qualifiedName":"'@2'","isImpliedIncluded":false}
		]`,
	}
	for name, document := range documents {
		t.Run(name, func(t *testing.T) {
			graph, err := ReadAPIJSON([]byte(document))
			if err != nil {
				t.Fatal(err)
			}
			for id, want := range map[string]string{
				"pump": "Fuel Pump",
				"at":   "'@2'",
			} {
				subject := rdf.ReferenceIRI(rdf.Term{}, id)
				got, ok := addressOf(graph, subject)
				if !ok || got != want {
					t.Errorf("%s qualifiedName = %q, want %q", id, got, want)
				}
			}
		})
	}
}

func TestAPIJSONOmitsMappingQualifiedNameWhenDefaultMatches(t *testing.T) {
	file := source.New("mapping-name.sysml", []byte(`package P {
    part def A;
    part a : A;
}`))
	graph, err := ToRDF(file, parser.New(file).ParseFile())
	if err != nil {
		t.Fatal(err)
	}
	defaults := mappingQualifiedNames(graphWithoutQualifiedNames(graph))
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	matched := 0
	for _, element := range elements {
		id, _ := element["@id"].(string)
		subject, ok := subjectForID(graph, id)
		if !ok {
			continue
		}
		graphName, hasGraphName := addressOf(graph, subject)
		defaultName, hasDefault := defaults[id]
		if hasGraphName && hasDefault && graphName == defaultName {
			matched++
			if _, hasKey := element["sysx:mappingQualifiedName"]; hasKey {
				t.Errorf("%s carries a redundant mappingQualifiedName", id)
			}
		}
	}
	if matched == 0 {
		t.Fatal("no element had a reconstructible mapping name")
	}
}

func TestAPIJSONOmitsMappingQualifiedNameWhenLibraryNameMatchesSpec(t *testing.T) {
	file := source.New("library-name.sysml", []byte(`package P {
    private import ScalarValues::Integer;
    part def V {
        attribute count : ScalarValues::Integer;
    }
}`))
	graph, err := ToRDF(file, parser.New(file).ParseFile())
	if err != nil {
		t.Fatal(err)
	}
	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	libraryID := ""
	for _, subject := range graph.Subjects() {
		qualifiedName, ok := addressOf(graph, subject)
		if !LibraryReference(graph, subject) || !ok || qualifiedName != "ScalarValues::Integer" {
			continue
		}
		libraryID, _ = rdf.SubjectID(subject)
	}
	if libraryID == "" {
		t.Fatal("the RDF graph has no ScalarValues::Integer library reference")
	}
	libraryElement := false
	typeReference := false
	for _, element := range elements {
		if element["@id"] == libraryID {
			libraryElement = true
		}
		if element["declaredName"] != "count" {
			continue
		}
		switch value := element["type"].(type) {
		case map[string]any:
			typeReference = value["@id"] == libraryID
		case []any:
			for _, item := range value {
				reference, ok := item.(map[string]any)
				typeReference = typeReference || ok && reference["@id"] == libraryID
			}
		}
	}
	if libraryElement {
		t.Fatal("the API JSON emits the ScalarValues::Integer library element")
	}
	if !typeReference {
		t.Fatal("the count attribute does not reference ScalarValues::Integer by @id")
	}

	compact, err := WriteAPIJSONCompact(graph, CompactAPIJSONOptions{OmitDerived: true})
	if err != nil {
		t.Fatal(err)
	}
	var compactDocument struct {
		IDs      []string         `json:"ids"`
		Elements []map[string]any `json:"elements"`
	}
	if err := json.Unmarshal(compact, &compactDocument); err != nil {
		t.Fatal(err)
	}
	for i := range compactDocument.Elements {
		if compactDocument.IDs[i] == libraryID {
			t.Fatal("the compact API JSON emits the ScalarValues::Integer library element")
		}
	}
}

func subjectForID(graph *rdf.Graph, id string) (rdf.Term, bool) {
	for _, subject := range graph.Subjects() {
		if subjectID, ok := rdf.SubjectID(subject); ok && subjectID == id {
			return subject, true
		}
	}
	return rdf.Term{}, false
}
