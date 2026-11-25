package export

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/naming"
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
				// A binding body is held by owned members and their value
				// expressions, not by the annotating element itself.
				apiJSONAddAnnotationBinding(graph, annotationTypes, name, element)
			}
		}
	}
	for _, subject := range graph.SubjectsView() {
		if !apiJSONIsAnnotationMember(graph, subject) {
			continue
		}
		// An element that annotates through an Annotation is already selected
		// with that annotation; here only the members owned in a body remain.
		if len(graph.ObjectsView(subject, rdf.SysML+pOwnedAnnotatingRelationship)) > 0 {
			continue
		}
		for _, name := range apiJSONAnnotationTypeNames(graph, subject) {
			annotationTypes.add(name, subject)
			for _, owner := range graph.ObjectsView(subject, rdf.SysML+pOwner) {
				annotationTypes.add(name, owner)
			}
			apiJSONAddAnnotationBinding(graph, annotationTypes, name, subject)
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

// annotationMemberMetaclasses is the AnnotatingElement metaclass and every
// metaclass below it, computed once from the ontology's parent edges.
var annotationMemberMetaclasses = func() map[string]bool {
	out := map[string]bool{"AnnotatingElement": true}
	for changed := true; changed; {
		changed = false
		for _, class := range ontology.Classes() {
			if out[class.Name] {
				continue
			}
			for _, parent := range class.Parents {
				if out[parent] {
					out[class.Name] = true
					changed = true
					break
				}
			}
		}
	}
	return out
}()

// apiJSONIsAnnotationMember reports whether subject is an annotating element,
// written as about usage or as a member owned in a body.
func apiJSONIsAnnotationMember(graph *rdf.Graph, subject rdf.Term) bool {
	metaclass, _, _ := apiJSONMetaclassNames(graph, subject, rdf.IRI(graph.Type(subject)))
	return annotationMemberMetaclasses[metaclass]
}

// annotationBindingPredicates are the containment edges followed to a binding.
// memberElement is absent: an alias states a reference on it, not ownership.
var annotationBindingPredicates = []string{
	pOwnedRelationship,
	pOwnedMembership,
	pOwnedMember,
	pOwnedFeature,
	pOwnedFeatureMembership,
	pOwnedMemberFeature,
	pOwnedMemberElement,
	pOwnedRelatedElement,
	pValue,
}

// apiJSONAddAnnotationBinding selects the owned members and value expressions
// needed to read an annotation's bindings from its usage.
func apiJSONAddAnnotationBinding(graph *rdf.Graph, names *projectionNames, name string, annotating rdf.Term) {
	seen := map[rdf.Term]bool{annotating: true}
	pending := []rdf.Term{annotating}
	for len(pending) > 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		for _, predicate := range annotationBindingPredicates {
			for _, object := range graph.ObjectsView(current, rdf.SysML+predicate) {
				if !object.IsIRI() || seen[object] {
					continue
				}
				seen[object] = true
				names.add(name, object)
				pending = append(pending, object)
			}
		}
	}
}

// apiJSONAnnotationTypeNames names an annotating element by metaclass and by
// its definitions' qualified names, last segments and declared short names.
func apiJSONAnnotationTypeNames(graph *rdf.Graph, annotating rdf.Term) []string {
	names := []string{apiJSONProjectionMetaclass(graph, annotating)}
	for _, relationship := range graph.ObjectsView(annotating, rdf.SysML+pOwnedRelationship) {
		if apiJSONProjectionMetaclass(graph, relationship) != "FeatureTyping" {
			continue
		}
		for _, typed := range graph.ObjectsView(relationship, rdf.SysML+"type") {
			if name, ok := graph.Lexical(typed, rdf.SysML+pQualifiedName); ok {
				names = append(names, name)
				segments := naming.Segments(name)
				last := segments[len(segments)-1]
				names = append(names, last, naming.Unquote(last))
			}
			if short, ok := graph.Lexical(typed, rdf.SysML+pDeclaredShortName); ok {
				names = append(names, short)
			}
		}
	}
	return names
}
