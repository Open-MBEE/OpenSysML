package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

// APIJSONForm selects the API JSON element form.
type APIJSONForm uint8

const (
	// APIJSONCompact writes only properties stated in the graph.
	APIJSONCompact APIJSONForm = iota
	// APIJSONFull appends ontology properties and faithfully derived values.
	APIJSONFull
)

// ParseAPIJSONForm parses a form name accepted by conversion surfaces.
func ParseAPIJSONForm(s string) (APIJSONForm, bool) {
	switch s {
	case "compact":
		return APIJSONCompact, true
	case "full":
		return APIJSONFull, true
	default:
		return APIJSONCompact, false
	}
}

// ModelToAPIJSON converts model documents to compact or full API JSON.
func ModelToAPIJSON(documents []ModelDocument, ids IDForm, form APIJSONForm) ([]byte, error) {
	output, _, err := modelToAPIJSON(documents, ids, form)
	return output, err
}

func modelToAPIJSON(documents []ModelDocument, ids IDForm, form APIJSONForm) ([]byte, []serializationFailure, error) {
	if form != APIJSONCompact && form != APIJSONFull {
		return nil, nil, fmt.Errorf("unknown API JSON form %d", form)
	}
	graph, res, encoders, err := modelToRDF(documents, ids)
	if err != nil {
		return nil, nil, err
	}
	if form == APIJSONCompact {
		output, err := WriteAPIJSON(graph)
		return output, nil, err
	}
	if len(encoders) == 0 {
		output, err := WriteAPIJSON(graph)
		return output, nil, err
	}
	subjects := buildSemanticSubjects(graph, res, encoders)
	settled, err := prepareFullAPIJSONGraph(graph, subjects)
	if err != nil {
		return nil, nil, err
	}
	evaluator := metamodel.New(res, encoders[0].ids.model, metamodel.Options{
		Structure: newGraphStructure(settled, subjects),
	})
	return writeFullAPIJSONSettled(settled, evaluator, subjects)
}

type semanticSubjectMap struct {
	byIRI        map[string]metamodel.Element
	byElement    map[metamodel.ElementKey]rdf.Term
	byMembership map[metamodel.MembershipKey]rdf.Term
	resolver     *resolve.Resolver
	catalog      *identity.Catalog
	failures     []serializationFailure
}

type serializationFailure struct {
	Property string
	Reason   string
	Source   string
	Target   string
}

type apiJSONOntology struct {
	propertiesByClass    map[string][]ontology.Property
	redefinitionsByClass map[string]map[string]ontology.Property
}

var apiOntology = newAPIJSONOntology()

func newAPIJSONOntology() apiJSONOntology {
	allProperties := ontology.Properties()
	classes := make(map[string]ontology.Class)
	propertiesByClass := make(map[string][]ontology.Property)
	propertiesByName := make(map[string]ontology.Property, len(allProperties))
	for _, class := range ontology.Classes() {
		classes[class.Name] = class
	}
	for _, property := range allProperties {
		propertiesByClass[property.DefiningClass] = append(propertiesByClass[property.DefiningClass], property)
		propertiesByName[property.QualifiedName()] = property
	}
	lineage := make(map[string][]string, len(classes))
	var classLineage func(string) []string
	classLineage = func(name string) []string {
		if cached, ok := lineage[name]; ok {
			return cached
		}
		seen := make(map[string]bool)
		var out []string
		var visit func(string)
		visit = func(current string) {
			if seen[current] {
				return
			}
			seen[current] = true
			out = append(out, current)
			for _, parent := range classes[current].Parents {
				visit(parent)
			}
		}
		visit(name)
		lineage[name] = out
		return out
	}
	properties := make(map[string][]ontology.Property, len(classes))
	redefinitions := make(map[string]map[string]ontology.Property, len(classes))
	for name := range classes {
		order := classLineage(name)
		seen := make(map[string]bool)
		for _, class := range order {
			for _, property := range propertiesByClass[class] {
				if seen[property.Name] {
					continue
				}
				if resolved, ok := ontology.PropertyOf(name, property.Name); ok {
					property = resolved
				}
				seen[property.Name] = true
				properties[name] = append(properties[name], property)
			}
		}
		overrides := make(map[string]ontology.Property)
		for _, class := range order {
			for _, candidate := range propertiesByClass[class] {
				visited := make(map[string]bool)
				var collect func(string)
				collect = func(target string) {
					if visited[target] {
						return
					}
					visited[target] = true
					if _, ok := overrides[target]; !ok {
						overrides[target] = candidate
					}
					if redefined, ok := propertiesByName[target]; ok {
						for _, ancestor := range redefined.Redefines {
							collect(ancestor)
						}
					}
				}
				for _, target := range candidate.Redefines {
					collect(target)
				}
			}
		}
		redefinitions[name] = overrides
	}
	return apiJSONOntology{propertiesByClass: properties, redefinitionsByClass: redefinitions}
}

func newSemanticSubjectMap() *semanticSubjectMap {
	return &semanticSubjectMap{
		byIRI:        make(map[string]metamodel.Element),
		byElement:    make(map[metamodel.ElementKey]rdf.Term),
		byMembership: make(map[metamodel.MembershipKey]rdf.Term),
	}
}

func (s *semanticSubjectMap) add(subject rdf.Term, element metamodel.Element) {
	if subject.Value == "" {
		return
	}
	s.byIRI[subject.Value] = element
	s.byElement[element.Key()] = subject
	if element.IsMembership {
		s.byMembership[element.Membership.Key()] = subject
	}
}

func (s *semanticSubjectMap) addMembership(subject rdf.Term, membership metamodel.Membership) {
	if subject.Value == "" {
		return
	}
	s.byMembership[membership.Key()] = subject
	s.byElement[metamodel.MembershipElement(membership).Key()] = subject
}

func buildSemanticSubjects(graph *rdf.Graph, res *resolve.Resolver, encoders []*encoder) *semanticSubjectMap {
	mapped := newSemanticSubjectMap()
	mapped.resolver = res
	mapped.catalog = identity.LibraryCatalog(res.Index())
	for _, libraryElement := range mapped.catalog.Elements() {
		element := metamodel.ElementOf(libraryElement.Symbol)
		mapped.add(rdf.ElementIRIForID(libraryElement.ID), element)
		membership := metamodel.MembershipElement(element.Membership)
		if hasSemanticMembership(element.Membership) {
			membershipTerm := rdf.ElementIRIForID(libraryElement.OwningMembershipID)
			mapped.addMembership(membershipTerm, membership.Membership)
		}
	}
	graphSubjects := make(map[string]bool)
	for _, subject := range graph.Subjects() {
		graphSubjects[subject.Value] = true
	}
	for _, e := range encoders {
		for iri, element := range e.origins {
			if !graphSubjects[iri] {
				continue
			}
			subject := rdf.IRI(iri)
			mapped.add(subject, element)
			if element.IsMembership {
				mapped.addMembership(subject, element.Membership)
			}
		}
	}
	return mapped
}

func hasSemanticMembership(membership metamodel.Membership) bool {
	return membership.Symbol != nil || membership.Node != nil || membership.Member != nil
}

func indexImportOwners(scope *symbols.Scope, owners map[*ast.Import]*symbols.Symbol, seen map[*symbols.Scope]bool) {
	if scope == nil || seen[scope] {
		return
	}
	seen[scope] = true
	for _, imp := range scope.Imports() {
		owners[imp] = scope.Owner()
	}
	for _, sym := range scope.AllMembers() {
		indexImportOwners(sym.Scope, owners, seen)
	}
}

func writeFullAPIJSON(graph *rdf.Graph, evaluator *metamodel.Evaluator, subjects *semanticSubjectMap) ([]byte, []serializationFailure, error) {
	settled, err := prepareFullAPIJSONGraph(graph, subjects)
	if err != nil {
		return nil, nil, err
	}
	return writeFullAPIJSONSettled(settled, evaluator, subjects)
}

func prepareFullAPIJSONGraph(graph *rdf.Graph, subjects *semanticSubjectMap) (*rdf.Graph, error) {
	wrapped, err := withRootNamespace(graph)
	if err != nil {
		return nil, err
	}
	settled, err := rdf.ReconcileCollections(wrapped)
	if err != nil {
		return nil, err
	}
	addRootNamespaceOrigins(settled, subjects)
	return settled, nil
}

func writeFullAPIJSONSettled(graph *rdf.Graph, evaluator *metamodel.Evaluator, subjects *semanticSubjectMap) ([]byte, []serializationFailure, error) {
	graphSubjects := graph.Subjects()
	var unmapped []string
	for _, subject := range graphSubjects {
		if hasSysMLType(graph.Objects(subject, rdf.RDFType)) {
			if _, ok := subjects.byIRI[subject.Value]; !ok {
				unmapped = append(unmapped, subject.Value)
			}
		}
	}
	if len(unmapped) > 0 {
		return nil, nil, fmt.Errorf("SysML subjects have no semantic origin: %s", strings.Join(unmapped, ", "))
	}
	elements := make([]apiJSONObject, 0, len(graphSubjects))
	for _, subject := range graphSubjects {
		element, err := apiJSONElement(graph, subject)
		if err != nil {
			return nil, nil, err
		}
		element, err = appendFullProperties(graph, subject, element, evaluator, subjects)
		if err != nil {
			return nil, nil, err
		}
		elements = append(elements, element)
	}
	var compact bytes.Buffer
	w := apiJSONWriter{buf: &compact, enc: json.NewEncoder(&compact)}
	compact.WriteByte('[')
	for i, element := range elements {
		if i > 0 {
			compact.WriteByte(',')
		}
		if err := w.object(element); err != nil {
			return nil, nil, err
		}
	}
	compact.WriteByte(']')
	var out bytes.Buffer
	out.Grow(compact.Len() * 2)
	if err := json.Indent(&out, compact.Bytes(), "", "  "); err != nil {
		return nil, nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), subjects.failures, nil
}

func addRootNamespaceOrigins(graph *rdf.Graph, subjects *semanticSubjectMap) {
	for _, subject := range graph.Subjects() {
		if !transparentRootSubject(graph, subject) {
			continue
		}
		subjects.add(subject, metamodel.Element{Aspect: "api-root"})
		for _, membership := range graph.Objects(subject, rdf.SysML+pOwnedRelationship) {
			if graph.Type(membership) == rdf.SysML+mOwningMembership {
				handle := metamodel.Membership{Aspect: "api-root"}
				subjects.add(membership, metamodel.MembershipElement(handle))
			}
		}
	}
}

func hasSysMLType(types []rdf.Term) bool {
	for _, typ := range types {
		if strings.HasPrefix(typ.Value, rdf.SysML) {
			return true
		}
	}
	return false
}

func appendFullProperties(graph *rdf.Graph, subject rdf.Term, element apiJSONObject, evaluator *metamodel.Evaluator, subjects *semanticSubjectMap) (apiJSONObject, error) {
	metaclass := graph.Type(subject)
	if !strings.HasPrefix(metaclass, rdf.SysML) {
		return element, nil
	}
	metaclass = strings.TrimPrefix(metaclass, rdf.SysML)
	elementHandle := subjects.byIRI[subject.Value]
	keyed := make(map[string]bool, len(element))
	for _, member := range element {
		keyed[member.key] = true
	}
	properties := ontologyProperties(metaclass)
	if len(properties) == 0 {
		if libraryProperty, ok := ontology.PropertyOf("Element", "isLibraryElement"); ok {
			properties = []ontology.Property{libraryProperty}
		}
	}
	for _, property := range properties {
		if keyed[property.Name] {
			continue
		}
		if id, ok := membershipElementID(graph, subject, property); ok {
			element = append(element, apiJSONMember{key: property.Name, value: id})
			continue
		}
		effective := redefinedProperty(metaclass, property)
		if effective.QualifiedName() != property.QualifiedName() && keyed[effective.Name] {
			if value, ok := apiJSONMemberValue(element, effective.Name); ok {
				if value, ok := redefinedAPIJSONValue(value, effective, property); ok {
					element = append(element, apiJSONMember{key: property.Name, value: value})
				}
			}
			continue
		}
		value, ok := evaluator.Property(elementHandle, effective.DefiningClass, effective.Name)
		var failures []serializationFailure
		element, failures = appendFullProperty(element, subject, property, effective, value, ok, subjects)
		subjects.failures = append(subjects.failures, failures...)
	}
	if _, ok := apiJSONMemberValue(element, "isLibraryElement"); !ok {
		element = append(element, apiJSONMember{
			key:   "isLibraryElement",
			value: subjects.isLibraryHandle(elementHandle),
		})
	}
	return element, nil
}

func apiJSONMemberValue(element apiJSONObject, key string) (any, bool) {
	for _, member := range element {
		if member.key == key {
			return member.value, true
		}
	}
	return nil, false
}

func redefinedAPIJSONValue(value any, source, target ontology.Property) (any, bool) {
	if source.Many == target.Many {
		return value, true
	}
	if target.Many {
		if value == nil {
			return []any{}, true
		}
		return []any{value}, true
	}
	values, ok := value.([]any)
	if !ok {
		return nil, false
	}
	switch len(values) {
	case 0:
		return nil, true
	case 1:
		return values[0], true
	default:
		return nil, false
	}
}

func membershipElementID(graph *rdf.Graph, subject rdf.Term, property ontology.Property) (string, bool) {
	var memberPredicate string
	switch property.QualifiedName() {
	case "Membership::memberElementId":
		memberPredicate = pMemberElement
	case "OwningMembership::ownedMemberElementId":
		memberPredicate = pOwnedMemberElement
	default:
		return "", false
	}
	member, ok := graph.Object(subject, rdf.SysML+memberPredicate)
	if !ok || member.Kind != rdf.TermIRI {
		return "", false
	}
	id, ok := graph.Object(member, rdf.SysML+pElementID)
	if !ok || id.Kind != rdf.TermLiteral {
		return "", false
	}
	return id.Value, true
}

func appendFullProperty(element apiJSONObject, subject rdf.Term, property, effective ontology.Property, value metamodel.Value, computed bool, subjects *semanticSubjectMap) (apiJSONObject, []serializationFailure) {
	if computed {
		encoded, serializable, failures := semanticValueJSON(subject, value, property, subjects)
		for i := range failures {
			failures[i].Property = property.QualifiedName()
		}
		if serializable {
			element = append(element, apiJSONMember{key: property.Name, value: encoded})
		}
		return element, failures
	}
	if effective.Derived {
		return element, nil
	}
	element = append(element, apiJSONMember{key: property.Name, value: ecoreFallback(effective)})
	return element, nil
}

func ontologyProperties(metaclass string) []ontology.Property {
	return apiOntology.propertiesByClass[metaclass]
}

func redefinedProperty(metaclass string, property ontology.Property) ontology.Property {
	if redefined, ok := apiOntology.redefinitionsByClass[metaclass][property.QualifiedName()]; ok {
		return redefined
	}
	return property
}

func semanticValueJSON(subject rdf.Term, value metamodel.Value, property ontology.Property, subjects *semanticSubjectMap) (any, bool, []serializationFailure) {
	if value.Kind == metamodel.SequenceValue {
		itemProperty := property
		itemProperty.Many = false
		values := make([]any, 0, len(value.Values))
		var failures []serializationFailure
		complete := true
		for _, item := range value.Values {
			encoded, ok, itemFailures := semanticValueJSON(subject, item, itemProperty, subjects)
			failures = append(failures, itemFailures...)
			if !ok {
				complete = false
				continue
			}
			values = append(values, encoded)
		}
		if !complete || len(failures) > 0 {
			return nil, false, failures
		}
		if property.Many {
			return values, true, nil
		}
		switch len(values) {
		case 0:
			return nil, true, nil
		case 1:
			return values[0], true, nil
		default:
			return nil, false, []serializationFailure{{
				Reason: "multiple values for single-valued property",
				Source: subject.Value,
				Target: property.QualifiedName(),
			}}
		}
	}
	switch value.Kind {
	case metamodel.NullValue:
		if property.Many {
			return []any{}, true, nil
		}
		return nil, true, nil
	case metamodel.StringValue:
		return semanticScalarJSON(value.String, property), true, nil
	case metamodel.BooleanValue:
		return semanticScalarJSON(value.Boolean, property), true, nil
	case metamodel.IntegerValue:
		return semanticScalarJSON(json.Number(strconv.FormatInt(value.Integer, 10)), property), true, nil
	case metamodel.RealValue:
		return semanticScalarJSON(json.Number(strconv.FormatFloat(value.Real, 'g', -1, 64)), property), true, nil
	case metamodel.EnumValue:
		return semanticScalarJSON(value.Enum, property), true, nil
	case metamodel.ElementValue:
		target, ok := subjects.byElement[value.Element.Key()]
		if !ok {
			target, ok = subjects.libraryElementTerm(value.Element)
		}
		if !ok {
			return nil, false, []serializationFailure{{
				Reason: subjects.missingElementReason(value.Element),
				Source: subject.Value,
				Target: subjects.elementLabel(value.Element),
			}}
		}
		return semanticScalarJSON(apiJSONReference{ID: rdf.ReferenceID(subject, target)}, property), true, nil
	case metamodel.MembershipValue:
		target, ok := subjects.byMembership[value.Membership.Key()]
		if !ok {
			target, ok = subjects.libraryMembershipTerm(value.Membership)
		}
		if !ok {
			return nil, false, []serializationFailure{{
				Reason: subjects.missingMembershipReason(value.Membership),
				Source: subject.Value,
				Target: subjects.membershipLabel(value.Membership),
			}}
		}
		return semanticScalarJSON(apiJSONReference{ID: rdf.ReferenceID(subject, target)}, property), true, nil
	default:
		return nil, false, nil
	}
}

func semanticScalarJSON(value any, property ontology.Property) any {
	if property.Many {
		return []any{value}
	}
	return value
}

func (s *semanticSubjectMap) missingElementReason(element metamodel.Element) string {
	if element.IsMembership {
		return s.missingMembershipReason(element.Membership)
	}
	if s.isLibrary(element.Symbol) {
		libraryElement, ok := s.libraryElement(element.Symbol)
		if !ok || libraryElement.ID == "" {
			return fmt.Sprintf("library symbol %q is absent from identity.LibraryCatalog", s.elementLabel(element))
		}
		return "handle-key mismatch"
	}
	if element.Symbol == nil && s.isLibrary(element.Container) {
		return fmt.Sprintf("AST-only library node %T has no catalog identity", element.Node)
	}
	if element.Symbol != nil {
		switch element.Symbol.Naming {
		case symbols.NamedByRedefinition:
			return fmt.Sprintf("redefining member %q has no graph subject", s.elementLabel(element))
		case symbols.NamedByReference:
			return fmt.Sprintf("reference-named member %q has no graph subject", s.elementLabel(element))
		default:
			return fmt.Sprintf("model symbol %q (%s) has no graph subject", s.elementLabel(element), element.Symbol.Kind)
		}
	}
	if element.Node != nil {
		return fmt.Sprintf("AST-only model node %T has no minted subject", element.Node)
	}
	if element.Container != nil {
		return fmt.Sprintf("derived handle %q has no graph subject", element.Aspect)
	}
	return "handle-key mismatch"
}

func (s *semanticSubjectMap) missingMembershipReason(membership metamodel.Membership) string {
	for _, symbol := range []*symbols.Symbol{membership.Symbol, membership.Member, membership.Owner} {
		if !s.isLibrary(symbol) {
			continue
		}
		element, ok := s.libraryElement(symbol)
		if !ok {
			return fmt.Sprintf("library symbol %q is absent from identity.LibraryCatalog", s.elementLabel(metamodel.ElementOf(symbol)))
		}
		if element.OwningMembershipID == "" {
			return fmt.Sprintf("library member %q has no owning membership ID", s.elementLabel(metamodel.ElementOf(symbol)))
		}
		return "handle-key mismatch"
	}
	for _, symbol := range []*symbols.Symbol{membership.Symbol, membership.Member} {
		if symbol == nil {
			continue
		}
		switch symbol.Naming {
		case symbols.NamedByRedefinition:
			return fmt.Sprintf("redefining member %q has no membership subject", s.elementLabel(metamodel.ElementOf(symbol)))
		case symbols.NamedByReference:
			return fmt.Sprintf("reference-named member %q has no membership subject", s.elementLabel(metamodel.ElementOf(symbol)))
		default:
			return fmt.Sprintf("model member %q (%s) has no membership subject", s.elementLabel(metamodel.ElementOf(symbol)), symbol.Kind)
		}
	}
	if membership.Node != nil {
		return fmt.Sprintf("AST-only membership node %T has no minted subject", membership.Node)
	}
	if membership.Owner != nil {
		return fmt.Sprintf("membership owned by %q has no graph subject", s.elementLabel(metamodel.ElementOf(membership.Owner)))
	}
	return "handle-key mismatch"
}

func (s *semanticSubjectMap) isLibrary(symbol *symbols.Symbol) bool {
	if symbol == nil || s.resolver == nil || s.resolver.Index() == nil {
		return false
	}
	index := s.resolver.Index()
	return index.Library(symbol) || index.IsLibraryDocument(symbol.DocName)
}

func (s *semanticSubjectMap) isLibraryHandle(element metamodel.Element) bool {
	if element.IsMembership {
		for _, symbol := range []*symbols.Symbol{
			element.Membership.Symbol, element.Membership.Member, element.Membership.Owner,
		} {
			if s.isLibrary(symbol) {
				return true
			}
		}
		return false
	}
	return s.isLibrary(element.Symbol) || s.isLibrary(element.Container)
}

func (s *semanticSubjectMap) libraryElement(symbol *symbols.Symbol) (*identity.LibraryElement, bool) {
	if !s.isLibrary(symbol) {
		return nil, false
	}
	if s.catalog == nil {
		s.catalog = identity.LibraryCatalog(s.resolver.Index())
	}
	return s.catalog.ElementForSymbol(symbol)
}

func (s *semanticSubjectMap) libraryElementTerm(element metamodel.Element) (rdf.Term, bool) {
	libraryElement, ok := s.libraryElement(element.Symbol)
	if !ok || libraryElement.ID == "" {
		return rdf.Term{}, false
	}
	return rdf.ElementIRIForID(libraryElement.ID), true
}

func (s *semanticSubjectMap) libraryMembershipTerm(membership metamodel.Membership) (rdf.Term, bool) {
	symbol := membership.Member
	if symbol == nil {
		symbol = membership.Symbol
	}
	libraryElement, ok := s.libraryElement(symbol)
	if !ok || libraryElement.OwningMembershipID == "" {
		return rdf.Term{}, false
	}
	return rdf.ElementIRIForID(libraryElement.OwningMembershipID), true
}

func (s *semanticSubjectMap) elementLabel(element metamodel.Element) string {
	if element.Symbol != nil && s.resolver != nil && s.resolver.Index() != nil {
		return s.resolver.Index().GetFQN(element.Symbol)
	}
	if element.Node != nil {
		if relationship, ok := element.Node.(*ast.Relationship); ok {
			return fmt.Sprintf("*ast.Relationship(kind=%v,target=%T)", relationship.Kind, relationship.Target)
		}
		return fmt.Sprintf("%T", element.Node)
	}
	if element.Aspect != "" {
		return element.Aspect
	}
	return "<unnamed element>"
}

func (s *semanticSubjectMap) membershipLabel(membership metamodel.Membership) string {
	if membership.Member != nil {
		return s.elementLabel(metamodel.ElementOf(membership.Member))
	}
	if membership.Symbol != nil {
		return s.elementLabel(metamodel.ElementOf(membership.Symbol))
	}
	if membership.Node != nil {
		return fmt.Sprintf("%T", membership.Node)
	}
	if membership.Aspect != "" {
		return membership.Aspect
	}
	return "<unnamed membership>"
}

func ecoreFallback(property ontology.Property) any {
	if property.HasDefault {
		switch ontology.LocalName(property.Range) {
		case "boolean":
			if value, err := strconv.ParseBool(property.Default); err == nil {
				return value
			}
		case "byte", "short", "int", "integer", "long", "BigInteger":
			if _, err := strconv.ParseInt(property.Default, 10, 64); err == nil {
				return json.Number(property.Default)
			}
		case "float", "double", "decimal", "BigDecimal":
			if _, err := strconv.ParseFloat(property.Default, 64); err == nil {
				return json.Number(property.Default)
			}
		}
		return property.Default
	}
	if property.Many {
		return []any{}
	}
	return nil
}
