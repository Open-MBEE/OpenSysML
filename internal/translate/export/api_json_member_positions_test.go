package export

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

func TestAPIJSONMappingNamesCountDirectOwnedDeclarations(t *testing.T) {
	graph := rdf.NewGraph()
	pkg := rdf.ElementIRIForID("pkg")
	dependency := rdf.ElementIRIForID("dependency")
	annotation := rdf.ElementIRIForID("annotation")
	metadata := rdf.ElementIRIForID("metadata")
	nestedDependency := rdf.ElementIRIForID("nested-dependency")
	membership := rdf.ElementIRIForID("membership")
	member := rdf.ElementIRIForID("member")

	graph.Add(pkg, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("Package"))
	graph.Add(pkg, rdf.SysMLTerm("declaredName"), rdf.String("P"))
	graph.Add(pkg, rdf.SysMLTerm("qualifiedName"), rdf.String("P"))
	graph.Add(pkg, rdf.SysMLTerm("ownedRelationship"), dependency)
	graph.Add(pkg, rdf.SysMLTerm("ownedRelationship"), membership)
	graph.Add(pkg, rdf.SysMLTerm("ownedMembership"), membership)

	graph.Add(dependency, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("Dependency"))
	graph.Add(dependency, rdf.SysMLTerm("qualifiedName"), rdf.String("P::@0"))
	graph.Add(dependency, rdf.SysMLTerm("owner"), pkg)
	graph.Add(dependency, rdf.SysMLTerm("owningNamespace"), pkg)
	graph.Add(dependency, rdf.SysMLTerm("owningRelatedElement"), pkg)
	graph.Add(dependency, rdf.SysMLTerm("ownedRelationship"), annotation)
	graph.Add(dependency, rdf.SysMLTerm("ownedRelationship"), nestedDependency)

	graph.Add(annotation, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("Annotation"))
	graph.Add(annotation, rdf.SysMLTerm("owner"), dependency)
	graph.Add(annotation, rdf.SysMLTerm("owningRelatedElement"), dependency)
	graph.Add(annotation, rdf.SysMLTerm("ownedRelatedElement"), metadata)
	graph.Add(annotation, rdf.SysMLTerm("annotatingElement"), metadata)
	graph.Add(annotation, rdf.SysMLTerm("annotatedElement"), dependency)

	graph.Add(metadata, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("MetadataUsage"))
	graph.Add(metadata, rdf.SysMLTerm("declaredKeyword"), rdf.String("@"))
	graph.Add(metadata, rdf.SysMLTerm("qualifiedName"), rdf.String("P::@0::@0"))
	graph.Add(metadata, rdf.SysMLTerm("owner"), dependency)
	graph.Add(metadata, rdf.SysMLTerm("owningRelationship"), annotation)

	graph.Add(nestedDependency, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("Dependency"))
	graph.Add(nestedDependency, rdf.SysMLTerm("qualifiedName"), rdf.String("P::@0::@1"))
	graph.Add(nestedDependency, rdf.SysMLTerm("owner"), dependency)
	graph.Add(nestedDependency, rdf.SysMLTerm("owningRelatedElement"), dependency)

	graph.Add(membership, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("FeatureMembership"))
	graph.Add(membership, rdf.SysMLTerm("membershipOwningNamespace"), pkg)
	graph.Add(membership, rdf.SysMLTerm("owningRelatedElement"), pkg)
	graph.Add(membership, rdf.SysMLTerm("memberElement"), member)
	graph.Add(membership, rdf.SysMLTerm("ownedMemberFeature"), member)
	graph.Add(membership, rdf.SysMLTerm("ownedRelatedElement"), member)

	graph.Add(member, rdf.IRI(rdf.RDFType), rdf.SysMLTerm("AttributeUsage"))
	graph.Add(member, rdf.SysMLTerm("qualifiedName"), rdf.String("P::@1"))
	graph.Add(member, rdf.SysMLTerm("owningMembership"), membership)
	graph.Add(member, rdf.SysMLTerm("owningRelationship"), membership)
	graph.Add(member, rdf.SysMLTerm("owningNamespace"), pkg)

	document, err := WriteAPIJSON(graph)
	if err != nil {
		t.Fatalf("WriteAPIJSON: %v", err)
	}

	for _, test := range []struct {
		name      string
		stripSysx bool
	}{
		{name: "with_sysx"},
		{name: "without_sysx", stripSysx: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := document
			if test.stripSysx {
				var elements []map[string]any
				if err := json.Unmarshal(document, &elements); err != nil {
					t.Fatalf("decode API JSON: %v", err)
				}
				// The marker stays: without it the document is an external
				// writer's element form and reads through that path.
				for _, element := range elements {
					for key := range element {
						if strings.HasPrefix(key, "sysx:") && key != apiJSONStatedDefaultsKey {
							delete(element, key)
						}
					}
				}
				withoutSysx, err := json.Marshal(elements)
				if err != nil {
					t.Fatalf("encode API JSON without sysx properties: %v", err)
				}
				input = withoutSysx
			}

			decoded, err := ReadAPIJSON(input)
			if err != nil {
				t.Fatalf("ReadAPIJSON: %v", err)
			}
			for id, want := range map[string]string{
				"dependency":        "P::@0",
				"metadata":          "P::@0::@0",
				"nested-dependency": "P::@0::@1",
				"member":            "P::@1",
			} {
				subject := rdf.ElementIRIForID(id)
				got, ok := addressOf(decoded, subject)
				if !ok || got != want {
					t.Errorf("%s qualifiedName = %q, present=%v; want %q", id, got, ok, want)
				}
			}
		})
	}
}
