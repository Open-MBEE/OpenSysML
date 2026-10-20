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

func TestAPIJSONMappingPropertyOrderOnlyRecordsMisplacedSidecarProperties(t *testing.T) {
	subject := rdf.ElementIRIForID("p")
	reference := apiJSONReference{ID: "other"}
	for _, test := range []struct {
		name       string
		properties []string
		element    apiJSONObject
		want       bool
	}{
		{
			name:       "ordinary property order mismatch",
			properties: []string{"owner", "target"},
			element: apiJSONObject{
				{key: "@type", value: "PartUsage"},
				{key: "@id", value: "p"},
				{key: "target", value: reference},
				{key: "owner", value: reference},
			},
			want: false,
		},
		{
			name:       "sidecar-only property is misplaced",
			properties: []string{"target", "relatedElement", "owner"},
			element: apiJSONObject{
				{key: "@type", value: "PartUsage"},
				{key: "@id", value: "p"},
				{key: "target", value: reference},
				{key: "owner", value: reference},
				{key: "sysx:mappingValues", value: apiJSONObject{
					{key: "relatedElement", value: []any{reference}},
				}},
			},
			want: true,
		},
		{
			name:       "sidecar-only property is naturally placed",
			properties: []string{"target", "relatedElement", "owner"},
			element: apiJSONObject{
				{key: "@type", value: "PartUsage"},
				{key: "@id", value: "p"},
				{key: "target", value: reference},
				{key: "sysx:mappingValues", value: apiJSONObject{
					{key: "relatedElement", value: []any{reference}},
				}},
				{key: "owner", value: reference},
			},
			want: false,
		},
		{
			name:       "ordinary order mismatch does not misplace sidecar property",
			properties: []string{"owner", "target", "relatedElement"},
			element: apiJSONObject{
				{key: "@type", value: "PartUsage"},
				{key: "@id", value: "p"},
				{key: "target", value: reference},
				{key: "owner", value: reference},
				{key: "sysx:mappingValues", value: apiJSONObject{
					{key: "relatedElement", value: []any{reference}},
				}},
			},
			want: false,
		},
		{
			name:       "qualified name in element order is misplaced",
			properties: []string{"isComposite", "qualifiedName"},
			element: apiJSONObject{
				{key: "@type", value: "PartUsage"},
				{key: "@id", value: "p"},
				{key: "qualifiedName", value: "P::p"},
				{key: "isComposite", value: true},
			},
			want: true,
		},
		{
			name:       "qualified name in graph order needs no sidecar",
			properties: []string{"qualifiedName", "isComposite"},
			element: apiJSONObject{
				{key: "@type", value: "PartUsage"},
				{key: "@id", value: "p"},
				{key: "qualifiedName", value: "P::p"},
				{key: "isComposite", value: true},
			},
			want: false,
		},
		{
			name:       "reconstructed qualified name placement is preserved",
			properties: []string{"isComposite", "qualifiedName"},
			element: apiJSONObject{
				{key: "@type", value: "PartUsage"},
				{key: "@id", value: "p"},
				{key: "isComposite", value: true},
			},
			want: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			graph := rdf.NewGraph()
			graph.AddTriple(rdf.Triple{
				Subject:   subject,
				Predicate: rdf.IRI(rdf.RDFType),
				Object:    rdf.SysMLTerm("PartUsage"),
			})
			for _, property := range test.properties {
				graph.AddTriple(rdf.Triple{
					Subject:   subject,
					Predicate: rdf.SysMLTerm(property),
					Object:    rdf.String(property),
				})
			}
			graphOrder := apiJSONGraphPropertyOrder(graph, subject)
			naturalOrder, sidecarOnly := apiJSONNaturalPropertyOrder(graph, subject, test.element)
			if got := apiJSONHasMisplacedSidecarProperty(graphOrder, naturalOrder, sidecarOnly); got != test.want {
				t.Fatalf("needs mapping property order = %v, want %v (graph %v, natural %v, sidecar-only %v)",
					got, test.want, graphOrder, naturalOrder, sidecarOnly)
			}
		})
	}
}
