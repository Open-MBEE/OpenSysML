package export

import (
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

type graphStructure struct {
	graph                     *rdf.Graph
	subjects                  *semanticSubjectMap
	annotationsByAnnotated    map[rdf.Term][]rdf.Term
	owningAnnotatingByElement map[rdf.Term][]rdf.Term
	ownedRelationshipsByOwner map[rdf.Term]ownedRelationshipIndexEntry
}

type ownedRelationshipIndexEntry struct {
	relationships []metamodel.Element
	known         bool
}

func newGraphStructure(graph *rdf.Graph, subjects *semanticSubjectMap) *graphStructure {
	structure := &graphStructure{
		graph:                     graph,
		subjects:                  subjects,
		annotationsByAnnotated:    make(map[rdf.Term][]rdf.Term),
		owningAnnotatingByElement: make(map[rdf.Term][]rdf.Term),
		ownedRelationshipsByOwner: make(map[rdf.Term]ownedRelationshipIndexEntry),
	}
	if graph == nil {
		return structure
	}
	graphSubjects := graph.Subjects()
	for _, relation := range graphSubjects {
		metaclass := graph.Type(relation)
		if !strings.HasPrefix(metaclass, rdf.SysML) ||
			!ontology.IsAncestorOrSelf(strings.TrimPrefix(metaclass, rdf.SysML), "Annotation") {
			continue
		}
		for _, related := range graph.Objects(relation, rdf.SysML+pOwnedRelatedElement) {
			relatedMetaclass := graph.Type(related)
			if strings.HasPrefix(relatedMetaclass, rdf.SysML) &&
				ontology.IsAncestorOrSelf(strings.TrimPrefix(relatedMetaclass, rdf.SysML), "AnnotatingElement") {
				structure.owningAnnotatingByElement[related] = append(
					structure.owningAnnotatingByElement[related], relation,
				)
			}
		}
		annotated, ok := graph.Object(relation, rdf.SysML+pAnnotatedElement)
		if ok {
			structure.annotationsByAnnotated[annotated] = append(
				structure.annotationsByAnnotated[annotated], relation,
			)
		}
	}
	for annotated, relations := range structure.annotationsByAnnotated {
		sort.SliceStable(relations, func(i, j int) bool {
			left, leftOK := structure.element(relations[i])
			right, rightOK := structure.element(relations[j])
			if !leftOK || left.Node == nil {
				return false
			}
			if !rightOK || right.Node == nil {
				return true
			}
			return left.Node.Span().Offset < right.Node.Span().Offset
		})
		structure.annotationsByAnnotated[annotated] = relations
	}
	for _, owner := range graphSubjects {
		metaclass := graph.Type(owner)
		if !strings.HasPrefix(metaclass, rdf.SysML) {
			continue
		}
		if structure.libraryStub(owner) {
			structure.ownedRelationshipsByOwner[owner] = ownedRelationshipIndexEntry{}
			continue
		}
		property, known := ontology.PropertyOf(strings.TrimPrefix(metaclass, rdf.SysML), pOwnedRelationship)
		if !known || property.Derived {
			structure.ownedRelationshipsByOwner[owner] = ownedRelationshipIndexEntry{}
			continue
		}
		terms := graph.Objects(owner, rdf.SysML+pOwnedRelationship)
		relationships := make([]metamodel.Element, 0, len(terms))
		seen := make(map[metamodel.ElementKey]bool, len(terms))
		known = true
		for _, term := range terms {
			relationship, ok := structure.element(term)
			if !ok {
				known = false
				break
			}
			if !seen[relationship.Key()] {
				seen[relationship.Key()] = true
				relationships = append(relationships, relationship)
			}
		}
		for _, term := range structure.annotationsByAnnotated[owner] {
			relationship, ok := structure.element(term)
			if !ok {
				known = false
				break
			}
			if !seen[relationship.Key()] {
				seen[relationship.Key()] = true
				relationships = append(relationships, relationship)
			}
		}
		structure.ownedRelationshipsByOwner[owner] = ownedRelationshipIndexEntry{
			relationships: relationships,
			known:         known,
		}
	}
	return structure
}

func (s *graphStructure) Metaclass(element metamodel.Element) (string, bool) {
	subject, ok := s.subject(element)
	if !ok {
		return "", false
	}
	metaclass := s.graph.Type(subject)
	if !strings.HasPrefix(metaclass, rdf.SysML) {
		return "", false
	}
	return strings.TrimPrefix(metaclass, rdf.SysML), true
}

func (*graphStructure) Specializes(sub, super string) bool {
	return ontology.IsAncestorOrSelf(sub, super)
}

func (s *graphStructure) OwnedRelationships(element metamodel.Element) ([]metamodel.Element, bool) {
	subject, ok := s.subject(element)
	if !ok {
		return nil, false
	}
	if s.libraryStubElement(element) || s.libraryStub(subject) {
		return nil, false
	}
	owned, ok := s.ownedRelationshipsByOwner[subject]
	if !ok || !owned.known {
		return nil, false
	}
	return owned.relationships, true
}

func (s *graphStructure) OwningRelationship(element metamodel.Element) (metamodel.Element, bool, bool) {
	return s.relatedOne(element, pOwningRelationship)
}

func (s *graphStructure) OwnedRelatedElements(element metamodel.Element) ([]metamodel.Element, bool) {
	return s.related(element, pOwnedRelatedElement)
}

func (s *graphStructure) OwningRelatedElement(element metamodel.Element) (metamodel.Element, bool, bool) {
	subject, ok := s.subject(element)
	if !ok || s.libraryStubElement(element) || s.libraryStub(subject) {
		return metamodel.Element{}, false, false
	}
	if strings.TrimPrefix(s.graph.Type(subject), rdf.SysML) == "Annotation" {
		if owners := s.graph.Objects(subject, rdf.SysML+pOwningRelatedElement); len(owners) == 1 {
			owner, ok := s.element(owners[0])
			return owner, ok, ok
		} else if len(owners) > 1 {
			return metamodel.Element{}, false, false
		}
		return s.relatedOne(element, pAnnotatedElement)
	}
	return s.relatedOne(element, pOwningRelatedElement)
}

func (s *graphStructure) OwningAnnotatingRelationship(element metamodel.Element) (metamodel.Element, bool, bool) {
	subject, ok := s.subject(element)
	if !ok || s.libraryStubElement(element) || s.libraryStub(subject) {
		return metamodel.Element{}, false, false
	}
	relationships := s.owningAnnotatingByElement[subject]
	if len(relationships) == 0 {
		return metamodel.Element{}, false, true
	}
	if len(relationships) != 1 {
		return metamodel.Element{}, false, false
	}
	relationship, ok := s.element(relationships[0])
	if !ok {
		return metamodel.Element{}, false, false
	}
	return relationship, true, true
}

func (s *graphStructure) RelatedElements(element metamodel.Element, property string) ([]metamodel.Element, bool) {
	if property == "guardExpression" {
		return s.relatedByPredicate(element, rdf.OpenSysML+xGuard)
	}
	subject, ok := s.subject(element)
	if !ok {
		return nil, false
	}
	if (s.libraryStubElement(element) || s.libraryStub(subject)) &&
		(strings.HasPrefix(property, "owned") || strings.HasPrefix(property, "owning")) {
		return nil, false
	}
	metaclass := s.graph.Type(subject)
	if !strings.HasPrefix(metaclass, rdf.SysML) {
		return nil, false
	}
	definition, ok := ontology.PropertyOf(strings.TrimPrefix(metaclass, rdf.SysML), property)
	if !ok || definition.Derived || definition.Kind != ontology.ObjectProperty {
		return nil, false
	}
	return s.related(element, property)
}

func (s *graphStructure) Attribute(element metamodel.Element, property string) (metamodel.Value, bool) {
	subject, ok := s.subject(element)
	if !ok {
		return metamodel.Value{}, false
	}
	metaclass := s.graph.Type(subject)
	if !strings.HasPrefix(metaclass, rdf.SysML) {
		return metamodel.Value{}, false
	}
	definition, ok := ontology.PropertyOf(strings.TrimPrefix(metaclass, rdf.SysML), property)
	if !ok || definition.Derived || definition.Kind != ontology.DatatypeProperty {
		return metamodel.Value{}, false
	}
	objects := s.graph.Objects(subject, rdf.SysML+property)
	if len(objects) == 0 {
		if !definition.HasDefault {
			if definition.Lower != 0 {
				return metamodel.Value{}, false
			}
			if definition.Many {
				return metamodel.Value{Kind: metamodel.SequenceValue, Values: []metamodel.Value{}, Success: true}, true
			}
			return metamodel.Value{Kind: metamodel.NullValue, Success: true}, true
		}
		switch definition.Range {
		case rdf.XSD + "boolean":
			return metamodel.Value{
				Kind: metamodel.BooleanValue, Boolean: definition.Default == "true",
				Success: true,
			}, true
		case rdf.XSD + "integer", rdf.XSD + "int", rdf.XSD + "long", rdf.XSD + "short", rdf.XSD + "byte":
			value, err := strconv.ParseInt(definition.Default, 10, 64)
			if err != nil {
				return metamodel.Value{}, false
			}
			return metamodel.Value{Kind: metamodel.IntegerValue, Integer: value, Success: true}, true
		default:
			return metamodel.Value{Kind: metamodel.StringValue, String: definition.Default, Success: true}, true
		}
	}
	values := make([]metamodel.Value, 0, len(objects))
	for _, object := range objects {
		value, ok := s.value(object)
		if !ok {
			return metamodel.Value{}, false
		}
		values = append(values, value)
	}
	if len(values) == 1 {
		return values[0], true
	}
	return metamodel.Value{Kind: metamodel.SequenceValue, Values: values, Success: true}, true
}

func (s *graphStructure) subject(element metamodel.Element) (rdf.Term, bool) {
	if element.IsMembership {
		if subject, ok := s.subjects.byMembership[element.Membership.Key()]; ok {
			return subject, true
		}
		return s.subjects.libraryMembershipTerm(element.Membership)
	}
	if subject, ok := s.subjects.byElement[element.Key()]; ok {
		return subject, true
	}
	return s.subjects.libraryElementTerm(element)
}

func (s *graphStructure) ElementIdentity(element metamodel.Element) (string, bool) {
	subject, ok := s.subject(element)
	return subject.Value, ok && subject.IsIRI()
}

func (s *graphStructure) related(element metamodel.Element, property string) ([]metamodel.Element, bool) {
	return s.relatedByPredicate(element, rdf.SysML+property)
}

func (s *graphStructure) relatedByPredicate(element metamodel.Element, predicate string) ([]metamodel.Element, bool) {
	subject, ok := s.subject(element)
	if !ok {
		return nil, false
	}
	if s.libraryStubElement(element) || s.libraryStub(subject) {
		return nil, false
	}
	objects := s.graph.Objects(subject, predicate)
	values := make([]metamodel.Element, 0, len(objects))
	for _, object := range objects {
		value, ok := s.element(object)
		if !ok {
			return nil, false
		}
		values = append(values, value)
	}
	sort.SliceStable(values, func(i, j int) bool {
		left, right := values[i].Node, values[j].Node
		if left == nil {
			return false
		}
		if right == nil {
			return true
		}
		return left.Span().Offset < right.Span().Offset
	})
	return values, true
}

func (s *graphStructure) relatedOne(element metamodel.Element, property string) (metamodel.Element, bool, bool) {
	subject, ok := s.subject(element)
	if !ok {
		return metamodel.Element{}, false, false
	}
	if s.libraryStubElement(element) || s.libraryStub(subject) {
		return metamodel.Element{}, false, false
	}
	objects := s.graph.Objects(subject, rdf.SysML+property)
	if len(objects) == 0 {
		return metamodel.Element{}, false, true
	}
	if len(objects) != 1 {
		return metamodel.Element{}, false, false
	}
	value, ok := s.element(objects[0])
	return value, ok, ok
}

func (s *graphStructure) libraryStub(subject rdf.Term) bool {
	objects := s.graph.Objects(subject, rdf.SysML+"isLibraryElement")
	return len(objects) == 1 && objects[0].Kind == rdf.TermLiteral &&
		(objects[0].Value == "true" || objects[0].Value == "1")
}

func (s *graphStructure) libraryStubElement(element metamodel.Element) bool {
	return s.subjects != nil && s.subjects.isLibraryHandle(element)
}

func (s *graphStructure) element(term rdf.Term) (metamodel.Element, bool) {
	element, ok := s.subjects.byIRI[term.Value]
	if ok {
		return element, true
	}
	id, hasID := rdf.SubjectID(term)
	if !hasID || s.subjects.resolver == nil || s.subjects.resolver.Index() == nil {
		return metamodel.Element{}, false
	}
	catalog := identity.LibraryCatalog(s.subjects.resolver.Index())
	if library, found := catalog.Element(id); found {
		return metamodel.ElementOf(library.Symbol), true
	}
	if library, found := catalog.OwningMembership(id); found {
		return metamodel.MembershipElement(metamodel.ElementOf(library.Symbol).Membership), true
	}
	return metamodel.Element{}, false
}

func (s *graphStructure) value(term rdf.Term) (metamodel.Value, bool) {
	if term.Kind == rdf.TermIRI {
		element, ok := s.element(term)
		if !ok {
			return metamodel.Value{}, false
		}
		return metamodel.Value{Kind: metamodel.ElementValue, Element: element, Success: true}, true
	}
	switch term.Datatype {
	case rdf.XSD + "boolean":
		return metamodel.Value{Kind: metamodel.BooleanValue, Boolean: term.Value == "true" || term.Value == "1", Success: true}, true
	case rdf.XSD + "integer", rdf.XSD + "int", rdf.XSD + "long", rdf.XSD + "short", rdf.XSD + "byte":
		value, err := strconv.ParseInt(term.Value, 10, 64)
		if err != nil {
			return metamodel.Value{}, false
		}
		return metamodel.Value{Kind: metamodel.IntegerValue, Integer: value, Success: true}, true
	case rdf.XSD + "double", rdf.XSD + "float", rdf.XSD + "decimal":
		value, err := strconv.ParseFloat(term.Value, 64)
		if err != nil {
			return metamodel.Value{}, false
		}
		return metamodel.Value{Kind: metamodel.RealValue, Real: value, Success: true}, true
	default:
		return metamodel.Value{Kind: metamodel.StringValue, String: term.Value, Success: true}, true
	}
}
