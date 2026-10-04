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
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
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
	if form != APIJSONCompact && form != APIJSONFull {
		return nil, fmt.Errorf("unknown API JSON form %d", form)
	}
	graph, res, encoders, err := modelToRDF(documents, ids)
	if err != nil {
		return nil, err
	}
	if form == APIJSONCompact {
		return WriteAPIJSON(graph)
	}
	if len(encoders) == 0 {
		return WriteAPIJSON(graph)
	}
	evaluator := metamodel.New(res, encoders[0].ids.model)
	subjects := buildSemanticSubjects(graph, res, encoders, encoders[0].ids.model)
	return writeFullAPIJSON(graph, evaluator, subjects)
}

type semanticSubjectMap struct {
	byIRI        map[string]metamodel.Element
	byElement    map[metamodel.Element]rdf.Term
	byMembership map[metamodel.Membership]rdf.Term
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
		byElement:    make(map[metamodel.Element]rdf.Term),
		byMembership: make(map[metamodel.Membership]rdf.Term),
	}
}

func (s *semanticSubjectMap) add(subject rdf.Term, element metamodel.Element) {
	if subject.Value == "" {
		return
	}
	s.byIRI[subject.Value] = element
	s.byElement[element] = subject
}

func buildSemanticSubjects(graph *rdf.Graph, res *resolve.Resolver, encoders []*encoder, model *semantics.Model) *semanticSubjectMap {
	mapped := newSemanticSubjectMap()
	for _, libraryElement := range identity.LibraryCatalog(res.Index()).Elements() {
		element := metamodel.ElementOf(libraryElement.Symbol)
		mapped.add(rdf.ElementIRIForID(libraryElement.ID), element)
		membership := metamodel.MembershipElement(element.Membership)
		if hasSemanticMembership(element.Membership) {
			membershipTerm := rdf.ElementIRIForID(libraryElement.OwningMembershipID)
			mapped.byMembership[element.Membership] = membershipTerm
			mapped.byElement[membership] = membershipTerm
		}
	}
	graphSubjects := make(map[string]bool)
	for _, subject := range graph.Subjects() {
		graphSubjects[subject.Value] = true
	}
	importOwners := make(map[*ast.Import]*symbols.Symbol)
	for _, e := range encoders {
		indexImportOwners(res.Index().DocumentRoot(e.file.Name()), importOwners, make(map[*symbols.Scope]bool))
	}
	for _, e := range encoders {
		for node, fqn := range e.fqn {
			subject := e.ids.subjectForNode(node, fqn)
			sym := e.ids.declSym[node]
			if sym == nil {
				sym = symbolForNode(res, node, fqn)
			}
			element := metamodel.NodeOf(node)
			if imp, ok := node.(*ast.Import); ok {
				element = metamodel.Element{Node: imp, Container: importOwners[imp]}
			} else if sym != nil {
				element = metamodel.ElementOf(sym)
			}
			if graphSubjects[subject.Value] {
				mapped.add(subject, element)
			}
			if sym != nil {
				addSemanticMembership(mapped, e, node, subject, sym)
			}
		}
		for _, ref := range e.libraryRefs {
			sym := e.ids.declSym[ref.node]
			if sym == nil {
				sym = symbolForNode(res, ref.node, ref.fqn)
			}
			if sym == nil {
				continue
			}
			element := metamodel.ElementOf(sym)
			mapped.add(ref.subject, element)
			addSemanticMembership(mapped, e, ref.node, ref.subject, sym)
			if ref.membership.Value != "" && hasSemanticMembership(element.Membership) {
				membership := metamodel.MembershipElement(element.Membership)
				mapped.byMembership[element.Membership] = ref.membership
				mapped.byElement[membership] = ref.membership
			}
		}
		if model != nil {
			for _, sym := range e.ids.declSym {
				for _, site := range model.AnnotationSitesOf(sym) {
					if site.Node == nil {
						continue
					}
					fqn, ok := e.fqn[site.Node]
					if !ok {
						continue
					}
					subject := e.ids.subjectForNode(site.Node, fqn)
					if graphSubjects[subject.Value] {
						mapped.add(subject, metamodel.Element{
							Node: site.Node, Container: sym, Aspect: "annotation",
						})
					}
				}
			}
		}
	}
	return mapped
}

func addSemanticMembership(mapped *semanticSubjectMap, e *encoder, node ast.Node, subject rdf.Term, sym *symbols.Symbol) {
	element := metamodel.ElementOf(sym)
	if !hasSemanticMembership(element.Membership) {
		return
	}
	if sym.Kind == symbols.SymbolAlias {
		mapped.byMembership[element.Membership] = subject
		mapped.byElement[metamodel.MembershipElement(element.Membership)] = subject
		return
	}
	membership := e.ids.owningMembershipOf(node, subject)
	mapped.byMembership[element.Membership] = membership
	mapped.byElement[metamodel.MembershipElement(element.Membership)] = membership
}

func hasSemanticMembership(membership metamodel.Membership) bool {
	return membership.Symbol != nil || membership.Node != nil || membership.Member != nil
}

func symbolForNode(res *resolve.Resolver, node ast.Node, fqn string) *symbols.Symbol {
	if res == nil || res.Index() == nil {
		return nil
	}
	for _, sym := range res.Index().LookupQualified(fqn) {
		if sym != nil && sym.Decl == node {
			return sym
		}
	}
	if sym := res.Index().Declaring(fqn); sym != nil && sym.Decl == node {
		return sym
	}
	return nil
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

func writeFullAPIJSON(graph *rdf.Graph, evaluator *metamodel.Evaluator, subjects *semanticSubjectMap) ([]byte, error) {
	wrapped, err := withRootNamespace(graph)
	if err != nil {
		return nil, err
	}
	settled, err := rdf.ReconcileCollections(wrapped)
	if err != nil {
		return nil, err
	}
	elements := make([]apiJSONObject, 0, len(settled.Subjects()))
	for _, subject := range settled.Subjects() {
		element, err := apiJSONElement(settled, subject)
		if err != nil {
			return nil, err
		}
		element, err = appendFullProperties(settled, subject, element, evaluator, subjects)
		if err != nil {
			return nil, err
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
			return nil, err
		}
	}
	compact.WriteByte(']')
	var out bytes.Buffer
	out.Grow(compact.Len() * 2)
	if err := json.Indent(&out, compact.Bytes(), "", "  "); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

func appendFullProperties(graph *rdf.Graph, subject rdf.Term, element apiJSONObject, evaluator *metamodel.Evaluator, subjects *semanticSubjectMap) (apiJSONObject, error) {
	types := graph.Objects(subject, rdf.RDFType)
	if len(types) != 1 || !strings.HasPrefix(types[0].Value, rdf.SysML) {
		return element, nil
	}
	metaclass := strings.TrimPrefix(types[0].Value, rdf.SysML)
	elementHandle := subjects.byIRI[subject.Value]
	keyed := make(map[string]bool, len(element))
	for _, member := range element {
		keyed[member.key] = true
	}
	for _, property := range ontologyProperties(metaclass) {
		if keyed[property.Name] {
			continue
		}
		effective := redefinedProperty(metaclass, property)
		value, ok := evaluator.Property(elementHandle, effective.DefiningClass, effective.Name)
		if ok {
			encoded, serializable := semanticValueJSON(subject, value, effective, subjects)
			if serializable {
				element = append(element, apiJSONMember{key: property.Name, value: encoded})
				continue
			}
		}
		if effective.Derived {
			continue
		}
		element = append(element, apiJSONMember{
			key: property.Name, value: ecoreFallback(effective),
		})
	}
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

func semanticValueJSON(subject rdf.Term, value metamodel.Value, property ontology.Property, subjects *semanticSubjectMap) (any, bool) {
	switch value.Kind {
	case metamodel.NullValue:
		return nil, true
	case metamodel.StringValue:
		return value.String, true
	case metamodel.BooleanValue:
		return value.Boolean, true
	case metamodel.IntegerValue:
		return json.Number(strconv.FormatInt(value.Integer, 10)), true
	case metamodel.RealValue:
		return json.Number(strconv.FormatFloat(value.Real, 'g', -1, 64)), true
	case metamodel.EnumValue:
		return value.Enum, true
	case metamodel.ElementValue:
		target, ok := subjects.byElement[value.Element]
		if !ok {
			return nil, false
		}
		return apiJSONReference{ID: rdf.ReferenceID(subject, target)}, true
	case metamodel.MembershipValue:
		target, ok := subjects.byMembership[value.Membership]
		if !ok {
			return nil, false
		}
		return apiJSONReference{ID: rdf.ReferenceID(subject, target)}, true
	case metamodel.SequenceValue:
		values := make([]any, 0, len(value.Values))
		for _, item := range value.Values {
			encoded, ok := semanticValueJSON(subject, item, property, subjects)
			if !ok {
				return nil, false
			}
			values = append(values, encoded)
		}
		if property.Many {
			return values, true
		}
		switch len(values) {
		case 0:
			return nil, true
		case 1:
			return values[0], true
		default:
			return values, true
		}
	default:
		return nil, false
	}
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
