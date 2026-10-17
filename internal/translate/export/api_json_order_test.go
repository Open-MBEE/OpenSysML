package export

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func TestAPIJSONMappingPropertyOrderReordersElementProperties(t *testing.T) {
	data := []byte(`[
		{
			"@type": "PartUsage",
			"@id": "p",
			"owner": {"@id": "o"},
			"target": {"@id": "t"},
			"declaredName": "p",
			"sysx:mappingPropertyOrder": ["target", "owner"]
		}
	]`)
	graph, err := ReadAPIJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	subject := rdf.ElementIRIForID("p")
	want := []string{
		rdf.RDFType,
		rdf.SysML + "target",
		rdf.SysML + "owner",
		rdf.SysML + "declaredName",
	}
	if got := graph.PredicatesView(subject); !slices.Equal(got, want) {
		t.Fatalf("property order = %v, want %v", got, want)
	}
	if graph.HasProperty(subject, rdf.OpenSysML+"mappingPropertyOrder") {
		t.Fatal("sysx:mappingPropertyOrder was emitted as an RDF property")
	}
}

func TestAPIJSONWithoutMappingPropertyOrderUsesNaturalOrder(t *testing.T) {
	data := []byte(`[
		{
			"@type": "PartUsage",
			"@id": "p",
			"owner": {"@id": "o"},
			"target": {"@id": "t"},
			"declaredName": "p"
		}
	]`)
	graph, err := ReadAPIJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	subject := rdf.ElementIRIForID("p")
	want := []string{
		rdf.RDFType,
		rdf.SysML + "owner",
		rdf.SysML + "target",
		rdf.SysML + "declaredName",
	}
	if got := graph.PredicatesView(subject); !slices.Equal(got, want) {
		t.Fatalf("property order = %v, want %v", got, want)
	}
}
