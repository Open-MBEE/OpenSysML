package export

import (
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
)

// APIJSONLibraryReference names a standard-library element without declaring a root.
type APIJSONLibraryReference struct {
	ID            string `json:"@id"`
	Type          string `json:"@type"`
	QualifiedName string `json:"qualifiedName"`
}

func apiJSONLibraryStub(graph *rdf.Graph, subject rdf.Term) apiJSONObject {
	id, _ := rdf.SubjectID(subject)
	return apiJSONObject{
		{key: "@type", value: apiJSONType(graph.ObjectsView(subject, rdf.RDFType)[0])},
		{key: "@id", value: id},
		{key: "declaredName", value: apiJSONLibraryName(graph, subject, "declaredName")},
		{key: "elementId", value: id},
		{key: "isLibraryElement", value: true},
		{key: "qualifiedName", value: apiJSONLibraryName(graph, subject, "qualifiedName")},
	}
}

func apiJSONLibraryReferences(graph *rdf.Graph, elements []apiJSONObject) []APIJSONLibraryReference {
	referenced := map[string]bool{}
	for _, element := range elements {
		for _, member := range element {
			walkCompactValue(member.value, func(value any) {
				if ref, ok := value.(apiJSONReference); ok {
					referenced[ref.ID] = true
				}
			})
		}
	}
	out := make([]APIJSONLibraryReference, 0)
	for _, subject := range graph.SubjectsView() {
		id, ok := rdf.SubjectID(subject)
		if !ok || !referenced[id] || !LibraryReference(graph, subject) {
			continue
		}
		out = append(out, APIJSONLibraryReference{ID: id,
			Type:          strings.TrimPrefix(graph.Type(subject), rdf.SysML),
			QualifiedName: apiJSONLibraryName(graph, subject, "qualifiedName"),
		})
	}
	slices.SortFunc(out, func(a, b APIJSONLibraryReference) int { return strings.Compare(a.ID, b.ID) })
	return out
}

func apiJSONLibraryName(graph *rdf.Graph, subject rdf.Term, property string) string {
	for _, value := range graph.ObjectsView(subject, rdf.SysML+property) {
		if value.IsLiteral() {
			return value.Value
		}
	}
	return ""
}
