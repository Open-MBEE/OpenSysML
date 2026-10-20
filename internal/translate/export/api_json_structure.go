package export

import (
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

type apiJSONGraphStructure struct {
	graph                       *rdf.Graph
	bySubject                   map[string]metamodel.Element
	byElement                   map[metamodel.ElementKey]rdf.Term
	byID                        map[string][]rdf.Term
	ownedRelationshipsByEnd     map[string][]rdf.Term
	owningRelationshipByOwned   map[string][]rdf.Term
	owningMembershipByMember    map[string][]rdf.Term
	owningRelatedByRelationship map[string][]rdf.Term
}

type apiJSONEvaluation struct {
	structure *apiJSONGraphStructure
	evaluator *metamodel.Evaluator
}

func newAPIJSONEvaluation(graph *rdf.Graph) *apiJSONEvaluation {
	graph = apiJSONEvaluationGraph(graph)
	structure := newAPIJSONGraphStructure(graph)
	return &apiJSONEvaluation{
		structure: structure,
		evaluator: metamodel.New(nil, nil, metamodel.Options{Structure: structure}),
	}
}

func apiJSONEvaluationGraph(graph *rdf.Graph) *rdf.Graph {
	if graph == nil {
		return nil
	}
	catalog := identity.LibraryCatalogWithOpenSysML(libs.SharedBase())
	subjects := make(map[string]bool, len(graph.SubjectsView()))
	for _, subject := range graph.SubjectsView() {
		subjects[subject.Value] = true
	}
	libraryReferences := make(map[rdf.Term]*identity.LibraryElement)
	for _, triple := range graph.Triples() {
		if !triple.Object.IsIRI() || subjects[triple.Object.Value] {
			continue
		}
		id, ok := rdf.SubjectID(triple.Object)
		if !ok {
			continue
		}
		library, ok := catalog.Element(id)
		if !ok || library.Symbol == nil {
			continue
		}
		libraryReferences[triple.Object] = library
	}
	if len(libraryReferences) == 0 {
		return graph
	}
	additions := make([]rdf.Triple, 0, len(libraryReferences)*5)
	add := func(subject, predicate, object rdf.Term) {
		additions = append(additions, rdf.Triple{Subject: subject, Predicate: predicate, Object: object})
	}
	for subject, library := range libraryReferences {
		id, ok := rdf.SubjectID(subject)
		if !ok {
			continue
		}
		metaclass := declaredMetaclass(library.Symbol.Decl)
		if metaclass == "" {
			continue
		}
		add(subject, rdf.IRI(rdf.RDFType), rdf.SysMLTerm(metaclass))
		add(subject, rdf.SysMLTerm("elementId"), rdf.String(id))
		add(subject, rdf.SysMLTerm("isLibraryElement"), rdf.Bool(true))
		if library.Symbol.Name != "" {
			add(subject, rdf.SysMLTerm("declaredName"), rdf.String(library.Symbol.Name))
		}
		if library.FQN != "" {
			add(subject, rdf.SysMLTerm(pQualifiedName), rdf.String(library.FQN))
		}
	}
	triples := make([]rdf.Triple, 0, graph.Len()+len(additions))
	triples = append(triples, graph.Triples()...)
	triples = append(triples, additions...)
	return rdf.NewGraphOfWithIndexOverlay(triples, graph.Prefixes, graph, additions)
}

func newAPIJSONGraphStructure(graph *rdf.Graph) *apiJSONGraphStructure {
	s := &apiJSONGraphStructure{
		graph:                       graph,
		bySubject:                   make(map[string]metamodel.Element),
		byElement:                   make(map[metamodel.ElementKey]rdf.Term),
		byID:                        make(map[string][]rdf.Term),
		ownedRelationshipsByEnd:     make(map[string][]rdf.Term),
		owningRelationshipByOwned:   make(map[string][]rdf.Term),
		owningMembershipByMember:    make(map[string][]rdf.Term),
		owningRelatedByRelationship: make(map[string][]rdf.Term),
	}
	if graph == nil {
		return s
	}
	subjects := graph.SubjectsView()
	for _, subject := range subjects {
		class, ok := s.metaclass(subject)
		if !ok {
			continue
		}
		element := metamodel.Element{Aspect: "graph:" + subject.Value}
		if ontology.IsAncestorOrSelf(class, "Membership") {
			element = metamodel.MembershipElement(metamodel.Membership{Aspect: "graph:" + subject.Value})
		}
		s.bySubject[subject.Value] = element
		s.byElement[element.Key()] = subject
		if id, ok := rdf.SubjectID(subject); ok {
			s.byID[id] = append(s.byID[id], subject)
		}
	}
	for _, subject := range subjects {
		if _, ok := s.bySubject[subject.Value]; !ok {
			continue
		}
		class, _ := s.metaclass(subject)
		if ontology.IsAncestorOrSelf(class, "Relationship") {
			for _, owner := range graph.ObjectsView(subject, rdf.SysML+"owningRelatedElement") {
				s.ownedRelationshipsByEnd[owner.Value] = append(s.ownedRelationshipsByEnd[owner.Value], subject)
			}
			for _, member := range graph.ObjectsView(subject, rdf.SysML+"ownedRelatedElement") {
				s.owningRelationshipByOwned[member.Value] = append(s.owningRelationshipByOwned[member.Value], subject)
			}
		}
		if ontology.IsAncestorOrSelf(class, "Membership") {
			property, declared := ontology.PropertyOf(class, "memberElement")
			if declared && !property.Derived {
				for _, member := range graph.ObjectsView(subject, rdf.SysML+"memberElement") {
					s.owningMembershipByMember[member.Value] = append(s.owningMembershipByMember[member.Value], subject)
				}
			}
			for _, namespace := range graph.ObjectsView(subject, rdf.SysML+"membershipOwningNamespace") {
				s.ownedRelationshipsByEnd[namespace.Value] = append(s.ownedRelationshipsByEnd[namespace.Value], subject)
				s.owningRelatedByRelationship[subject.Value] = append(s.owningRelatedByRelationship[subject.Value], namespace)
			}
		}
		for _, relationship := range graph.ObjectsView(subject, rdf.SysML+"ownedRelationship") {
			s.owningRelatedByRelationship[relationship.Value] = append(
				s.owningRelatedByRelationship[relationship.Value], subject,
			)
		}
	}
	return s
}

func (s *apiJSONGraphStructure) metaclass(subject rdf.Term) (string, bool) {
	if s.graph == nil {
		return "", false
	}
	typ := s.graph.Type(subject)
	if typ == "" {
		return "", false
	}
	if class := apiJSONOwnedAbsenceMetaclassInGraph(s.graph, typ); class != "" {
		return class, true
	}
	if strings.HasPrefix(typ, rdf.OpenSysML) {
		return metamodel.ExtensionMetaclassPrefix + strings.TrimPrefix(typ, rdf.OpenSysML), true
	}
	return "", false
}

func (s *apiJSONGraphStructure) subject(element metamodel.Element) (rdf.Term, bool) {
	subject, ok := s.byElement[element.Key()]
	return subject, ok
}

func (s *apiJSONGraphStructure) element(subject rdf.Term) (metamodel.Element, bool) {
	element, ok := s.bySubject[subject.Value]
	return element, ok
}

func (s *apiJSONGraphStructure) Metaclass(element metamodel.Element) (string, bool) {
	subject, ok := s.subject(element)
	if !ok {
		return "", false
	}
	return s.metaclass(subject)
}

func (*apiJSONGraphStructure) Specializes(sub, super string) bool {
	return ontology.IsAncestorOrSelf(sub, super)
}

func (s *apiJSONGraphStructure) OwnedRelationships(element metamodel.Element) ([]metamodel.Element, bool) {
	subject, ok := s.subject(element)
	if !ok || LibraryReference(s.graph, subject) {
		return nil, false
	}
	terms := append([]rdf.Term(nil), s.graph.ObjectsView(subject, rdf.SysML+"ownedRelationship")...)
	terms = append(terms, s.ownedRelationshipsByEnd[subject.Value]...)
	if len(terms) == 0 {
		class, known := s.metaclass(subject)
		definition, declared := ontology.PropertyOf(class, "ownedRelationship")
		_, annotated := s.graph.Object(subject, rdf.AnnotationJSON+"ownedRelationship")
		if (!known || !declared || definition.Derived) && !annotated {
			return nil, false
		}
	}
	seen := make(map[string]bool, len(terms))
	relationships := make([]metamodel.Element, 0, len(terms))
	for _, term := range terms {
		if seen[term.Value] {
			continue
		}
		seen[term.Value] = true
		relationship, ok := s.element(term)
		if !ok {
			return nil, false
		}
		relationships = append(relationships, relationship)
	}
	return relationships, true
}

func (s *apiJSONGraphStructure) OwningRelationship(element metamodel.Element) (metamodel.Element, bool, bool) {
	subject, ok := s.subject(element)
	if !ok || LibraryReference(s.graph, subject) {
		return metamodel.Element{}, false, false
	}
	if relationship, present, known := s.relatedOne(subject, "owningRelationship"); !known || present {
		return relationship, present, known
	}
	relationships := s.owningRelationshipByOwned[subject.Value]
	if len(relationships) == 0 {
		relationships = s.owningMembershipByMember[subject.Value]
	}
	if len(relationships) == 0 {
		return metamodel.Element{}, false, true
	}
	if len(relationships) != 1 {
		return metamodel.Element{}, false, false
	}
	relationship, ok := s.element(relationships[0])
	return relationship, ok, ok
}

func (s *apiJSONGraphStructure) OwnedRelatedElements(element metamodel.Element) ([]metamodel.Element, bool) {
	subject, ok := s.subject(element)
	if !ok || LibraryReference(s.graph, subject) {
		return nil, false
	}
	related, known := s.related(subject, "ownedRelatedElement")
	if !known || len(related) > 0 {
		return related, known
	}
	class, _ := s.metaclass(subject)
	if !ontology.IsAncestorOrSelf(class, "OwningMembership") {
		return related, true
	}
	related, known = s.related(subject, "memberElement")
	if known && len(related) > 0 {
		return related, true
	}
	return s.related(subject, "ownedMemberElement")
}

func (s *apiJSONGraphStructure) OwningRelatedElement(element metamodel.Element) (metamodel.Element, bool, bool) {
	subject, ok := s.subject(element)
	if !ok || LibraryReference(s.graph, subject) {
		return metamodel.Element{}, false, false
	}
	if related, present, known := s.relatedOne(subject, "owningRelatedElement"); !known || present {
		return related, present, known
	}
	class, _ := s.metaclass(subject)
	property := ""
	switch {
	case ontology.IsAncestorOrSelf(class, "Membership"):
		property = "membershipOwningNamespace"
	case class == "FeatureChaining":
		property = "owner"
	case class == "Annotation":
		property = "annotatedElement"
	}
	if property != "" {
		if related, present, known := s.relatedOne(subject, property); !known || present {
			return related, present, known
		}
	}
	owners := s.owningRelatedByRelationship[subject.Value]
	if len(owners) == 0 {
		return metamodel.Element{}, false, true
	}
	if len(owners) != 1 {
		return metamodel.Element{}, false, false
	}
	owner, ok := s.element(owners[0])
	return owner, ok, ok
}

func (s *apiJSONGraphStructure) RelatedElements(element metamodel.Element, property string) ([]metamodel.Element, bool) {
	subject, ok := s.subject(element)
	if !ok || LibraryReference(s.graph, subject) {
		return nil, false
	}
	class, ok := s.metaclass(subject)
	if !ok || strings.HasPrefix(class, metamodel.ExtensionMetaclassPrefix) {
		return nil, false
	}
	definition, ok := ontology.PropertyOf(class, property)
	if !ok || definition.Kind != ontology.ObjectProperty {
		return nil, false
	}
	return s.related(subject, property)
}

func (s *apiJSONGraphStructure) related(subject rdf.Term, property string) ([]metamodel.Element, bool) {
	if annotation, ok := s.graph.Object(subject, rdf.AnnotationJSON+property); ok {
		if !annotation.IsLiteral() {
			return nil, false
		}
		members, err := rdf.ParseCollectionJSON(annotation.Value)
		if err != nil {
			return nil, false
		}
		values := make([]metamodel.Element, 0, len(members))
		for _, member := range members {
			if member.ID == "" {
				return nil, false
			}
			targets := s.byID[member.ID]
			if len(targets) != 1 {
				ref := rdf.ReferenceIRI(subject, member.ID)
				if element, ok := s.element(ref); ok {
					values = append(values, element)
					continue
				}
				return nil, false
			}
			element, ok := s.element(targets[0])
			if !ok {
				return nil, false
			}
			values = append(values, element)
		}
		return values, true
	}
	objects := s.graph.ObjectsView(subject, rdf.SysML+property)
	values := make([]metamodel.Element, 0, len(objects))
	for _, object := range objects {
		element, ok := s.element(object)
		if !ok {
			return nil, false
		}
		values = append(values, element)
	}
	return values, true
}

func (s *apiJSONGraphStructure) relatedOne(subject rdf.Term, property string) (metamodel.Element, bool, bool) {
	objects := s.graph.ObjectsView(subject, rdf.SysML+property)
	if len(objects) == 0 {
		return metamodel.Element{}, false, true
	}
	if len(objects) != 1 {
		return metamodel.Element{}, false, false
	}
	element, ok := s.element(objects[0])
	return element, ok, ok
}

func (s *apiJSONGraphStructure) Attribute(element metamodel.Element, property string) (metamodel.Value, bool) {
	subject, ok := s.subject(element)
	if !ok {
		return metamodel.Value{}, false
	}
	class, ok := s.metaclass(subject)
	if !ok || strings.HasPrefix(class, metamodel.ExtensionMetaclassPrefix) {
		return metamodel.Value{}, false
	}
	graphProperty := apiJSONGraphAttributeName(property)
	definition, ok := ontology.PropertyOf(class, graphProperty)
	if !ok || definition.Kind != ontology.DatatypeProperty {
		return metamodel.Value{}, false
	}
	objects := s.graph.ObjectsView(subject, rdf.SysML+graphProperty)
	if len(objects) == 0 {
		if definition.Lower > 0 {
			return metamodel.Value{}, false
		}
		if definition.Many {
			return metamodel.Value{Kind: metamodel.SequenceValue, Values: []metamodel.Value{}, Success: true}, true
		}
		return metamodel.Value{Kind: metamodel.NullValue, Success: true}, true
	}
	values := make([]metamodel.Value, 0, len(objects))
	for _, object := range objects {
		if object.Kind == rdf.TermIRI {
			return metamodel.Value{}, false
		}
		switch object.Datatype {
		case rdf.XSD + "boolean":
			values = append(values, metamodel.Value{Kind: metamodel.BooleanValue, Boolean: object.Value == "true" || object.Value == "1", Success: true})
		case rdf.XSD + "integer", rdf.XSD + "int", rdf.XSD + "long", rdf.XSD + "short", rdf.XSD + "byte":
			value, err := strconv.ParseInt(object.Value, 10, 64)
			if err != nil {
				return metamodel.Value{}, false
			}
			values = append(values, metamodel.Value{Kind: metamodel.IntegerValue, Integer: value, Success: true})
		case rdf.XSD + "double", rdf.XSD + "float", rdf.XSD + "decimal":
			value, err := strconv.ParseFloat(object.Value, 64)
			if err != nil {
				return metamodel.Value{}, false
			}
			values = append(values, metamodel.Value{Kind: metamodel.RealValue, Real: value, Success: true})
		default:
			values = append(values, metamodel.Value{Kind: metamodel.StringValue, String: object.Value, Success: true})
		}
	}
	if len(values) == 1 {
		return values[0], true
	}
	return metamodel.Value{Kind: metamodel.SequenceValue, Values: values, Success: true}, true
}

func (s *apiJSONGraphStructure) StatedAttribute(element metamodel.Element, property string) (metamodel.Value, bool) {
	subject, ok := s.subject(element)
	if !ok || !s.graph.HasProperty(subject, rdf.SysML+apiJSONGraphAttributeName(property)) {
		return metamodel.Value{}, false
	}
	return s.Attribute(element, property)
}

func apiJSONGraphAttributeName(property string) string {
	switch property {
	case "name":
		return "declaredName"
	case "shortName":
		return "declaredShortName"
	default:
		return property
	}
}

func (s *apiJSONGraphStructure) StatedRelatedElements(element metamodel.Element, property string) ([]metamodel.Element, bool) {
	subject, ok := s.subject(element)
	if !ok {
		return nil, false
	}
	class, ok := s.metaclass(subject)
	if !ok || strings.HasPrefix(class, metamodel.ExtensionMetaclassPrefix) {
		return nil, false
	}
	definition, ok := ontology.PropertyOf(class, property)
	if !ok || definition.Kind != ontology.ObjectProperty || definition.Derived {
		return nil, false
	}
	if !s.graph.HasProperty(subject, rdf.SysML+property) {
		return nil, false
	}
	return s.related(subject, property)
}

func (s *apiJSONGraphStructure) HasStatedRelatedElement(element metamodel.Element, property string) bool {
	subject, ok := s.subject(element)
	if !ok {
		return false
	}
	class, ok := s.metaclass(subject)
	if !ok || strings.HasPrefix(class, metamodel.ExtensionMetaclassPrefix) {
		return false
	}
	definition, ok := ontology.PropertyOf(class, property)
	return ok && definition.Kind == ontology.ObjectProperty &&
		!definition.Derived && len(s.graph.ObjectsView(subject, rdf.SysML+property)) > 0
}

func (s *apiJSONGraphStructure) HasUnresolvedRelatedElement(element metamodel.Element, property string) bool {
	subject, ok := s.subject(element)
	if !ok {
		return false
	}
	for _, object := range s.graph.ObjectsView(subject, rdf.SysML+property) {
		if object.IsIRI() {
			if _, ok := s.element(object); !ok {
				return true
			}
		}
	}
	return false
}

func (s *apiJSONGraphStructure) ElementIdentity(element metamodel.Element) (string, bool) {
	subject, ok := s.subject(element)
	if !ok {
		return "", false
	}
	return rdf.SubjectID(subject)
}

func (s *apiJSONGraphStructure) QualifiedName(element metamodel.Element) (string, bool) {
	subject, ok := s.subject(element)
	if !ok || !LibraryReference(s.graph, subject) {
		return "", false
	}
	return s.graph.Lexical(subject, rdf.SysML+pQualifiedName)
}

func (s *apiJSONGraphStructure) EffectiveName(element metamodel.Element) (string, bool) {
	subject, ok := s.subject(element)
	if !ok || !s.graph.BoolValue(subject, rdf.OpenSysML+"isKindImplicit") {
		return "", false
	}
	return "kind", true
}
