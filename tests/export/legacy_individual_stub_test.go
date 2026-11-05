package export_test

import (
	"encoding/json"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// TestLegacyIndividualLibraryStubKeepsItsFlag covers a library reference typed
// sysml:IndividualUsage, the shape earlier releases wrote: written as a stub it is an
// OccurrenceUsage stating isIndividual, and read back it states the flag.
func TestLegacyIndividualLibraryStubKeepsItsFlag(t *testing.T) {
	turtle := `@prefix sysml: <https://www.omg.org/spec/SysML#> .
@prefix elmt: <urn:sysmlv2:element:> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

elmt:Lib__io
    a sysml:IndividualUsage ;
    sysml:elementId "Lib__io" ;
    sysml:declaredName "io" ;
    sysml:isLibraryElement "true"^^xsd:boolean .
`
	graph, err := rdf.ParseTurtle([]byte(turtle))
	if err != nil {
		t.Fatal(err)
	}
	subject := rdf.IRI("urn:sysmlv2:element:Lib__io")
	if !export.LibraryReference(graph, subject) {
		t.Fatal("the fixture's io is not a library reference")
	}
	apiJSON, err := export.WriteAPIJSONWithOptions(graph, export.APIJSONOptions{IncludeLibraryStubs: true})
	if err != nil {
		t.Fatal(err)
	}
	var elements []map[string]any
	if err := json.Unmarshal(apiJSON, &elements); err != nil {
		t.Fatal(err)
	}
	var stub map[string]any
	for _, element := range elements {
		payload, _ := element["payload"].(map[string]any)
		if payload == nil {
			payload = element
		}
		if payload["@id"] == "Lib__io" {
			stub = payload
		}
	}
	if stub == nil {
		t.Fatalf("no stub for io in\n%s", apiJSON)
	}
	if stub["@type"] != "OccurrenceUsage" || stub["isIndividual"] != true {
		t.Errorf("legacy stub = %v, want @type OccurrenceUsage with isIndividual true", stub)
	}
	back, err := export.ReadAPIJSON(apiJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !back.BoolValue(subject, "https://www.omg.org/spec/SysML#isIndividual") {
		t.Errorf("the stub read back states no isIndividual")
	}
}
