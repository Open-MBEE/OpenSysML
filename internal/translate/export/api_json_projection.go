package export

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// APIJSONProjection names the api-json elements a conversion writes.
// Its selectors are unioned; all empty writes the whole model.
type APIJSONProjection struct {
	// QualifiedNames selects elements by their written qualifiedName.
	QualifiedNames []string
	// Metaclasses selects elements by their written @type.
	Metaclasses []string
	// AnnotationTypes selects named annotations and what they annotate.
	AnnotationTypes []string
}

// Empty reports that the projection places no limit on what is written.
func (p *APIJSONProjection) Empty() bool {
	return p == nil || (len(p.QualifiedNames) == 0 && len(p.Metaclasses) == 0 && len(p.AnnotationTypes) == 0)
}

// APIJSONProjectionError reports a projection name no element carries,
// so a misspelled name fails rather than quietly returning nothing.
type APIJSONProjectionError struct {
	// Selector is qualified_names, metaclasses or annotation_types.
	Selector string
	// Name is the unmatched name.
	Name string
}

func (e *APIJSONProjectionError) Error() string {
	return fmt.Sprintf("projection %s names %q, which the model does not carry", e.Selector, e.Name)
}

// apiJSONProjectionIndex is the set of subjects a projection writes,
// built over the same settled graph the writer encodes.
type apiJSONProjectionIndex struct {
	selected map[rdf.Term]bool
}

// Matches reports whether subject is in the projection.
func (i *apiJSONProjectionIndex) Matches(subject rdf.Term) bool {
	return i.selected[subject]
}

// projectionNames indexes names to the subjects carrying them.
// A subject is recorded once per name, independent of statement order.
type projectionNames struct {
	byName map[string][]rdf.Term
	seen   map[string]map[string]bool
}

func newProjectionNames() *projectionNames {
	return &projectionNames{byName: map[string][]rdf.Term{}, seen: map[string]map[string]bool{}}
}

func (n *projectionNames) add(name string, subject rdf.Term) {
	if name == "" {
		return
	}
	marked := n.seen[name]
	if marked == nil {
		marked = map[string]bool{}
		n.seen[name] = marked
	}
	if marked[subject.Value] {
		return
	}
	marked[subject.Value] = true
	n.byName[name] = append(n.byName[name], subject)
}

func newAPIJSONProjectionIndex(graph *rdf.Graph, writer *apiJSONElementWriter, p *APIJSONProjection) (*apiJSONProjectionIndex, error) {
	names := newProjectionNames()
	metaclasses := newProjectionNames()
	annotationTypes := newProjectionNames()
	for _, subject := range graph.SubjectsView() {
		if name, err := writer.writtenQualifiedName(subject); err != nil {
			return nil, err
		} else {
			names.add(name, subject)
		}
		metaclasses.add(apiJSONProjectionMetaclass(graph, subject), subject)
	}
	for _, subject := range graph.SubjectsView() {
		if apiJSONProjectionMetaclass(graph, subject) != "Annotation" {
			continue
		}
		annotating := graph.ObjectsView(subject, rdf.SysML+pAnnotatingElement)
		annotated := graph.ObjectsView(subject, rdf.SysML+pAnnotatedElement)
		for _, element := range annotating {
			for _, name := range apiJSONAnnotationTypeNames(graph, element) {
				// The annotation and its ends are written with the declaration.
				annotationTypes.add(name, subject)
				for _, end := range annotating {
					annotationTypes.add(name, end)
				}
				for _, end := range annotated {
					annotationTypes.add(name, end)
				}
			}
		}
	}

	selected := map[rdf.Term]bool{}
	for _, name := range p.QualifiedNames {
		found := names.byName[name]
		if len(found) == 0 {
			return nil, &APIJSONProjectionError{Selector: "qualified_names", Name: name}
		}
		for _, subject := range found {
			selected[subject] = true
		}
	}
	for _, name := range p.Metaclasses {
		found := metaclasses.byName[name]
		if len(found) == 0 {
			// A declared metaclass is known even when this model has none of it.
			if _, declared := ontology.LookupClass(name); !declared {
				return nil, &APIJSONProjectionError{Selector: "metaclasses", Name: name}
			}
			continue
		}
		for _, subject := range found {
			selected[subject] = true
		}
	}
	for _, name := range p.AnnotationTypes {
		found := annotationTypes.byName[name]
		if len(found) == 0 {
			return nil, &APIJSONProjectionError{Selector: "annotation_types", Name: name}
		}
		for _, subject := range found {
			selected[subject] = true
		}
	}
	return &apiJSONProjectionIndex{selected: selected}, nil
}

// apiJSONProjectionMetaclass is the @type the writer puts on subject.
func apiJSONProjectionMetaclass(graph *rdf.Graph, subject rdf.Term) string {
	_, typeName, _ := apiJSONMetaclassNames(graph, subject, rdf.IRI(graph.Type(subject)))
	return typeName
}

// apiJSONAnnotationTypeNames names an annotating element by metaclass
// and by the qualified name of the definition that types it.
func apiJSONAnnotationTypeNames(graph *rdf.Graph, annotating rdf.Term) []string {
	names := []string{apiJSONProjectionMetaclass(graph, annotating)}
	for _, relationship := range graph.ObjectsView(annotating, rdf.SysML+pOwnedRelationship) {
		if apiJSONProjectionMetaclass(graph, relationship) != "FeatureTyping" {
			continue
		}
		for _, typed := range graph.ObjectsView(relationship, rdf.SysML+"type") {
			if name, ok := graph.Lexical(typed, rdf.SysML+pQualifiedName); ok {
				names = append(names, name)
				if i := strings.LastIndex(name, "::"); i >= 0 {
					names = append(names, name[i+2:])
				}
			}
		}
	}
	return names
}
