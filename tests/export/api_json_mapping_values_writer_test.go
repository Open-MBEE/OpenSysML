package export_test

import (
	"encoding/json"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func TestAPIJSONWriterPreservesNoncanonicalNumericLexicalValue(t *testing.T) {
	subject := rdf.IRI(rdf.Expression + "literal-rational")
	graph := rdf.NewGraphOf([]rdf.Triple{
		{Subject: subject, Predicate: rdf.IRI(rdf.RDFType), Object: rdf.SysMLTerm("LiteralRational")},
		{Subject: subject, Predicate: rdf.SysMLTerm("value"), Object: rdf.TypedLiteral(".1", rdf.XSD+"decimal")},
	}, nil)
	document, err := export.WriteAPIJSON(graph)
	if err != nil {
		t.Fatal(err)
	}
	var elements []struct {
		ID           string              `json:"@id"`
		Value        json.Number         `json:"value"`
		MappingValue map[string][]string `json:"sysx:mappingValues"`
	}
	if err := json.Unmarshal(document, &elements); err != nil {
		t.Fatal(err)
	}
	for _, element := range elements {
		if element.ID != "literal-rational" {
			continue
		}
		if element.Value != json.Number("0.1") {
			t.Fatalf("value = %s, want 0.1", element.Value)
		}
		if got := element.MappingValue["value"]; len(got) != 1 || got[0] != ".1" {
			t.Fatalf("mappingValues.value = %v, want [\".1\"]", got)
		}
		restored, err := export.ReadAPIJSON(document)
		if err != nil {
			t.Fatal(err)
		}
		if !restored.Has(rdf.Triple{
			Subject: subject, Predicate: rdf.SysMLTerm("value"),
			Object: rdf.TypedLiteral(".1", rdf.XSD+"decimal"),
		}) {
			t.Fatal("mappingValues did not restore the exact numeric literal")
		}
		return
	}
	t.Fatal("API JSON omitted the LiteralRational element")
}
