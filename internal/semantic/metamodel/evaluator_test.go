package metamodel

import (
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
)

var testMetaclassParents = map[string]string{
	"Package":                         "Namespace",
	"LibraryPackage":                  "Package",
	"Namespace":                       "Element",
	"Definition":                      "Classifier",
	"Classifier":                      "Type",
	"Class":                           "Classifier",
	"Structure":                       "Class",
	"Association":                     "Classifier",
	"ConstraintDefinition":            "Definition",
	"RequirementDefinition":           "ConstraintDefinition",
	"Interaction":                     "Behavior",
	"Predicate":                       "Function",
	"Behavior":                        "Type",
	"Function":                        "Behavior",
	"Type":                            "Namespace",
	"Feature":                         "Type",
	"Expression":                      "Feature",
	"InstantiationExpression":         "Expression",
	"InvocationExpression":            "InstantiationExpression",
	"OperatorExpression":              "InvocationExpression",
	"FeatureReferenceExpression":      "Expression",
	"PayloadFeature":                  "Feature",
	"FlowEnd":                         "Feature",
	"Usage":                           "Feature",
	"OccurrenceUsage":                 "Usage",
	"ConstraintUsage":                 "OccurrenceUsage",
	"TestTransitionUsage":             "TransitionUsage",
	"PartDefinition":                  "Definition",
	"PortDefinition":                  "OccurrenceDefinition",
	"IndividualDefinition":            "OccurrenceDefinition",
	"AttributeDefinition":             "Definition",
	"ItemDefinition":                  "Definition",
	"OccurrenceDefinition":            "Definition",
	"ActionDefinition":                "Behavior",
	"FlowUsage":                       "ActionUsage",
	"PartUsage":                       "Usage",
	"AttributeUsage":                  "Usage",
	"ReferenceUsage":                  "Usage",
	"ItemUsage":                       "Usage",
	"ActionUsage":                     "Usage",
	"PerformActionUsage":              "ActionUsage",
	"ExhibitStateUsage":               "PerformActionUsage",
	"AcceptActionUsage":               "ActionUsage",
	"StateUsage":                      "ActionUsage",
	"TransitionUsage":                 "ActionUsage",
	"SuccessionAsUsage":               "Succession",
	"FeatureMembership":               "OwningMembership",
	"ActorMembership":                 "ParameterMembership",
	"ParameterMembership":             "FeatureMembership",
	"SubjectMembership":               "ParameterMembership",
	"StakeholderMembership":           "ParameterMembership",
	"ObjectiveMembership":             "FeatureMembership",
	"ReturnParameterMembership":       "FeatureMembership",
	"FramedConcernMembership":         "RequirementConstraintMembership",
	"RequirementConstraintMembership": "FeatureMembership",
	"StateSubactionMembership":        "FeatureMembership",
	"TransitionFeatureMembership":     "FeatureMembership",
	"ElementFilterMembership":         "OwningMembership",
	"EndFeatureMembership":            "FeatureMembership",
	"Membership":                      "Relationship",
	"OwningMembership":                "Membership",
	"VariantMembership":               "OwningMembership",
	"Relationship":                    "Element",
	"Annotation":                      "Relationship",
	"Documentation":                   "Relationship",
	"Import":                          "Relationship",
	"FeatureValue":                    "Relationship",
	"Subclassification":               "Specialization",
	"Dependency":                      "Relationship",
	"Connector":                       "Relationship",
	"Flow":                            "Connector",
	"Conjugation":                     "Relationship",
	"Differencing":                    "Relationship",
	"FeatureChaining":                 "Relationship",
	"CrossSubsetting":                 "Subsetting",
	"FeatureInverting":                "Subsetting",
	"FeatureTyping":                   "Specialization",
	"ReferenceSubsetting":             "Subsetting",
	"Redefinition":                    "Subsetting",
	"Subsetting":                      "Specialization",
	"MetadataUsage":                   "Usage",
	"Specialization":                  "Relationship",
	"TypeFeaturing":                   "Relationship",
	"Disjoining":                      "Relationship",
	"Intersecting":                    "Relationship",
	"Unioning":                        "Relationship",
}

func newTestEvaluator(t *testing.T, name, text string) (*Evaluator, *symbols.Scope) {
	return newTestEvaluatorWithIndex(t, symbols.NewIndex(), name, text)
}

func newTestEvaluatorWithIndex(t *testing.T, index *symbols.Index, name, text string) (*Evaluator, *symbols.Scope) {
	t.Helper()
	p := parser.New(source.New(name, []byte(text)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	index.AddDocumentWithKind(name, root, source.KindOf(name))
	resolver := resolve.New(index)
	model := semantics.NewModel(resolver)
	resolver.SetModel(model)
	resolver.ResolveDocument(name, root)
	evaluator := New(resolver, model)
	var elements []*symbols.Symbol
	var visit func(*symbols.Scope)
	visit = func(scope *symbols.Scope) {
		for _, element := range scope.Members() {
			elements = append(elements, element)
			if element.Scope != nil {
				visit(element.Scope)
			}
		}
	}
	visit(index.DocumentRoot(name))
	structure := testNamespaceStructure(evaluator, elements...)
	for _, element := range elements {
		if class, ok := testMetaclass(ElementOf(element)); ok {
			structure.metaclasses[ElementOf(element).Key()] = class
		}
	}
	evaluator.options.Structure = structure
	return evaluator, index.DocumentRoot(name)
}

type testStructure struct {
	metaclass             func(Element) (string, bool)
	specializes           func(string, string) bool
	metaclasses           map[ElementKey]string
	ownedRelationships    map[ElementKey][]Element
	owningRelationships   map[ElementKey]testOptionalElement
	ownedRelatedElements  map[ElementKey][]Element
	owningRelatedElements map[ElementKey]testOptionalElement
	relatedElements       map[testRelatedKey][]Element
	attributes            map[testRelatedKey]Value
	unknownOwnedRelations map[ElementKey]bool
	unresolvedRelated     map[testRelatedKey]bool
	statedRelated         map[testRelatedKey]bool
}

type testOptionalElement struct {
	element Element
	present bool
}

type testRelatedKey struct {
	element  ElementKey
	property string
}

func (s testStructure) Metaclass(element Element) (string, bool) {
	if metaclass, ok := s.metaclasses[element.Key()]; ok {
		return metaclass, true
	}
	if s.metaclass == nil {
		return "", false
	}
	return s.metaclass(element)
}

func (s testStructure) Specializes(sub, super string) bool {
	if s.specializes != nil {
		return s.specializes(sub, super)
	}
	return testSpecializes(sub, super)
}

func (s testStructure) OwnedRelationships(element Element) ([]Element, bool) {
	if s.unknownOwnedRelations[element.Key()] {
		return nil, false
	}
	value, ok := s.ownedRelationships[element.Key()]
	if !ok {
		_, known := s.Metaclass(element)
		return nil, known
	}
	return value, true
}

func (s testStructure) OwningRelationship(element Element) (Element, bool, bool) {
	value, ok := s.owningRelationships[element.Key()]
	if !ok {
		_, known := s.Metaclass(element)
		return Element{}, false, known
	}
	return value.element, value.present, true
}

func (s testStructure) OwnedRelatedElements(element Element) ([]Element, bool) {
	value, ok := s.ownedRelatedElements[element.Key()]
	if !ok {
		_, known := s.Metaclass(element)
		return nil, known
	}
	return value, true
}

func (s testStructure) OwningRelatedElement(element Element) (Element, bool, bool) {
	value, ok := s.owningRelatedElements[element.Key()]
	if !ok {
		_, known := s.Metaclass(element)
		return Element{}, false, known
	}
	return value.element, value.present, true
}

func (s testStructure) RelatedElements(element Element, property string) ([]Element, bool) {
	value, ok := s.relatedElements[testRelatedKey{element: element.Key(), property: property}]
	if !ok {
		_, known := s.Metaclass(element)
		return nil, known
	}
	return value, true
}

func (s testStructure) StatedRelatedElements(element Element, property string) ([]Element, bool) {
	return s.RelatedElements(element, property)
}

func (s testStructure) HasUnresolvedRelatedElement(element Element, property string) bool {
	return s.unresolvedRelated[testRelatedKey{element: element.Key(), property: property}]
}

func (s testStructure) HasStatedRelatedElement(element Element, property string) bool {
	return s.statedRelated[testRelatedKey{element: element.Key(), property: property}]
}

func (s testStructure) Attribute(element Element, property string) (Value, bool) {
	value, ok := s.attributes[testRelatedKey{element: element.Key(), property: property}]
	if ok {
		return value, true
	}
	metaclass, ok := s.Metaclass(element)
	if !ok {
		return Value{}, false
	}
	definition, ok := ontology.PropertyOf(metaclass, property)
	if !ok || definition.Derived || definition.Kind != ontology.DatatypeProperty {
		return Value{}, false
	}
	if !definition.HasDefault {
		if definition.Lower != 0 {
			return Value{}, false
		}
		if definition.Many {
			return sequence(), true
		}
		return nullValue(), true
	}
	switch definition.Range {
	case "http://www.w3.org/2001/XMLSchema#boolean":
		return booleanValue(definition.Default == "true"), true
	default:
		return stringValue(definition.Default), true
	}
}

func (s testStructure) StatedAttribute(element Element, property string) (Value, bool) {
	value, ok := s.attributes[testRelatedKey{element: element.Key(), property: property}]
	return value, ok
}

func testNamespaceStructure(evaluator *Evaluator, namespaces ...*symbols.Symbol) testStructure {
	structure := testStructure{
		metaclass:             testMetaclass,
		specializes:           testSpecializes,
		metaclasses:           make(map[ElementKey]string),
		ownedRelationships:    make(map[ElementKey][]Element),
		owningRelationships:   make(map[ElementKey]testOptionalElement),
		ownedRelatedElements:  make(map[ElementKey][]Element),
		owningRelatedElements: make(map[ElementKey]testOptionalElement),
		relatedElements:       make(map[testRelatedKey][]Element),
		attributes:            make(map[testRelatedKey]Value),
	}
	for _, namespace := range namespaces {
		if namespace == nil {
			continue
		}
		owner := ElementOf(namespace)
		if class, ok := testMetaclass(owner); ok {
			structure.metaclasses[owner.Key()] = class
			addTestStructureAttributes(evaluator, &structure, owner, namespace)
		}
		if _, ok := structure.owningRelationships[owner.Key()]; !ok {
			structure.owningRelationships[owner.Key()] = testOptionalElement{present: false}
		}
		relationships := make([]Element, 0)
		if namespace.Scope != nil {
			seenMembers := make(map[*symbols.Symbol]bool)
			addMember := func(member *symbols.Symbol) {
				if member == nil || seenMembers[member] {
					return
				}
				seenMembers[member] = true
				memberElement := ElementOf(member)
				if class, ok := testMetaclass(memberElement); ok {
					structure.metaclasses[memberElement.Key()] = class
					addTestStructureAttributes(evaluator, &structure, memberElement, member)
				}
				membership := memberElement.Membership
				if _, ok := member.Decl.(*ast.RequireMember); ok {
					membership.kind = "RequirementConstraintMembership"
					memberElement.Membership = membership
				}
				if !hasMembership(membership) {
					return
				}
				handle := MembershipElement(membership)
				relationshipClass := membership.kind
				if relationshipClass == "Alias" {
					relationshipClass = "Membership"
				}
				structure.metaclasses[handle.Key()] = relationshipClass
				addTestStructureAttributes(evaluator, &structure, handle, member)
				relationships = append(relationships, handle)
				ownedMemberElement := memberElement
				if alias, ok := member.Decl.(*ast.Alias); ok {
					if owner := member.Owner(); owner != nil && owner.Scope != nil {
						if target, resolved := evaluator.resolver.ResolveTarget(owner.Scope, alias.For); resolved && target != nil {
							ownedMemberElement = ElementOf(target)
						}
					}
				}
				structure.relatedElements[testRelatedKey{element: handle.Key(), property: "memberElement"}] =
					[]Element{ownedMemberElement}
				structure.relatedElements[testRelatedKey{element: handle.Key(), property: "membershipOwningNamespace"}] =
					[]Element{owner}
				structure.owningRelationships[ElementOf(member).Key()] =
					testOptionalElement{element: handle, present: true}
				structure.ownedRelatedElements[handle.Key()] = []Element{ownedMemberElement}
				structure.owningRelatedElements[handle.Key()] =
					testOptionalElement{element: owner, present: true}
			}
			for _, member := range namespace.Scope.AllMembers() {
				addMember(member)
			}
			namespace.Scope.ForEachAnonymousMember(func(member *symbols.Symbol) bool {
				addMember(member)
				return true
			})
		}
		if namespace.Scope != nil {
			for _, imp := range namespace.Scope.Imports() {
				handle := Element{Node: imp, Container: namespace}
				structure.metaclasses[handle.Key()] = "Import"
				addTestStructureAttributes(evaluator, &structure, handle, nil)
				relationships = append(relationships, handle)
				structure.ownedRelatedElements[handle.Key()] = []Element{}
				structure.owningRelatedElements[handle.Key()] =
					testOptionalElement{element: owner, present: true}
			}
		}
		structure.ownedRelationships[owner.Key()] = relationships
		addTestSemanticRelationships(evaluator, &structure, namespace)
		sort.SliceStable(structure.ownedRelationships[owner.Key()], func(i, j int) bool {
			left := testStructureElementNode(structure.ownedRelationships[owner.Key()][i])
			right := testStructureElementNode(structure.ownedRelationships[owner.Key()][j])
			if left == nil {
				return false
			}
			if right == nil {
				return true
			}
			return left.Span().Offset < right.Span().Offset
		})
	}
	return structure
}

func testStructureElementNode(element Element) ast.Node {
	if element.IsMembership {
		return element.Membership.Node
	}
	return element.Node
}

func addTestSemanticRelationships(evaluator *Evaluator, structure *testStructure, owner *symbols.Symbol) {
	ownerElement := ElementOf(owner)
	for _, relationship := range semantics.RelationshipsOf(owner) {
		if relationship == nil {
			continue
		}
		handle := Element{Node: relationship, Container: owner}
		class, sourceProperty, targetProperty := testRelationshipMetaclass(relationship.Kind)
		structure.metaclasses[handle.Key()] = class
		structure.ownedRelationships[ownerElement.Key()] = append(
			structure.ownedRelationships[ownerElement.Key()], handle,
		)
		structure.owningRelatedElements[handle.Key()] =
			testOptionalElement{element: ownerElement, present: true}
		structure.ownedRelatedElements[handle.Key()] = []Element{ownerElement}
		addTestStructureAttributes(evaluator, structure, handle, nil)
		structure.relatedElements[testRelatedKey{element: handle.Key(), property: sourceProperty}] =
			[]Element{ownerElement}
		target := evaluator.model.RelationshipTarget(owner, relationship)
		if target == nil {
			structure.relatedElements[testRelatedKey{element: handle.Key(), property: targetProperty}] = []Element{}
			continue
		}
		structure.relatedElements[testRelatedKey{element: handle.Key(), property: targetProperty}] =
			[]Element{ElementOf(target)}
	}
}

func addTestStructureAttributes(
	evaluator *Evaluator, structure *testStructure, element Element, symbol *symbols.Symbol,
) {
	if symbol == nil && element.IsMembership {
		symbol = element.Membership.Member
	}
	if symbol == nil {
		return
	}
	metaclass, ok := structure.Metaclass(element)
	if !ok {
		return
	}
	if alias, ok := symbol.Decl.(*ast.Alias); ok {
		if alias.Ident.Name != "" {
			structure.attributes[testRelatedKey{element: element.Key(), property: "memberName"}] =
				stringValue(alias.Ident.Name)
		}
		if alias.Ident.ShortName != "" {
			structure.attributes[testRelatedKey{element: element.Key(), property: "memberShortName"}] =
				stringValue(alias.Ident.ShortName)
		}
	}
	if element.IsMembership {
		structure.attributes[testRelatedKey{element: element.Key(), property: "declaredName"}] = nullValue()
		structure.attributes[testRelatedKey{element: element.Key(), property: "declaredShortName"}] = nullValue()
	}
	for _, property := range []string{
		"declaredName", "declaredShortName", "memberName", "memberShortName",
		"visibility", "direction", "isComposite", "isEnd", "isVariable",
		"isReference", "mayTimeVary",
	} {
		definition, ok := ontology.PropertyOf(metaclass, property)
		if !ok || definition.Derived {
			continue
		}
		if element.IsMembership && (property == "declaredName" || property == "declaredShortName") {
			continue
		}
		value, ok := evaluator.model.ReflectiveFeatureValue(symbol, property)
		if !ok {
			continue
		}
		var result Value
		switch value.Kind {
		case symbols.FilterValueBool:
			result = booleanValue(value.Bool)
		case symbols.FilterValueString:
			if property == "direction" || property == "visibility" {
				result = enumValue(value.Str)
			} else {
				result = stringValue(value.Str)
			}
		default:
			continue
		}
		structure.attributes[testRelatedKey{element: element.Key(), property: property}] = result
	}
}

func testRelationshipMetaclass(kind ast.RelationshipKind) (string, string, string) {
	switch kind {
	case ast.RelTyping:
		return "FeatureTyping", "typedFeature", "type"
	case ast.RelSpecializes:
		return "Specialization", "specific", "general"
	case ast.RelSubsets:
		return "Subsetting", "subsettingFeature", "subsettedFeature"
	case ast.RelRedefines:
		return "Redefinition", "redefiningFeature", "redefinedFeature"
	case ast.RelReferences, ast.RelIncludes:
		return "ReferenceSubsetting", "referencingFeature", "referencedFeature"
	case ast.RelChains:
		return "FeatureChaining", "featureChained", "chainingFeature"
	case ast.RelCrosses:
		return "CrossSubsetting", "crossingFeature", "crossedFeature"
	case ast.RelDisjoint:
		return "Disjoining", "typeDisjoined", "disjoiningType"
	case ast.RelIntersects:
		return "Intersecting", "typeIntersected", "intersectingType"
	case ast.RelUnions:
		return "Unioning", "typeUnioned", "unioningType"
	case ast.RelDifferences:
		return "Differencing", "typeDifferenced", "differencingType"
	case ast.RelFeaturedBy:
		return "TypeFeaturing", "featureOfType", "featuringType"
	default:
		return "Relationship", "source", "target"
	}
}

func testDocumentStructure(evaluator *Evaluator, root *symbols.Scope, namespaces ...*symbols.Symbol) testStructure {
	structure := testNamespaceStructure(evaluator, namespaces...)
	document := Element{Aspect: "api-root"}
	structure.metaclasses[document.Key()] = "Namespace"
	structure.owningRelationships[document.Key()] = testOptionalElement{present: false}
	structure.owningRelatedElements[document.Key()] = testOptionalElement{present: false}
	structure.ownedRelationships[document.Key()] = nil
	for _, member := range root.Members() {
		class, ok := testMetaclass(ElementOf(member))
		if !ok {
			continue
		}
		addTestOwnedMembership(&structure, document, ElementOf(member), class)
	}
	return structure
}

type testFilterModel struct {
	*semantics.Model
	allowed map[*symbols.Symbol]bool
}

func (m testFilterModel) SatisfiesElementFilter(_ symbols.ElementFilter, element *symbols.Symbol) bool {
	return m.allowed[element]
}

func addTestOwnedMembership(structure *testStructure, owner, member Element, memberClass string) Element {
	membership := MembershipElement(Membership{
		Member: member.Symbol, Node: member.Node, Aspect: member.Aspect, kind: "OwningMembership",
	})
	structure.metaclasses[owner.Key()] = "Namespace"
	structure.metaclasses[member.Key()] = memberClass
	structure.metaclasses[membership.Key()] = "OwningMembership"
	structure.ownedRelationships[owner.Key()] = append(structure.ownedRelationships[owner.Key()], membership)
	structure.ownedRelatedElements[membership.Key()] = []Element{member}
	structure.relatedElements[testRelatedKey{element: membership.Key(), property: "memberElement"}] = []Element{member}
	structure.owningRelationships[member.Key()] = testOptionalElement{element: membership, present: true}
	structure.owningRelatedElements[membership.Key()] = testOptionalElement{element: owner, present: true}
	return membership
}

func testMetaclass(element Element) (string, bool) {
	if _, ok := element.Node.(*ast.RequireMember); ok {
		return "ConstraintUsage", true
	}
	if transition, ok := element.Node.(*ast.TransitionMember); ok && transition != nil {
		return "TransitionUsage", true
	}
	if usage, ok := element.Node.(*ast.Usage); ok && usage != nil {
		if usage.IsExhibitedState() {
			return "ExhibitStateUsage", true
		}
		metaclass, ok := map[ast.UsageKind]string{
			ast.UsagePart:             "PartUsage",
			ast.UsageAttribute:        "AttributeUsage",
			ast.UsageItem:             "ItemUsage",
			ast.UsageOccurrence:       "OccurrenceUsage",
			ast.UsageIndividual:       "OccurrenceUsage",
			ast.UsageMetadata:         "MetadataUsage",
			ast.UsageFlow:             "FlowUsage",
			ast.UsageConnector:        "ConnectorAsUsage",
			ast.UsageAction:           "ActionUsage",
			ast.UsageState:            "StateUsage",
			ast.UsageStep:             "Step",
			ast.UsageConstraint:       "ConstraintUsage",
			ast.UsageRequirement:      "RequirementUsage",
			ast.UsageSubject:          "SubjectUsage",
			ast.UsageActor:            "PartUsage",
			ast.UsageStakeholder:      "PartUsage",
			ast.UsageObjective:        "ObjectiveRequirementUsage",
			ast.UsageCase:             "CaseUsage",
			ast.UsageUseCase:          "UseCaseUsage",
			ast.UsageInteraction:      "InteractionUsage",
			ast.UsagePort:             "PortUsage",
			ast.UsageInterface:        "InterfaceUsage",
			ast.UsageAllocation:       "AllocationUsage",
			ast.UsageBinding:          "BindingConnectorAsUsage",
			ast.UsageSuccession:       "SuccessionAsUsage",
			ast.UsageEnumeration:      "EnumerationUsage",
			ast.UsageCalc:             "CalcUsage",
			ast.UsageExpr:             "Expression",
			ast.UsageVerificationCase: "VerificationCaseUsage",
			ast.UsageAnalysisCase:     "AnalysisCaseUsage",
		}[usage.Kind]
		return metaclass, ok
	}
	if element.IsMembership {
		return element.Membership.kind, element.Membership.kind != ""
	}
	if element.Symbol != nil {
		switch element.Symbol.Kind {
		case symbols.SymbolPackage:
			return "Package", true
		case symbols.SymbolNamespace:
			return "Namespace", true
		case symbols.SymbolRelationship:
			return "Specialization", true
		}
		kind := element.Symbol.Kind.String()
		if kind == "unknown" {
			return "", false
		}
		metaclass := strings.ToUpper(kind[:1]) + kind[1:]
		if strings.HasSuffix(metaclass, "Def") {
			metaclass = strings.TrimSuffix(metaclass, "Def") + "Definition"
		}
		return metaclass, true
	}
	if _, ok := element.Node.(*ast.Relationship); ok {
		return "Specialization", true
	}
	if _, ok := element.Node.(*ast.Import); ok {
		return "Import", true
	}
	if _, ok := element.Node.(*ast.LiteralBool); ok {
		return "Expression", true
	}
	return "", false
}

func testSpecializes(sub, super string) bool {
	for class := sub; class != ""; class = testMetaclassParents[class] {
		if class == super {
			return true
		}
	}
	if super == "Definition" && strings.HasSuffix(sub, "Definition") {
		return true
	}
	if super == "Usage" && strings.HasSuffix(sub, "Usage") {
		return true
	}
	return false
}

func testSymbol(t *testing.T, scope *symbols.Scope, name string) *symbols.Symbol {
	t.Helper()
	sym, ok := scope.LookupLocal(name)
	if !ok {
		t.Fatalf("symbol %q not found", name)
	}
	return sym
}

func testProperty(t *testing.T, evaluator *Evaluator, element Element, class, name string) Value {
	t.Helper()
	value, ok := evaluator.Property(element, class, name)
	if !ok {
		t.Fatalf("%s::%s is not computable for element %#v", class, name, element)
	}
	if !value.Success {
		t.Fatalf("%s::%s returned an unsuccessful value", class, name)
	}
	return value
}

func valueElements(t *testing.T, value Value) []Element {
	t.Helper()
	if value.Kind != SequenceValue {
		t.Fatalf("value kind = %v, want sequence", value.Kind)
	}
	var elements []Element
	for _, item := range value.Values {
		if item.Kind != ElementValue {
			t.Fatalf("sequence item kind = %v, want element", item.Kind)
		}
		elements = append(elements, item.Element)
	}
	return elements
}

func valueMemberships(t *testing.T, value Value) []Membership {
	t.Helper()
	if value.Kind != SequenceValue {
		t.Fatalf("value kind = %v, want sequence", value.Kind)
	}
	var memberships []Membership
	for _, item := range value.Values {
		if item.Kind != MembershipValue {
			t.Fatalf("sequence item kind = %v, want membership", item.Kind)
		}
		memberships = append(memberships, item.Membership)
	}
	return memberships
}

func containsElementSymbol(elements []Element, candidate *symbols.Symbol) bool {
	for _, element := range elements {
		if element.Symbol == candidate {
			return true
		}
	}
	return false
}

func testMembershipElements(memberships []Membership) []Element {
	elements := make([]Element, 0, len(memberships))
	for _, membership := range memberships {
		elements = append(elements, Element{Membership: membership, IsMembership: true})
	}
	return elements
}

func assertConnectorProjectionOmitted(t *testing.T, evaluator *Evaluator, connector Element, property string) {
	t.Helper()
	if _, ok := evaluator.Property(connector, "Connector", property); ok {
		t.Fatalf("Connector::%s was computed without owned reference subsettings", property)
	}
	if _, ok := evaluator.Explain(connector, "Connector", property); !ok {
		t.Fatalf("Connector::%s omission has no typed reason", property)
	}
}

func membershipHasMember(memberships []Membership, member *symbols.Symbol) bool {
	for _, membership := range memberships {
		if membership.Member == member || membership.Symbol == member {
			return true
		}
	}
	return false
}

func resolveMembershipMembers(t *testing.T, evaluator *Evaluator, memberships []Membership) []*symbols.Symbol {
	t.Helper()
	var out []*symbols.Symbol
	for _, membership := range memberships {
		member := membership.Member
		if member.Kind == symbols.SymbolAlias {
			resolved, ok := evaluator.resolver.ResolveAliasTarget(member)
			if !ok || resolved == nil {
				t.Fatal("could not resolve alias")
			}
			member = resolved
		}
		if !slices.Contains(out, member) {
			out = append(out, member)
		}
	}
	return out
}

func membershipMembers(memberships []Membership) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, membership := range memberships {
		out = append(out, membership.Member)
	}
	return out
}

func owningMemberships(memberships []Membership) []Membership {
	var owning []Membership
	for _, membership := range memberships {
		if testSpecializes(membership.kind, "OwningMembership") {
			owning = append(owning, membership)
		}
	}
	return owning
}

func assertMembershipOrder(t *testing.T, got []Membership, want []*symbols.Symbol) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("membership count = %d, want %d: %#v", len(got), len(want), got)
	}
	for i, expected := range want {
		if got[i].Member != expected {
			t.Fatalf("membership[%d].Member = %v, want %v", i, got[i].Member, expected)
		}
	}
}

func assertElementSymbolOrder(t *testing.T, got []Element, want []*symbols.Symbol) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("element count = %d, want %d: %#v", len(got), len(want), got)
	}
	for i, expected := range want {
		if got[i].Symbol != expected {
			t.Fatalf("element[%d] = %v, want %v", i, got[i].Symbol, expected)
		}
	}
}

func containsElement(value Value, expected Element) bool {
	if value.Kind == ElementValue {
		return value.Element.Key() == expected.Key()
	}
	for _, item := range value.Values {
		if item.Kind == ElementValue && item.Element.Key() == expected.Key() {
			return true
		}
	}
	return false
}
