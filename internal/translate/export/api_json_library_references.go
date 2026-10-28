package export

import (
	"encoding/json"
	"fmt"
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

func apiJSONReferencedLibraryIDs(elements []apiJSONObject) map[string]bool {
	referenced := map[string]bool{}
	for _, element := range elements {
		subject := rdf.ReferenceIRI(rdf.Term{}, element[1].value.(string))
		for _, member := range element {
			walkCompactValue(member.value, func(value any) {
				if ref, ok := value.(apiJSONReference); ok {
					id, _ := rdf.SubjectID(rdf.ReferenceIRI(subject, ref.ID))
					referenced[id] = true
				}
			})
		}
	}
	return referenced
}

func apiJSONLibraryReferences(graph *rdf.Graph, elements []apiJSONObject) ([]APIJSONLibraryReference, error) {
	referenced := apiJSONReferencedLibraryIDs(elements)
	out := make([]APIJSONLibraryReference, 0)
	for _, subject := range graph.SubjectsView() {
		id, ok := rdf.SubjectID(subject)
		if !ok || !referenced[id] || !LibraryReference(graph, subject) {
			continue
		}
		types := graph.ObjectsView(subject, rdf.RDFType)
		if len(types) != 1 || !types[0].IsIRI() || (!strings.HasPrefix(types[0].Value, rdf.SysML) && !strings.HasPrefix(types[0].Value, rdf.OpenSysML)) {
			return nil, &UnsupportedError{What: fmt.Sprintf("library subject <%s> must have exactly one metamodel rdf:type", subject.Value)}
		}
		out = append(out, APIJSONLibraryReference{ID: id,
			Type:          apiJSONType(types[0]),
			QualifiedName: apiJSONLibraryName(graph, subject, "qualifiedName"),
		})
	}
	slices.SortFunc(out, func(a, b APIJSONLibraryReference) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}

func apiJSONLibraryName(graph *rdf.Graph, subject rdf.Term, property string) string {
	for _, value := range graph.ObjectsView(subject, rdf.SysML+property) {
		if value.IsLiteral() {
			return value.Value
		}
	}
	return ""
}

// The table is supplemental metadata; expansion never makes its entries roots.
func readAPIJSONLibraryReferences(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil || token != json.Delim('[') {
		return fmt.Errorf("compact API libraryReferences must be an array")
	}
	ids := map[string]bool{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil || token != json.Delim('{') {
			return fmt.Errorf("compact API libraryReferences entries must be objects")
		}
		entry, err := apiJSONCompactObjectOf(decoder)
		if err != nil {
			return err
		}
		if len(entry) != 3 {
			return fmt.Errorf("compact API libraryReferences entries require @id, @type and qualifiedName")
		}
		id := ""
		for _, member := range entry {
			if member.key != "@id" && member.key != "@type" && member.key != "qualifiedName" {
				return fmt.Errorf("compact API libraryReferences has unknown key %q", member.key)
			}
			value, ok := member.value.(string)
			if !ok || (value == "" && member.key != "qualifiedName") {
				return fmt.Errorf("compact API libraryReferences %s must be a literal string (nonempty except qualifiedName)", member.key)
			}
			if member.key == "@id" {
				id = value
			}
		}
		if ids[id] {
			return fmt.Errorf("compact API libraryReferences repeats id %q", id)
		}
		ids[id] = true
	}
	_, err = decoder.Token()
	return err
}
