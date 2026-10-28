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
func readAPIJSONLibraryReferences(decoder *json.Decoder) ([]APIJSONLibraryReference, error) {
	token, err := decoder.Token()
	if err != nil || token != json.Delim('[') {
		return nil, fmt.Errorf("compact API libraryReferences must be an array")
	}
	ids := map[string]bool{}
	var references []APIJSONLibraryReference
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil || token != json.Delim('{') {
			return nil, fmt.Errorf("compact API libraryReferences entries must be objects")
		}
		entry, err := apiJSONCompactObjectOf(decoder)
		if err != nil {
			return nil, err
		}
		if len(entry) != 3 {
			return nil, fmt.Errorf("compact API libraryReferences entries require @id, @type and qualifiedName")
		}
		var reference APIJSONLibraryReference
		for _, member := range entry {
			if member.key != "@id" && member.key != "@type" && member.key != "qualifiedName" {
				return nil, fmt.Errorf("compact API libraryReferences has unknown key %q", member.key)
			}
			value, ok := member.value.(string)
			if !ok || (value == "" && member.key != "qualifiedName") {
				return nil, fmt.Errorf("compact API libraryReferences %s must be a literal string (nonempty except qualifiedName)", member.key)
			}
			switch member.key {
			case "@id":
				reference.ID = value
			case "@type":
				reference.Type = value
			case "qualifiedName":
				reference.QualifiedName = value
			}
		}
		if ids[reference.ID] {
			return nil, fmt.Errorf("compact API libraryReferences repeats id %q", reference.ID)
		}
		ids[reference.ID] = true
		references = append(references, reference)
	}
	_, err = decoder.Token()
	return references, err
}

func addAPIJSONLibraryReferences(graph *rdf.Graph, references []APIJSONLibraryReference) error {
	for _, reference := range references {
		subject := rdf.ReferenceIRI(rdf.Term{}, reference.ID)
		typ, err := apiJSONTypeIRI(reference.Type)
		if err != nil {
			return err
		}
		if types := graph.ObjectsView(subject, rdf.RDFType); len(types) > 0 {
			if !LibraryReference(graph, subject) || len(types) != 1 || types[0] != typ {
				return fmt.Errorf("compact API libraryReferences conflicts with element %q", reference.ID)
			}
			if graph.HasProperty(subject, rdf.SysML+"qualifiedName") &&
				apiJSONLibraryName(graph, subject, "qualifiedName") != reference.QualifiedName {
				return fmt.Errorf("compact API libraryReferences conflicts with qualifiedName of %q", reference.ID)
			}
		} else {
			graph.Add(subject, rdf.IRI(rdf.RDFType), typ)
			graph.Add(subject, rdf.SysMLTerm("isLibraryElement"), rdf.Bool(true))
		}
		if !graph.HasProperty(subject, rdf.SysML+"qualifiedName") {
			graph.Add(subject, rdf.SysMLTerm("qualifiedName"), rdf.String(reference.QualifiedName))
		}
	}
	return nil
}
