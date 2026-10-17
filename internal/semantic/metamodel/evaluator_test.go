package metamodel

import (
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
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
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

func TestCanonicalElementKeysIgnoreContextAndMembershipKeysTrackOwner(t *testing.T) {
	_, root := newTestEvaluator(t, "identity.sysml", `package P {
		part def Owner { part member; }
	}`)
	pkg := testSymbol(t, root, "P")
	owner := testSymbol(t, pkg.Scope, "Owner")
	member := testSymbol(t, owner.Scope, "member")
	element := ElementOf(member)
	other := element
	other.Container = pkg
	other.Membership.Owner = pkg
	if element.Key() != other.Key() {
		t.Fatalf("element keys differ with contextual handles: %v != %v", element.Key(), other.Key())
	}
	if got, want := element.Key(), (Element{Node: member.Decl}).Key(); got != want {
		t.Fatalf("symbol and node element keys differ: %v != %v", got, want)
	}
	membership := element.Membership
	otherMembership := membership
	otherMembership.Owner = pkg
	otherMembership.Feature = false
	if membership.Key() == otherMembership.Key() {
		t.Fatalf("membership keys are equal across different owners: %v", membership.Key())
	}
	if membership.Key() != (Membership{Owner: owner, Node: member.Decl}).Key() {
		t.Fatalf("membership key changes with a non-identity feature flag: %v", membership.Key())
	}
	if got, want := membership.Key(), (Membership{Node: member.Decl, Owner: owner}).Key(); got != want {
		t.Fatalf("symbol and node membership keys differ: %v != %v", got, want)
	}
}

func TestMembershipFeatureFlagRequiresFeatureMembership(t *testing.T) {
	_, root := newTestEvaluator(t, "membership-feature.sysml", `package P {
		part packageFeature;
		part def Owner {
			part typeFeature;
			variant part alternative;
			in part parameter;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	owner := testSymbol(t, pkg.Scope, "Owner")
	for _, test := range []struct {
		name string
		want bool
	}{
		{name: "packageFeature", want: false},
		{name: "typeFeature", want: true},
		{name: "alternative", want: false},
		{name: "parameter", want: true},
	} {
		scope := pkg.Scope
		if test.name != "packageFeature" {
			scope = owner.Scope
		}
		symbol := testSymbol(t, scope, test.name)
		if got := ElementOf(symbol).Membership.Feature; got != test.want {
			t.Errorf("%s membership Feature = %t, want %t", test.name, got, test.want)
		}
	}
}

func TestVariableFlagsUseSemanticDerivations(t *testing.T) {
	kermlEvaluator, kermlRoot := newTestEvaluator(t, "variable-flags.kerml", `package K {
		class C { const feature f; }
	}`)
	kerml := testSymbol(t, testSymbol(t, kermlRoot, "K").Scope, "C")
	constant := testSymbol(t, kerml.Scope, "f")
	kermlEvaluator.options.Structure = testStructure{
		metaclass:   testMetaclass,
		specializes: testSpecializes,
		attributes: map[testRelatedKey]Value{
			{element: ElementOf(constant).Key(), property: "isVariable"}: booleanValue(true),
		},
	}
	if got := testProperty(t, kermlEvaluator, ElementOf(constant), "Feature", "isVariable"); got.Kind != BooleanValue || !got.Boolean {
		t.Fatalf("Feature::isVariable = %v, want true for a KerML const feature", got)
	}

	sysmlEvaluator, sysmlRoot := newTestEvaluator(t, "may-time-vary.sysml", `package Occurrences {
		occurrence def Occurrence;
	}
	package P {
		occurrence def Owner specializes Occurrences::Occurrence {
			part member;
		}
	}`)
	owner := testSymbol(t, testSymbol(t, sysmlRoot, "P").Scope, "Owner")
	member := testSymbol(t, owner.Scope, "member")
	sysmlEvaluator.options.Structure = testNamespaceStructure(sysmlEvaluator, owner)
	for _, property := range []struct {
		class string
		name  string
	}{
		{class: "Usage", name: "mayTimeVary"},
		{class: "Feature", name: "isVariable"},
	} {
		if got := testProperty(t, sysmlEvaluator, ElementOf(member), property.class, property.name); got.Kind != BooleanValue || !got.Boolean {
			t.Errorf("%s::%s = %v, want true for a composite usage owned by an Occurrence", property.class, property.name, got)
		}
	}
}

func TestStructureOwnedPropertiesAreUnsupportedWithoutStructure(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "structure-required.sysml", `package P {
		part def Part;
		part value : Part;
	}`)
	pkg := testSymbol(t, root, "P")
	value := testSymbol(t, pkg.Scope, "value")
	evaluator.options.Structure = nil
	for _, property := range []struct {
		element Element
		class   string
		name    string
	}{
		{ElementOf(pkg), "Element", "ownedRelationship"},
		{ElementOf(pkg), "Namespace", "ownedMembership"},
		{ElementOf(value), "Feature", "isVariable"},
	} {
		if _, ok := evaluator.Property(property.element, property.class, property.name); ok {
			t.Errorf("%s::%s succeeded without Structure", property.class, property.name)
		}
	}
	library, ok := evaluator.Property(ElementOf(pkg), "Element", "isLibraryElement")
	if !ok || library.Kind != BooleanValue || library.Boolean {
		t.Fatalf("Element::isLibraryElement = %#v, %t without Structure; want false", library, ok)
	}
}

func TestEvaluatorCoreProperties(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "evaluator.sysml", `package Base {
		part def Anything;
		attribute def DataValue;
		attribute dataValues : DataValue;
	}
	package Parts {
		part def Part;
		feature parts : Part;
	}
	package P {
		part def Base {
			attribute inherited;
			private attribute hidden;
			protected attribute protectedValue;
			attribute same;
		}
		part def Mid specializes Base {
			attribute middle;
			attribute :>> same;
		}
		part def Leaf specializes Mid {
			in attribute input;
			out attribute output;
			attribute own[2];
			attribute typed : Base;
			part nested;
			part parent : Leaf {
				part child;
			}
		}
		part instance : Leaf;
		alias BaseAlias for Base;
	}`)
	pkg := testSymbol(t, root, "P")
	base := testSymbol(t, pkg.Scope, "Base")
	mid := testSymbol(t, pkg.Scope, "Mid")
	leaf := testSymbol(t, pkg.Scope, "Leaf")
	instance := testSymbol(t, pkg.Scope, "instance")
	parent := testSymbol(t, leaf.Scope, "parent")
	nested := testSymbol(t, parent.Scope, "child")
	input := testSymbol(t, leaf.Scope, "input")
	output := testSymbol(t, leaf.Scope, "output")
	typed := testSymbol(t, leaf.Scope, "typed")
	alias := testSymbol(t, pkg.Scope, "BaseAlias")
	structure := testDocumentStructure(
		evaluator, root, pkg, base, mid, leaf, instance, parent,
	)
	structure.attributes[testRelatedKey{
		element: ElementOf(nested).Key(), property: "isComposite",
	}] = booleanValue(true)
	evaluator.options.Structure = structure

	name := testProperty(t, evaluator, ElementOf(leaf), "Element", "name")
	if name.Kind != StringValue || name.String != "Leaf" {
		t.Fatalf("Element::name = %#v, want Leaf", name)
	}
	shortName := testProperty(t, evaluator, ElementOf(leaf), "Element", "shortName")
	if shortName.Kind != NullValue {
		t.Fatalf("Element::shortName = %#v, want null", shortName)
	}
	owner := testProperty(t, evaluator, ElementOf(leaf), "Element", "owner")
	if owner.Kind != ElementValue || owner.Element.Symbol != pkg {
		t.Fatalf("Element::owner = %#v, want package P", owner)
	}
	namespace := testProperty(t, evaluator, ElementOf(leaf), "Element", "owningNamespace")
	if namespace.Kind != ElementValue || namespace.Element.Symbol != pkg {
		t.Fatalf("Element::owningNamespace = %#v, want package P", namespace)
	}
	owningMembership := testProperty(t, evaluator, ElementOf(leaf), "Element", "owningMembership")
	if owningMembership.Kind != MembershipValue || owningMembership.Membership.Member != leaf {
		t.Fatalf("Element::owningMembership = %#v, want Leaf's membership", owningMembership)
	}
	ownedElements := valueElements(t, testProperty(t, evaluator, ElementOf(leaf), "Element", "ownedElement"))
	if !containsElementSymbol(ownedElements, parent) || containsElementSymbol(ownedElements, mid) {
		t.Fatalf("Element::ownedElement = %v, want local parent element only", ownedElements)
	}
	qualifiedName := testProperty(t, evaluator, ElementOf(leaf), "Element", "qualifiedName")
	if qualifiedName.Kind != StringValue || qualifiedName.String != "P::Leaf" {
		t.Fatalf("Element::qualifiedName = %#v, want P::Leaf", qualifiedName)
	}
	library := testProperty(t, evaluator, ElementOf(leaf), "Element", "isLibraryElement")
	if library.Kind != BooleanValue || library.Boolean {
		t.Fatalf("Element::isLibraryElement = %#v, want false", library)
	}
	for _, property := range []string{"documentation", "ownedAnnotation", "textualRepresentation"} {
		valueElements(t, testProperty(t, evaluator, ElementOf(leaf), "Element", property))
	}
	relationships := valueElements(t, testProperty(t, evaluator, ElementOf(leaf), "Element", "ownedRelationship"))
	for _, relationship := range relationships {
		implied := testProperty(t, evaluator, relationship, "Element", "isImpliedIncluded")
		if implied.Kind != BooleanValue || implied.Boolean {
			t.Fatalf("Element::ownedRelationship includes implied element: %#v", relationship)
		}
	}

	for _, property := range []string{"membership", "ownedMembership", "member", "ownedMember", "importedMembership", "ownedImport"} {
		value := testProperty(t, evaluator, ElementOf(pkg), "Namespace", property)
		if property == "membership" || property == "ownedMembership" {
			valueMemberships(t, value)
		} else {
			valueElements(t, value)
		}
	}

	for _, property := range []string{
		"feature", "ownedFeature", "inheritedFeature", "inheritedMembership",
		"featureMembership", "ownedFeatureMembership", "input", "output", "directedFeature",
		"endFeature", "ownedEndFeature", "ownedSpecialization", "differencingType", "intersectingType", "unioningType",
	} {
		value := testProperty(t, evaluator, ElementOf(leaf), "Type", property)
		switch property {
		case "inheritedMembership", "featureMembership", "ownedFeatureMembership":
			valueMemberships(t, value)
		default:
			valueElements(t, value)
		}
	}
	if got := valueElements(t, testProperty(t, evaluator, ElementOf(leaf), "Type", "inheritedFeature")); !containsElementSymbol(got, testSymbol(t, base.Scope, "inherited")) {
		t.Fatalf("Type::inheritedFeature = %v, want Base::inherited", got)
	}
	if got := valueElements(t, testProperty(t, evaluator, ElementOf(leaf), "Type", "input")); !containsElementSymbol(got, input) {
		t.Fatalf("Type::input = %v, want input", got)
	}
	if got := valueElements(t, testProperty(t, evaluator, ElementOf(leaf), "Type", "output")); !containsElementSymbol(got, output) {
		t.Fatalf("Type::output = %v, want output", got)
	}
	conjugated := testProperty(t, evaluator, ElementOf(leaf), "Type", "isConjugated")
	if conjugated.Kind != BooleanValue || conjugated.Boolean {
		t.Fatalf("Type::isConjugated = %#v, want false", conjugated)
	}
	multiplicity := testProperty(t, evaluator, ElementOf(instance), "Type", "multiplicity")
	if multiplicity.Kind != NullValue {
		t.Fatalf("Type::multiplicity = %#v, want null", multiplicity)
	}

	for _, property := range []string{
		"type", "featuringType", "owningType", "ownedTyping", "ownedSubsetting",
		"ownedRedefinition", "ownedReferenceSubsetting", "featureTarget", "chainingFeature",
	} {
		testProperty(t, evaluator, ElementOf(typed), "Feature", property)
	}
	composite := testProperty(t, evaluator, ElementOf(nested), "Feature", "isComposite")
	if composite.Kind != BooleanValue || !composite.Boolean {
		t.Fatalf("Feature::isComposite = %#v, want true", composite)
	}
	direction := testProperty(t, evaluator, ElementOf(input), "Feature", "direction")
	if direction.Kind != EnumValue || direction.Enum == "" {
		t.Fatalf("Feature::direction = %#v, want a direction", direction)
	}
	testProperty(t, evaluator, ElementOf(typed), "Feature", "crossFeature")

	for _, property := range []string{
		"definition", "usage", "nestedUsage", "directedUsage", "variant",
		"isReference", "owningUsage", "owningDefinition", "nestedPart", "nestedAttribute",
	} {
		testProperty(t, evaluator, ElementOf(parent), "Usage", property)
	}
	if _, ok := evaluator.Property(ElementOf(parent), "Usage", "ownedUsage"); ok {
		t.Fatal("Usage::ownedUsage is not defined for a Usage")
	}
	for _, property := range []string{"ownedUsage", "usage", "directedUsage", "variant"} {
		testProperty(t, evaluator, ElementOf(leaf), "Definition", property)
	}
	for _, property := range []string{"ownedPart", "ownedAttribute"} {
		testProperty(t, evaluator, ElementOf(leaf), "Definition", property)
	}
	ownerUsage := testProperty(t, evaluator, ElementOf(nested), "Usage", "owningUsage")
	if ownerUsage.Kind != ElementValue || ownerUsage.Element.Symbol != parent {
		t.Fatalf("Usage::owningUsage = %#v, want parent", ownerUsage)
	}
	ownerDefinition := testProperty(t, evaluator, ElementOf(nested), "Usage", "owningDefinition")
	if ownerDefinition.Kind != NullValue {
		t.Fatalf("nested Usage::owningDefinition = %#v, want null because owningType is a Usage", ownerDefinition)
	}
	direct := testSymbol(t, leaf.Scope, "nested")
	directDefinition := testProperty(t, evaluator, ElementOf(direct), "Usage", "owningDefinition")
	if directDefinition.Kind != ElementValue || directDefinition.Element.Symbol != leaf {
		t.Fatalf("directly owned Usage::owningDefinition = %#v, want Leaf", directDefinition)
	}
	directUsage := testProperty(t, evaluator, ElementOf(direct), "Usage", "owningUsage")
	if directUsage.Kind != NullValue {
		t.Fatalf("directly owned Usage::owningUsage = %#v, want null because owningType is a Definition", directUsage)
	}
	packageOwnedDefinition := testProperty(t, evaluator, ElementOf(instance), "Usage", "owningDefinition")
	if packageOwnedDefinition.Kind != NullValue {
		t.Fatalf("OwningMembership-owned Usage::owningDefinition = %#v, want null", packageOwnedDefinition)
	}
	packageOwnedUsage := testProperty(t, evaluator, ElementOf(instance), "Usage", "owningUsage")
	if packageOwnedUsage.Kind != NullValue {
		t.Fatalf("OwningMembership-owned Usage::owningUsage = %#v, want null", packageOwnedUsage)
	}
	aliasElement := ElementOf(alias)
	if !aliasElement.IsMembership {
		t.Fatal("alias declaration is not represented as its own membership")
	}
	for _, property := range []string{"memberElement", "memberName", "memberShortName", "membershipOwningNamespace"} {
		testProperty(t, evaluator, aliasElement, "Membership", property)
	}
	member := testProperty(t, evaluator, aliasElement, "Membership", "memberElement")
	if member.Kind != ElementValue || member.Element.Symbol != base {
		t.Fatalf("Membership::memberElement = %#v, want Base", member)
	}
	memberName := testProperty(t, evaluator, aliasElement, "Membership", "memberName")
	if memberName.Kind != StringValue || memberName.String != "BaseAlias" {
		t.Fatalf("Membership::memberName = %#v, want BaseAlias", memberName)
	}
	memberShortName := testProperty(t, evaluator, aliasElement, "Membership", "memberShortName")
	if memberShortName.Kind != NullValue {
		t.Fatalf("Membership::memberShortName = %#v, want null", memberShortName)
	}
	for _, property := range []string{"name", "shortName"} {
		value := testProperty(t, evaluator, aliasElement, "Element", property)
		if value.Kind != NullValue {
			t.Fatalf("Alias Element::%s = %#v, want null", property, value)
		}
	}
	owningNamespace := testProperty(t, evaluator, aliasElement, "Membership", "membershipOwningNamespace")
	if owningNamespace.Kind != ElementValue || owningNamespace.Element.Symbol != pkg {
		t.Fatalf("Membership::membershipOwningNamespace = %#v, want package P", owningNamespace)
	}
}

func TestEvaluatorInheritanceVisibilityAndRedefinition(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "inheritance.sysml", `package Base {
		part def Anything;
	}
	package Parts {
		part def Part;
	}
	package P {
		part def Base {
			attribute publicValue;
			private attribute privateValue;
			protected attribute protectedValue;
			attribute shared;
		}
		part def Middle specializes Base {
			attribute middleValue;
			attribute :>> shared;
		}
		part def Leaf specializes Middle {
			attribute leafValue;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	base := testSymbol(t, pkg.Scope, "Base")
	middle := testSymbol(t, pkg.Scope, "Middle")
	leaf := testSymbol(t, pkg.Scope, "Leaf")
	baseShared := testSymbol(t, base.Scope, "shared")
	middleShared := testSymbol(t, middle.Scope, "shared")
	publicValue := testSymbol(t, base.Scope, "publicValue")
	privateValue := testSymbol(t, base.Scope, "privateValue")
	protectedValue := testSymbol(t, base.Scope, "protectedValue")
	middleValue := testSymbol(t, middle.Scope, "middleValue")

	leafFeatures := valueElements(t, testProperty(t, evaluator, ElementOf(leaf), "Type", "feature"))
	if !containsElementSymbol(leafFeatures, publicValue) || !containsElementSymbol(leafFeatures, protectedValue) ||
		containsElementSymbol(leafFeatures, privateValue) || containsElementSymbol(leafFeatures, baseShared) ||
		!containsElementSymbol(leafFeatures, middleShared) || !containsElementSymbol(leafFeatures, middleValue) {
		var names []string
		for _, feature := range leafFeatures {
			names = append(names, evaluator.resolver.Index().GetFQN(feature.Symbol))
		}
		t.Fatalf("Leaf::feature does not respect inheritance visibility and redefinition: %v", names)
	}
	inherited := valueElements(t, testProperty(t, evaluator, ElementOf(leaf), "Type", "inheritedFeature"))
	if !containsElementSymbol(inherited, publicValue) || !containsElementSymbol(inherited, protectedValue) ||
		containsElementSymbol(inherited, privateValue) || containsElementSymbol(inherited, baseShared) {
		t.Fatalf("Leaf::inheritedFeature = %v, want public/protected and masked shared", inherited)
	}
}

func TestMembershipNamesUseGraphAttributes(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "membership-names.sysml", `package P {
		part def Vehicle {
			attribute watt;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	vehicle := testSymbol(t, pkg.Scope, "Vehicle")
	watt := testSymbol(t, vehicle.Scope, "watt")
	membership := MembershipElement(ElementOf(watt).Membership)
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			ElementOf(watt).Key(): "AttributeUsage",
			membership.Key():      "Membership",
		},
		ownedRelationships:    make(map[ElementKey][]Element),
		owningRelationships:   make(map[ElementKey]testOptionalElement),
		ownedRelatedElements:  make(map[ElementKey][]Element),
		owningRelatedElements: make(map[ElementKey]testOptionalElement),
		relatedElements:       make(map[testRelatedKey][]Element),
	}
	owningMembership := addTestOwnedMembership(&structure, ElementOf(vehicle), ElementOf(watt), "AttributeUsage")
	evaluator.options.Structure = structure

	for _, property := range []string{"memberName", "memberShortName"} {
		value := testProperty(t, evaluator, membership, "Membership", property)
		if value.Kind != NullValue {
			t.Errorf("Membership::%s = %#v, want null without a graph-stated alias", property, value)
		}
	}
	memberName := testProperty(t, evaluator, owningMembership, "Membership", "memberName")
	if memberName.Kind != StringValue || memberName.String != "watt" {
		t.Fatalf("OwningMembership::memberName = %#v, want derived member name watt", memberName)
	}
}

func TestEvaluatorMembershipDeclarationOrder(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "membership-order.sysml", `package Base {
		part def Anything;
	}
	package Parts {
		part def Part;
	}
	package P {
		attribute def Data;
		part def Base {
			part inheritedFirst;
			part inheritedLast;
		}
		part def Derived specializes Base {
			part alpha;
			attribute : Data;
			alias inheritedAlias for inheritedFirst;
			part middle;
			attribute : Data;
			part omega;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	derived := testSymbol(t, pkg.Scope, "Derived")
	base := testSymbol(t, pkg.Scope, "Base")
	evaluator.options.Structure = testDocumentStructure(evaluator, root, pkg, base, derived)

	local := valueMemberships(t, testProperty(t, evaluator, ElementOf(derived), "Namespace", "ownedMembership"))
	if len(local) != 6 {
		t.Fatalf("ownedMembership count = %d, want six local declarations", len(local))
	}
	for i := 1; i < len(local); i++ {
		if local[i-1].Node == nil || local[i].Node == nil ||
			local[i-1].Node.Span().Offset >= local[i].Node.Span().Offset {
			t.Fatalf("ownedMembership is not in declaration order: %#v", local)
		}
	}
	if local[2].kind != "Alias" {
		t.Fatalf("third owned membership kind = %q, want alias at its declaration position", local[2].kind)
	}
	for _, i := range []int{1, 4} {
		name := testProperty(t, evaluator, ElementOf(local[i].Member), "Element", "name")
		if name.Kind != NullValue {
			t.Fatalf("unnamed owned member at position %d has name %#v", i, name)
		}
	}
	if !local[0].Feature || local[2].Feature || !local[3].Feature {
		t.Fatalf("membership feature flags around alias are incorrect: %#v", local)
	}

	inherited := valueMemberships(t, testProperty(t, evaluator, ElementOf(derived), "Type", "inheritedMembership"))
	all := valueMemberships(t, testProperty(t, evaluator, ElementOf(derived), "Namespace", "membership"))
	wantMemberships := append(append([]Membership(nil), local...), inherited...)
	assertMembershipOrder(t, all, membershipMembers(wantMemberships))

	allMembers := resolveMembershipMembers(t, evaluator, wantMemberships)
	assertElementSymbolOrder(t, valueElements(t, testProperty(t, evaluator, ElementOf(derived), "Namespace", "ownedMember")),
		resolveMembershipMembers(t, evaluator, owningMemberships(local)))
	assertElementSymbolOrder(t, valueElements(t, testProperty(t, evaluator, ElementOf(derived), "Namespace", "member")), allMembers)

	alias := testSymbol(t, derived.Scope, "inheritedAlias")
	if ElementOf(alias).Membership.Feature {
		t.Fatal("alias membership is incorrectly marked as a FeatureMembership")
	}
}

func TestStructureBackedMembershipOrderPreservesUnnamedAndAliasedMembers(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "membership-order-structure.sysml", `package Items {
		abstract item def Item;
	}
	package Parts {
		abstract part def Part :> Items::Item;
	}
	package Base {
		abstract attribute def DataValue;
	}
	package P {
		attribute def Data;
		part def Base {
			part inheritedFirst;
			part inheritedLast;
		}
		part def Derived specializes Base {
			part alpha;
			attribute : Data;
			alias inheritedAlias for inheritedFirst;
			part middle;
			attribute : Data;
			part omega;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	derived := testSymbol(t, pkg.Scope, "Derived")
	local := memberships(derived.Scope.AllMembers())
	allMembers, ok := evaluator.model.ReflectiveElements(derived, "member")
	if !ok {
		t.Fatal("inherited member facts are unavailable")
	}
	inherited := memberships(evaluator.inheritedSymbols(derived, allMembers))
	all := append(append([]Membership(nil), local...), inherited...)

	structure := testStructure{
		metaclasses: map[ElementKey]string{
			ElementOf(derived).Key(): "PartDefinition",
		},
		ownedRelationships: map[ElementKey][]Element{
			ElementOf(derived).Key(): testMembershipElements(local),
		},
		relatedElements: make(map[testRelatedKey][]Element),
	}
	for _, membership := range all {
		element := Element{Membership: membership, IsMembership: true}
		if membership.Member == nil {
			t.Fatalf("membership has no member handle: %#v", membership)
		}
		member := membership.Member
		if member.Kind == symbols.SymbolAlias {
			resolved, ok := evaluator.resolver.ResolveAliasTarget(member)
			if !ok || resolved == nil {
				t.Fatalf("could not resolve alias member %q", member.Name)
			}
			member = resolved
		}
		switch {
		case membership.kind == "Alias":
			structure.metaclasses[element.Key()] = "Membership"
		case membership.Feature:
			structure.metaclasses[element.Key()] = "FeatureMembership"
		default:
			structure.metaclasses[element.Key()] = "OwningMembership"
		}
		memberElement := ElementOf(member)
		if memberClass, ok := testMetaclass(memberElement); ok {
			structure.metaclasses[memberElement.Key()] = memberClass
		} else if usage, ok := member.Decl.(*ast.Usage); ok {
			kind := usage.Kind.String()
			structure.metaclasses[memberElement.Key()] = strings.ToUpper(kind[:1]) + kind[1:] + "Usage"
		} else if membership.Feature {
			structure.metaclasses[memberElement.Key()] = "Feature"
		} else {
			structure.metaclasses[memberElement.Key()] = "Element"
		}
		structure.ownedRelationships[memberElement.Key()] = []Element{}
		structure.relatedElements[testRelatedKey{element: element.Key(), property: "memberElement"}] = []Element{memberElement}
		structure.relatedElements[testRelatedKey{element: element.Key(), property: "ownedMemberElement"}] = []Element{memberElement}
	}
	evaluator.options.Structure = structure

	gotOwned := valueMemberships(t, testProperty(t, evaluator, ElementOf(derived), "Namespace", "ownedMembership"))
	assertMembershipOrder(t, gotOwned, membershipMembers(local))
	if len(gotOwned) != 6 || gotOwned[2].kind != "Alias" {
		t.Fatalf("owned membership declaration order = %#v; want unnamed members and alias interleaved", gotOwned)
	}
	for _, index := range []int{1, 4} {
		name := testProperty(t, evaluator, ElementOf(gotOwned[index].Member), "Element", "name")
		if name.Kind != NullValue {
			t.Fatalf("unnamed owned member at %d has name %#v", index, name)
		}
	}

	gotAll := valueMemberships(t, testProperty(t, evaluator, ElementOf(derived), "Namespace", "membership"))
	assertMembershipOrder(t, gotAll, membershipMembers(all))
	assertElementSymbolOrder(t,
		valueElements(t, testProperty(t, evaluator, ElementOf(derived), "Namespace", "ownedMember")),
		resolveMembershipMembers(t, evaluator, owningMemberships(local)))
	assertElementSymbolOrder(t,
		valueElements(t, testProperty(t, evaluator, ElementOf(derived), "Namespace", "member")),
		resolveMembershipMembers(t, evaluator, all))
}

func testMembershipElements(memberships []Membership) []Element {
	elements := make([]Element, 0, len(memberships))
	for _, membership := range memberships {
		elements = append(elements, Element{Membership: membership, IsMembership: true})
	}
	return elements
}

func TestStructureQualifiedNameEscapesNameSegments(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "escaped-qualified-name.sysml", `package 'Space Name' {
		part def 'Wheel Type';
	}`)
	space := testSymbol(t, root, "Space Name")
	wheel := testSymbol(t, space.Scope, "Wheel Type")
	element := ElementOf(wheel)
	evaluator.options.Structure = testDocumentStructure(evaluator, root, space)
	got := testProperty(t, evaluator, element, "Element", "qualifiedName")
	if got.Kind != StringValue || got.String != "'Space Name'::'Wheel Type'" {
		t.Fatalf("Element::qualifiedName = %#v; want %q", got, "'Space Name'::'Wheel Type'")
	}
}

func TestStructureQualifiedNameUsesOwnedNamespacesAndRejectsDuplicateNames(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "qualified-name-structure.sysml", `package 'Space Name' {
		part def 'Wheel Type';
	}
	part def Top;`)
	space := testSymbol(t, root, "Space Name")
	first := Element{Aspect: "first-repeated"}
	second := Element{Aspect: "second-repeated"}
	wheel := testSymbol(t, space.Scope, "Wheel Type")
	top := testSymbol(t, root, "Top")
	structure := testDocumentStructure(evaluator, root, space)
	spaceElement := ElementOf(space)
	addTestOwnedMembership(&structure, spaceElement, first, "PartDefinition")
	addTestOwnedMembership(&structure, spaceElement, second, "PartDefinition")
	structure.attributes[testRelatedKey{element: first.Key(), property: "declaredName"}] = stringValue("Repeated")
	structure.attributes[testRelatedKey{element: second.Key(), property: "declaredName"}] = stringValue("Repeated")
	evaluator.options.Structure = structure

	for _, test := range []struct {
		element Element
		want    string
	}{
		{first, "'Space Name'::Repeated"},
		{ElementOf(wheel), "'Space Name'::'Wheel Type'"},
		{ElementOf(top), "Top"},
	} {
		got := testProperty(t, evaluator, test.element, "Element", "qualifiedName")
		if got.Kind != StringValue || got.String != test.want {
			t.Errorf("Element::qualifiedName = %#v; want %q", got, test.want)
		}
	}
	duplicate, ok := evaluator.Property(second, "Element", "qualifiedName")
	if !ok || duplicate.Kind != NullValue {
		t.Fatalf("duplicate qualifiedName = %#v, %t; want null", duplicate, ok)
	}
}

func TestModelQualifiedNameIsNullForLaterDuplicateMember(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "qualified-name-model.sysml", `package P {
	part def A;
	part def A;
}`)
	evaluator.options.Structure = nil
	pkg := testSymbol(t, root, "P")
	var definitions []*symbols.Symbol
	for _, member := range pkg.Scope.Members() {
		if evaluator.model.EffectiveNameOf(member) == "A" {
			definitions = append(definitions, member)
		}
	}
	if len(definitions) != 2 {
		t.Fatalf("got %d same-named definitions, want 2", len(definitions))
	}
	// KerML 1.1 deriveElementQualifiedName returns null when the matching ownedMember is not first.
	first := testProperty(t, evaluator, ElementOf(definitions[0]), "Element", "qualifiedName")
	if first.Kind != StringValue || first.String != "P::A" {
		t.Fatalf("first qualifiedName = %#v, want P::A", first)
	}
	second := testProperty(t, evaluator, ElementOf(definitions[1]), "Element", "qualifiedName")
	if second.Kind != NullValue {
		t.Fatalf("later duplicate qualifiedName = %#v, want null", second)
	}
}

func TestNamespaceOwnedMemberNamesResolveForQualifiedName(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "qualified-name-member-names.sysml", `package Synonyms {
		state def Idle;
		constraint def Safe;
		part def Vehicle {
			exhibit idling : Idle;
			assert safe : Safe;
			inv alsoSafe : Safe;
		}
	}`)
	namespace := testSymbol(t, testSymbol(t, root, "Synonyms").Scope, "Vehicle")
	members := testProperty(t, evaluator, ElementOf(namespace), "Namespace", "ownedMember")
	if len(members.Values) == 0 || members.Values[0].Kind != ElementValue {
		t.Fatalf("Namespace::ownedMember = %#v; want exhibit member first", members)
	}
	structure := testDocumentStructure(evaluator, root, testSymbol(t, root, "Synonyms"), namespace)
	exhibit := members.Values[0].Element
	evaluator.options.Structure = structure
	reference := Element{Aspect: "unresolved-exhibit-reference"}
	structure.metaclasses[reference.Key()] = "ReferenceSubsetting"
	structure.ownedRelationships[exhibit.Key()] = append(
		structure.ownedRelationships[exhibit.Key()], reference,
	)
	if structure.unresolvedRelated == nil {
		structure.unresolvedRelated = make(map[testRelatedKey]bool)
	}
	structure.unresolvedRelated[testRelatedKey{
		element: reference.Key(), property: "referencedFeature",
	}] = true
	evaluator.options.Structure = structure
	members = testProperty(t, evaluator, ElementOf(namespace), "Namespace", "ownedMember")
	for index, member := range members.Values {
		if member.Kind != ElementValue {
			t.Fatalf("Namespace::ownedMember[%d] = %#v; want element", index, member)
		}
		name, ok := evaluator.Property(member.Element, "Element", "name")
		if !ok {
			t.Errorf("Namespace::ownedMember[%d] Element::name is unsupported (%s)", index,
				evaluator.structureIdentityLabel(member.Element))
		} else if name.Kind != StringValue && name.Kind != NullValue {
			t.Errorf("Namespace::ownedMember[%d] Element::name = %#v; want string or null", index, name)
		}
	}
	if name, ok := evaluator.Property(exhibit, "Element", "name"); !ok || name.Kind != NullValue {
		t.Fatalf("ExhibitStateUsage::name = %#v, %t; want a computable null name", name, ok)
	}
	var safe Element
	for _, member := range members.Values {
		if member.Kind != ElementValue {
			continue
		}
		name, ok := evaluator.Property(member.Element, "Element", "name")
		if ok && name.Kind == StringValue && name.String == "safe" {
			safe = member.Element
			break
		}
	}
	if safe.Key() == (ElementKey{}) {
		t.Fatal("owned member safe was not found")
	}
	if got := testProperty(t, evaluator, safe, "Element", "qualifiedName"); got.Kind != StringValue ||
		got.String != "Synonyms::Vehicle::safe" {
		t.Fatalf("safe qualifiedName = %#v; want %q", got, "Synonyms::Vehicle::safe")
	}
}

func TestStructureQualifiedNameFallsBackToShortName(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "qualified-name-short-name.sysml", `package P {
		requirement def R {
			require constraint <'HLR-R059'> { x }
		}
	}`)
	pkg := testSymbol(t, root, "P")
	requirement := testSymbol(t, pkg.Scope, "R")
	evaluator.options.Structure = testNamespaceStructure(evaluator, pkg, requirement)
	members := testProperty(t, evaluator, ElementOf(requirement), "Namespace", "ownedMember")
	var constraint Element
	for _, member := range members.Values {
		if member.Kind != ElementValue {
			t.Fatalf("Namespace::ownedMember = %#v; want elements", members)
		}
		shortName, ok := evaluator.Property(member.Element, "Element", "shortName")
		if ok && shortName.Kind == StringValue && shortName.String == "HLR-R059" {
			constraint = member.Element
			break
		}
	}
	if constraint.Key() == (ElementKey{}) {
		t.Fatal("short-named constraint HLR-R059 was not found")
	}
	got := testProperty(t, evaluator, constraint, "Element", "qualifiedName")
	if got.Kind != StringValue || got.String != "R::'HLR-R059'" {
		namespace, namespaceOK := evaluator.Property(constraint, "Element", "owningNamespace")
		var namespaceOwner, parentName Value
		owningRelationship, relationshipPresent, relationshipKnown :=
			evaluator.options.Structure.OwningRelationship(constraint)
		var namespaceEndpoints []Element
		var endpointsKnown bool
		if relationshipPresent {
			namespaceEndpoints, endpointsKnown =
				evaluator.options.Structure.RelatedElements(owningRelationship, "membershipOwningNamespace")
		}
		if namespaceOK && namespace.Kind == ElementValue {
			namespaceOwner, _ = evaluator.Property(namespace.Element, "Element", "owner")
			parentName, _ = evaluator.Property(namespace.Element, "Element", "qualifiedName")
		}
		shortName, _ := evaluator.Property(constraint, "Element", "shortName")
		t.Fatalf("Element::qualifiedName = %#v; want %q (shortName=%#v membership=%#v owningRelationship=%#v %t %t membershipOwningNamespace=%#v %t owningNamespace=%#v namespaceOwner=%#v namespaceQualifiedName=%#v)",
			got, "R::'HLR-R059'", shortName, constraint.Membership, owningRelationship,
			relationshipPresent, relationshipKnown, namespaceEndpoints, endpointsKnown, namespace, namespaceOwner, parentName)
	}
}

func TestStructureQualifiedNameUsesOwningNamespaceOwnerForRoot(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "qualified-name-owner-root.sysml", "package P {}")
	outer := Element{Aspect: "outer-namespace"}
	namespace := Element{Aspect: "namespace-without-owner"}
	child := Element{Aspect: "child"}
	namespaceMembership := MembershipElement(Membership{Aspect: "namespace-membership", kind: "OwningMembership"})
	childMembership := MembershipElement(Membership{Aspect: "child-membership", kind: "OwningMembership"})
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			outer.Key():               "Namespace",
			namespace.Key():           "Namespace",
			child.Key():               "PartDefinition",
			namespaceMembership.Key(): "OwningMembership",
			childMembership.Key():     "OwningMembership",
		},
		ownedRelationships: map[ElementKey][]Element{
			namespace.Key(): {childMembership},
		},
		owningRelationships: map[ElementKey]testOptionalElement{
			namespace.Key(): {element: namespaceMembership, present: true},
			child.Key():     {element: childMembership, present: true},
		},
		owningRelatedElements: make(map[ElementKey]testOptionalElement),
		relatedElements: map[testRelatedKey][]Element{
			{element: namespaceMembership.Key(), property: "membershipOwningNamespace"}: {outer},
			{element: childMembership.Key(), property: "membershipOwningNamespace"}:     {namespace},
			{element: childMembership.Key(), property: "memberElement"}:                 {child},
		},
		attributes: map[testRelatedKey]Value{
			{element: namespace.Key(), property: "declaredName"}: stringValue("Inner"),
			{element: child.Key(), property: "declaredName"}:     stringValue("Child"),
		},
	}
	evaluator.options.Structure = structure
	owningNamespace := testProperty(t, evaluator, namespace, "Element", "owningNamespace")
	if owningNamespace.Kind != ElementValue || owningNamespace.Element.Key() != outer.Key() {
		t.Fatalf("namespace owningNamespace = %#v; want outer namespace", owningNamespace)
	}
	owner := testProperty(t, evaluator, namespace, "Element", "owner")
	if owner.Kind != NullValue {
		t.Fatalf("namespace owner = %#v; want null", owner)
	}
	got := testProperty(t, evaluator, child, "Element", "qualifiedName")
	if got.Kind != StringValue || got.String != "Child" {
		t.Fatalf("child qualifiedName = %#v; want the unqualified root name", got)
	}
}

// SysML PerformActionUsage::namingFeature uses performedAction when it differs from self (SysML.uml:11996–12005).
func TestPerformActionUsageNamingUsesPerformedAction(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "perform-action-naming.sysml", "package P {}")
	action := Element{Symbol: &symbols.Symbol{Name: "performed"}}
	chain := Element{Aspect: "performed-action-chain"}
	target := Element{Aspect: "performed-action"}
	redefinition := Element{Aspect: "performed-action-reference"}
	chaining := Element{Aspect: "performed-action-feature-chaining"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			action.Key():       "PerformActionUsage",
			redefinition.Key(): "ReferenceSubsetting",
			chain.Key():        "Feature",
			target.Key():       "ActionUsage",
			chaining.Key():     "FeatureChaining",
		},
		ownedRelationships: map[ElementKey][]Element{
			action.Key(): {redefinition},
			chain.Key():  {chaining},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: redefinition.Key(), property: "referencedFeature"}: {chain},
			{element: chaining.Key(), property: "chainingFeature"}:       {target},
		},
	}
	namingFeature, handled, ok := evaluator.structureNamingFeature(action)
	if !handled || !ok || namingFeature.Key() != target.Key() {
		t.Fatalf("PerformActionUsage namingFeature = %#v, %t; want performed action %v", namingFeature, ok, target)
	}
}

// KerML 1.1 §8.3.4.9.5 requires PayloadFeature to redefine
// Transfers::Transfer::payload; §8.4.2 makes that relationship implied.
func TestPayloadFeatureWithoutIdentificationHasNoEffectiveName(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "payload-feature-name.sysml", "package P {}")
	payload := Element{Aspect: "payload"}
	evaluator.options.Structure = testStructure{
		metaclasses:        map[ElementKey]string{payload.Key(): "PayloadFeature"},
		ownedRelationships: map[ElementKey][]Element{payload.Key(): {}},
	}
	for _, property := range []string{"name", "shortName"} {
		if _, ok := evaluator.Property(payload, "Element", property); ok {
			t.Errorf("PayloadFeature::%s was derived without its implied Redefinition", property)
		}
		reason, ok := evaluator.Explain(payload, "Element", property)
		if !ok || reason.Category != ReasonImpliedRelationshipNotMaterialized {
			t.Errorf("PayloadFeature::%s omission = %#v, %t; want implied-relationship-not-materialized",
				property, reason, ok)
		}
	}
}

func TestSuccessionAsUsageHasNoReferenceDerivedName(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "succession-name.sysml", "package P {}")
	succession := &symbols.Symbol{
		Name:         "join1",
		Kind:         symbols.SymbolSuccessionUsage,
		Naming:       symbols.NamedByReference,
		NamingTarget: &ast.QualifiedName{},
	}
	element := ElementOf(succession)
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{element.Key(): "SuccessionAsUsage"},
	}
	name := testProperty(t, evaluator, element, "Element", "name")
	if name.Kind != NullValue {
		t.Fatalf("SuccessionAsUsage::name = %#v; want null", name)
	}
}

func TestStructureEndUsageIsConstantWhenMayTimeVary(t *testing.T) {
	const model = `package P {
		port def Port;
		occurrence def I {
			end port p : Port;
		}
	}`
	evaluator, root := newTestEvaluatorWithIndex(t, libs.NewModelIndex(), "end-usage-is-constant.sysml", model)
	pkg := testSymbol(t, root, "P")
	iface := testSymbol(t, pkg.Scope, "I")
	end := testSymbol(t, iface.Scope, "p")
	if !evaluator.model.UsageMayTimeVary(end) {
		t.Fatal("end port's computed mayTimeVary is false; test requires a varying end")
	}
	got := testProperty(t, evaluator, ElementOf(end), "Feature", "isConstant")
	if got.Kind != BooleanValue || !got.Boolean {
		t.Fatalf("end Feature::isConstant = %#v; want true", got)
	}

	statedEvaluator, statedRoot := newTestEvaluatorWithIndex(
		t, libs.NewModelIndex(), "stated-end-is-constant.sysml", model,
	)
	statedIface := testSymbol(t, testSymbol(t, statedRoot, "P").Scope, "I")
	statedEnd := ElementOf(testSymbol(t, statedIface.Scope, "p"))
	structure := statedEvaluator.options.Structure.(testStructure)
	structure.attributes = map[testRelatedKey]Value{
		{element: statedEnd.Key(), property: "isConstant"}: booleanValue(false),
	}
	statedEvaluator.options.Structure = structure
	got = testProperty(t, statedEvaluator, statedEnd, "Feature", "isConstant")
	if got.Kind != BooleanValue || got.Boolean {
		t.Fatalf("stated end Feature::isConstant = %#v; want false", got)
	}
}

// KerML 1.1 Feature::namingFeature selects owned Redefinitions, not positional
// invocation parameters (lines 10475–10481).
func TestOperatorExpressionArgumentsDoNotInheritParameterNames(t *testing.T) {
	evaluator, root := newTestEvaluatorWithIndex(t, libs.NewModelIndex(), "operator-parameter-names.sysml", "package P {}")
	pkg := testSymbol(t, root, "P")
	operator := Element{
		Node:      &ast.OperatorExpr{Operator: ast.OpAdd},
		Container: pkg,
		Aspect:    "operator-expression",
	}
	function := evaluator.model.OperatorFunction(ast.OpAdd)
	if function == nil {
		t.Fatal("library '+' operator function is missing")
	}
	inputs := evaluator.model.InputParametersOf(function)
	result := evaluator.model.ResultParameterOf(function)
	if len(inputs) < 2 || result == nil {
		t.Fatalf("library '+' parameters = inputs %v, result %v; want two inputs and a result",
			inputs, result)
	}

	makePin := func(name, aspect, direction string) Element {
		pin := Element{Symbol: &symbols.Symbol{Name: name}}
		membership := MembershipElement(Membership{Aspect: aspect + "-membership", kind: "FeatureMembership"})
		structure := evaluator.options.Structure.(testStructure)
		if structure.metaclasses == nil {
			structure.metaclasses = make(map[ElementKey]string)
		}
		if structure.ownedRelationships == nil {
			structure.ownedRelationships = make(map[ElementKey][]Element)
		}
		if structure.owningRelationships == nil {
			structure.owningRelationships = make(map[ElementKey]testOptionalElement)
		}
		if structure.owningRelatedElements == nil {
			structure.owningRelatedElements = make(map[ElementKey]testOptionalElement)
		}
		if structure.relatedElements == nil {
			structure.relatedElements = make(map[testRelatedKey][]Element)
		}
		if structure.attributes == nil {
			structure.attributes = make(map[testRelatedKey]Value)
		}
		structure.metaclasses[operator.Key()] = "OperatorExpression"
		structure.metaclasses[pin.Key()] = "Feature"
		structure.metaclasses[membership.Key()] = "FeatureMembership"
		structure.ownedRelationships[operator.Key()] = append(structure.ownedRelationships[operator.Key()], membership)
		structure.owningRelationships[pin.Key()] = testOptionalElement{element: membership, present: true}
		structure.owningRelatedElements[membership.Key()] = testOptionalElement{element: operator, present: true}
		structure.relatedElements[testRelatedKey{element: membership.Key(), property: "memberElement"}] = []Element{pin}
		structure.attributes[testRelatedKey{element: pin.Key(), property: "direction"}] = enumValue(direction)
		evaluator.options.Structure = structure
		return pin
	}

	first := makePin("pin0", "first", "in")
	second := makePin("pin1", "second", "in")
	resultPin := makePin("pinResult", "result", "out")
	for _, pin := range []Element{first, second, resultPin} {
		got, handled, ok := evaluator.structureNamingFeature(pin)
		if !handled || !ok || got.Symbol != nil || got.Node != nil ||
			got.IsMembership || got.Aspect != "" {
			t.Errorf("namingFeature(%s) = %#v, %t; want no parameter-derived feature",
				pin.Symbol.Name, got, ok)
		}
	}
}

func TestFeatureChainExpressionOperatorIsDot(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "feature-chain-operator.sysml", "package P {}")
	chain := Element{Node: &ast.FeatureChainExpr{}}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{chain.Key(): "FeatureChainExpression"},
	}
	got := testProperty(t, evaluator, chain, "FeatureChainExpression", "operator")
	if got.Kind != StringValue || got.String != "." {
		t.Fatalf("FeatureChainExpression::operator = %#v; want %q", got, ".")
	}
}

func TestRequirementConstraintNamingUsesReferencedFeatureTarget(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "requirement-constraint-naming.sysml", `package P {
		part def Engine {
			part running;
		}
		part engine : Engine;
		requirement def R {
			require engine.running;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	engineDefinition := testSymbol(t, pkg.Scope, "Engine")
	engine := testSymbol(t, pkg.Scope, "engine")
	running := testSymbol(t, engineDefinition.Scope, "running")
	requirement := testSymbol(t, pkg.Scope, "R")
	structure := testNamespaceStructure(evaluator, pkg, requirement, engineDefinition, engine, running)
	evaluator.options.Structure = structure
	ownedMembers := testProperty(t, evaluator, ElementOf(requirement), "Type", "ownedFeature")
	var constraint Element
	for _, member := range ownedMembers.Values {
		if member.Kind == ElementValue && member.Element.Symbol != nil {
			constraint = member.Element
			break
		}
	}
	if constraint.Key() == (ElementKey{}) {
		t.Fatalf("requirement constraint membership was not found; ownedFeature=%#v members=%#v",
			ownedMembers, requirement.Scope.AllMembers())
	}
	namingFeature, handled, ok := evaluator.structureNamingFeature(constraint)
	if !handled || !ok || namingFeature.Symbol != running {
		t.Fatalf("RequirementConstraintMembership namingFeature = %#v, %t; want featureTarget %q", namingFeature, ok, running.Name)
	}
}

func TestRequirementConstraintNamingUsesGraphOwningMembership(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "requirement-constraint-graph-membership.sysml", `package P {
		part def Engine {
			part running;
		}
		part engine : Engine;
		requirement def R {
			require engine.running;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	engineDefinition := testSymbol(t, pkg.Scope, "Engine")
	engine := testSymbol(t, pkg.Scope, "engine")
	running := testSymbol(t, engineDefinition.Scope, "running")
	requirement := testSymbol(t, pkg.Scope, "R")
	structure := testNamespaceStructure(evaluator, pkg, requirement, engineDefinition, engine, running)
	evaluator.options.Structure = structure
	ownedFeatures := testProperty(t, evaluator, ElementOf(requirement), "Type", "ownedFeature")
	var constraint Element
	for _, feature := range ownedFeatures.Values {
		if feature.Kind == ElementValue && feature.Element.Symbol != nil {
			constraint = feature.Element
			break
		}
	}
	if constraint.Key() == (ElementKey{}) {
		t.Fatal("requirement constraint was not found")
	}
	constraint.Membership.kind = ""
	membership := MembershipElement(Membership{Aspect: "requirement-constraint-membership"})
	reference := Element{Aspect: "requirement-constraint-reference"}
	structure.metaclasses[membership.Key()] = "RequirementConstraintMembership"
	structure.metaclasses[reference.Key()] = "ReferenceSubsetting"
	structure.owningRelationships[constraint.Key()] = testOptionalElement{
		element: membership, present: true,
	}
	structure.ownedRelationships[constraint.Key()] = append(
		structure.ownedRelationships[constraint.Key()], reference,
	)
	structure.relatedElements[testRelatedKey{
		element: reference.Key(), property: "referencedFeature",
	}] = []Element{ElementOf(running)}
	evaluator.options.Structure = structure

	namingFeature, handled, ok := evaluator.structureNamingFeature(constraint)
	if !handled || !ok || namingFeature.Symbol != running {
		t.Fatalf("RequirementConstraintMembership namingFeature = %#v, %t; want featureTarget %q",
			namingFeature, ok, running.Name)
	}
}

func TestStructureQualifiedNameIsNullThroughUnnamedAncestor(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "unnamed-qualified-name.sysml", "package P {}")
	root := Element{Aspect: "root"}
	unnamed := Element{Aspect: "unnamed-namespace"}
	child := Element{Aspect: "named-child"}
	orphan := Element{Aspect: "element-without-namespace"}
	structure := testStructure{
		metaclasses:           make(map[ElementKey]string),
		ownedRelationships:    make(map[ElementKey][]Element),
		owningRelationships:   make(map[ElementKey]testOptionalElement),
		ownedRelatedElements:  make(map[ElementKey][]Element),
		owningRelatedElements: make(map[ElementKey]testOptionalElement),
		relatedElements:       make(map[testRelatedKey][]Element),
		attributes: map[testRelatedKey]Value{
			{element: child.Key(), property: "declaredName"}:   stringValue("Child"),
			{element: unnamed.Key(), property: "declaredName"}: nullValue(),
			{element: orphan.Key(), property: "declaredName"}:  stringValue("Orphan"),
		},
	}
	structure.metaclasses[root.Key()] = "Namespace"
	structure.metaclasses[unnamed.Key()] = "Namespace"
	structure.metaclasses[child.Key()] = "PartDefinition"
	structure.metaclasses[orphan.Key()] = "PartDefinition"
	structure.owningRelationships[root.Key()] = testOptionalElement{present: false}
	structure.owningRelatedElements[root.Key()] = testOptionalElement{present: false}
	addTestOwnedMembership(&structure, root, unnamed, "Namespace")
	addTestOwnedMembership(&structure, unnamed, child, "PartDefinition")
	evaluator.options.Structure = structure

	got, ok := evaluator.Property(child, "Element", "qualifiedName")
	if !ok || got.Kind != NullValue {
		t.Fatalf("Element::qualifiedName = %#v, %t; want null through an unnamed ancestor", got, ok)
	}
	got, ok = evaluator.Property(orphan, "Element", "qualifiedName")
	if !ok || got.Kind != NullValue {
		t.Fatalf("Element::qualifiedName without an owningNamespace = %#v, %t; want null", got, ok)
	}
}

func TestStructureNamesFollowRedefinitionAndReturnNullWhenUnnamed(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "structure-names.sysml", "package P {}")
	original := Element{Aspect: "original-feature"}
	redefined := Element{Aspect: "redefined-feature"}
	redefinition := Element{Aspect: "redefinition"}
	unnamed := Element{Aspect: "unnamed-feature"}
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			original.Key():     "PartUsage",
			redefined.Key():    "PartUsage",
			redefinition.Key(): "Redefinition",
			unnamed.Key():      "PartUsage",
		},
		ownedRelationships: map[ElementKey][]Element{
			redefined.Key(): {redefinition},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: redefinition.Key(), property: "redefinedFeature"}: {original},
		},
		attributes: map[testRelatedKey]Value{
			{element: original.Key(), property: "declaredName"}:      stringValue("source"),
			{element: original.Key(), property: "declaredShortName"}: stringValue("src"),
		},
	}
	evaluator.options.Structure = structure

	for _, test := range []struct {
		element  Element
		property string
		want     Value
	}{
		{element: redefined, property: "name", want: stringValue("source")},
		{element: redefined, property: "shortName", want: stringValue("src")},
		{element: unnamed, property: "name", want: nullValue()},
		{element: unnamed, property: "shortName", want: nullValue()},
	} {
		got, ok := evaluator.Property(test.element, "Element", test.property)
		if !ok || got.Kind != test.want.Kind || got.String != test.want.String {
			t.Errorf("Element::%s = %#v, %t; want %#v", test.property, got, ok, test.want)
		}
	}
}

func TestStructureVariantPropertiesSupportElementsWithoutSymbols(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "structure-variants.sysml", "package P {}")
	typeElement := Element{Aspect: "symbol-less-type"}
	membership := MembershipElement(Membership{Aspect: "symbol-less-variant", kind: "VariantMembership"})
	variant := Element{Aspect: "symbol-less-variant-usage"}
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			typeElement.Key(): "PartDefinition",
			membership.Key():  "VariantMembership",
			variant.Key():     "PartUsage",
		},
		ownedRelationships: map[ElementKey][]Element{
			typeElement.Key(): {membership},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: membership.Key(), property: "memberElement"}: {variant},
		},
	}
	evaluator.options.Structure = structure

	got, ok := evaluator.Property(typeElement, "Definition", "variantMembership")
	if !ok || len(got.Values) != 1 || got.Values[0].Kind != MembershipValue ||
		got.Values[0].Membership.Aspect != membership.Membership.Aspect {
		t.Fatalf("Definition::variantMembership = %#v, %t; want the Structure membership", got, ok)
	}
	got, ok = evaluator.Property(typeElement, "Definition", "variant")
	if !ok || len(got.Values) != 1 || got.Values[0].Kind != ElementValue ||
		got.Values[0].Element.Aspect != variant.Aspect {
		t.Fatalf("Definition::variant = %#v, %t; want the Structure member", got, ok)
	}
}

// SysML 2.1 derives variantMembership from ownedMembership only
// (lines 16768–16774 and 17304–17312).
func TestStructureVariantPropertiesExcludeInheritedMemberships(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "owned-variant-memberships.sysml", "package P {}")
	variation := Element{Aspect: "variation"}
	derived := Element{Aspect: "derived"}
	membership := MembershipElement(Membership{Aspect: "inherited-variant", kind: "VariantMembership"})
	variant := Element{Aspect: "variant-usage"}
	specialization := Element{Aspect: "derived-specialization"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			variation.Key():      "PartDefinition",
			derived.Key():        "PartDefinition",
			membership.Key():     "VariantMembership",
			variant.Key():        "PartUsage",
			specialization.Key(): "Specialization",
		},
		ownedRelationships: map[ElementKey][]Element{
			variation.Key(): {membership},
			derived.Key():   {specialization},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: membership.Key(), property: "memberElement"}: {variant},
			{element: specialization.Key(), property: "general"}:   {variation},
		},
	}

	for _, element := range []Element{variation, derived} {
		got, ok := evaluator.Property(element, "Definition", "variantMembership")
		if !ok {
			t.Fatalf("Definition::variantMembership for %s is undecidable", element.Aspect)
		}
		want := 0
		if element.Key() == variation.Key() {
			want = 1
		}
		if len(got.Values) != want {
			t.Errorf("%s variantMembership has %d values, want %d",
				element.Aspect, len(got.Values), want)
		}
		got, ok = evaluator.Property(element, "Definition", "variant")
		if !ok || len(got.Values) != want {
			t.Errorf("%s variant = %#v, %t; want %d owned variants",
				element.Aspect, got, ok, want)
		}
	}
}

func TestStructureOwnerRequiresOwningRelationship(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "owner-relationship.sysml", "package P {}")
	unowned := Element{Aspect: "unowned"}
	structure := testStructure{
		metaclasses:           map[ElementKey]string{unowned.Key(): "Element"},
		owningRelationships:   make(map[ElementKey]testOptionalElement),
		ownedRelatedElements:  make(map[ElementKey][]Element),
		owningRelatedElements: map[ElementKey]testOptionalElement{unowned.Key(): {}},
		relatedElements:       make(map[testRelatedKey][]Element),
		attributes:            make(map[testRelatedKey]Value),
	}
	evaluator.options.Structure = structure

	owner := testProperty(t, evaluator, unowned, "Element", "owner")
	if owner.Kind != NullValue {
		t.Fatalf("Element::owner = %#v, want null without owningRelationship", owner)
	}
}

func TestStructureRelationshipOwnerUsesOwningRelatedElement(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "owner-relationship.sysml", "package P {}")
	namespace := Element{Aspect: "namespace"}
	relationship := Element{Aspect: "relationship"}
	evaluator.options.Structure = testStructure{
		metaclasses:           map[ElementKey]string{relationship.Key(): "Membership"},
		owningRelationships:   make(map[ElementKey]testOptionalElement),
		ownedRelatedElements:  make(map[ElementKey][]Element),
		owningRelatedElements: map[ElementKey]testOptionalElement{relationship.Key(): {element: namespace, present: true}},
		relatedElements:       make(map[testRelatedKey][]Element),
		attributes:            make(map[testRelatedKey]Value),
	}

	owner := testProperty(t, evaluator, relationship, "Element", "owner")
	if owner.Kind != ElementValue || owner.Element.Key() != namespace.Key() {
		t.Fatalf("Element::owner = %#v, want relationship owningRelatedElement", owner)
	}
}

func TestStepParametersProjectOwnedParameterMemberships(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "step-parameters.sysml", "package P {}")
	step := Element{Aspect: "step"}
	parameter := Element{Aspect: "parameter"}
	membership := MembershipElement(Membership{Aspect: "parameter-membership"})
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			step.Key():       "Step",
			parameter.Key():  "Feature",
			membership.Key(): "ParameterMembership",
		},
		ownedRelationships: map[ElementKey][]Element{
			step.Key(): {membership},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: membership.Key(), property: "memberElement"}: {parameter},
		},
		owningRelationships:   make(map[ElementKey]testOptionalElement),
		ownedRelatedElements:  make(map[ElementKey][]Element),
		owningRelatedElements: make(map[ElementKey]testOptionalElement),
		attributes:            make(map[testRelatedKey]Value),
	}
	evaluator.options.Structure = structure

	got, ok := evaluator.Property(step, "Step", "parameter")
	if !ok || got.Kind != SequenceValue || len(got.Values) != 1 ||
		got.Values[0].Kind != ElementValue || got.Values[0].Element.Key() != parameter.Key() {
		t.Fatalf("Step::parameter = %#v, %t; want the owned ParameterMembership member", got, ok)
	}
}

func TestStructureBackedMembershipAndRelationshipProjections(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "membership-projections.sysml", "package P {}")
	expression := Element{Aspect: "feature-chain"}
	parameterMembership := MembershipElement(Membership{Aspect: "parameter-membership"})
	targetMembership := MembershipElement(Membership{Aspect: "target-membership"})
	targetFeature := Element{Aspect: "target-feature"}
	filterMembership := MembershipElement(Membership{Aspect: "filter-membership"})
	condition := Element{Aspect: "filter-condition"}
	transitionMembership := MembershipElement(Membership{Aspect: "transition-membership"})
	transitionFeature := Element{Aspect: "transition-feature"}
	variantMembership := MembershipElement(Membership{Aspect: "variant-membership"})
	variantUsage := Element{Aspect: "variant-usage"}
	portDefinition := Element{Aspect: "port-definition"}
	conjugatedPortDefinition := Element{Aspect: "conjugated-port-definition"}
	conjugatedTyping := Element{Aspect: "conjugated-port-typing"}
	portMembership := MembershipElement(Membership{Aspect: "conjugated-port-membership"})
	portConjugation := Element{Aspect: "port-conjugation"}
	portFeature := Element{Aspect: "port-feature"}
	subclassification := Element{Aspect: "subclassification"}
	classifier := Element{Aspect: "classifier"}
	multiplicity := Element{Aspect: "multiplicity"}
	lowerMembership := MembershipElement(Membership{Aspect: "lower-bound-membership"})
	upperMembership := MembershipElement(Membership{Aspect: "upper-bound-membership"})
	lowerBound := Element{Aspect: "lower-bound"}
	upperBound := Element{Aspect: "upper-bound"}
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			expression.Key():               "FeatureChainExpression",
			parameterMembership.Key():      "ParameterMembership",
			targetMembership.Key():         "FeatureMembership",
			targetFeature.Key():            "Feature",
			filterMembership.Key():         "ElementFilterMembership",
			condition.Key():                "Expression",
			transitionMembership.Key():     "TransitionFeatureMembership",
			transitionFeature.Key():        "Step",
			variantMembership.Key():        "VariantMembership",
			variantUsage.Key():             "PartUsage",
			portDefinition.Key():           "PortDefinition",
			conjugatedPortDefinition.Key(): "ConjugatedPortDefinition",
			conjugatedTyping.Key():         "ConjugatedPortTyping",
			portMembership.Key():           "OwningMembership",
			portConjugation.Key():          "PortConjugation",
			portFeature.Key():              "Feature",
			subclassification.Key():        "Subclassification",
			classifier.Key():               "Classifier",
			multiplicity.Key():             "MultiplicityRange",
			lowerMembership.Key():          "OwningMembership",
			upperMembership.Key():          "OwningMembership",
			lowerBound.Key():               "Expression",
			upperBound.Key():               "Expression",
		},
		ownedRelationships: map[ElementKey][]Element{
			expression.Key():               {parameterMembership, targetMembership},
			portDefinition.Key():           {portMembership},
			conjugatedPortDefinition.Key(): {portConjugation},
			multiplicity.Key():             {lowerMembership, upperMembership},
		},
		owningRelationships: map[ElementKey]testOptionalElement{
			conjugatedPortDefinition.Key(): {element: portMembership, present: true},
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			subclassification.Key(): {element: classifier, present: true},
			portMembership.Key():    {element: portDefinition, present: true},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: parameterMembership.Key(), property: "memberElement"}:         {Element{Aspect: "parameter"}},
			{element: targetMembership.Key(), property: "memberElement"}:            {targetFeature},
			{element: filterMembership.Key(), property: "memberElement"}:            {condition},
			{element: transitionMembership.Key(), property: "memberElement"}:        {transitionFeature},
			{element: variantMembership.Key(), property: "memberElement"}:           {variantUsage},
			{element: portMembership.Key(), property: "memberElement"}:              {conjugatedPortDefinition},
			{element: conjugatedTyping.Key(), property: "conjugatedPortDefinition"}: {conjugatedPortDefinition},
			{element: lowerMembership.Key(), property: "memberElement"}:             {lowerBound},
			{element: upperMembership.Key(), property: "memberElement"}:             {upperBound},
		},
		attributes: map[testRelatedKey]Value{
			{element: portDefinition.Key(), property: "declaredName"}: stringValue("port"),
			{element: portFeature.Key(), property: "direction"}:       enumValue(ast.DirIn.String()),
		},
	}
	evaluator.options.Structure = structure

	if got := testProperty(t, evaluator, expression, "FeatureChainExpression", "targetFeature"); got.Kind != ElementValue ||
		got.Element.Key() != targetFeature.Key() {
		t.Fatalf("FeatureChainExpression::targetFeature = %#v; want first non-parameter member", got)
	}
	if got := testProperty(t, evaluator, filterMembership, "ElementFilterMembership", "condition"); got.Kind != ElementValue ||
		got.Element.Key() != condition.Key() {
		t.Fatalf("ElementFilterMembership::condition = %#v; want member Expression", got)
	}
	if got := testProperty(t, evaluator, transitionMembership, "TransitionFeatureMembership", "transitionFeature"); got.Kind != ElementValue ||
		got.Element.Key() != transitionFeature.Key() {
		t.Fatalf("TransitionFeatureMembership::transitionFeature = %#v; want member Step", got)
	}
	if got := testProperty(t, evaluator, variantMembership, "VariantMembership", "ownedVariantUsage"); got.Kind != ElementValue ||
		got.Element.Key() != variantUsage.Key() {
		t.Fatalf("VariantMembership::ownedVariantUsage = %#v; want member Usage", got)
	}
	conjugator := testProperty(t, evaluator, conjugatedPortDefinition, "ConjugatedPortDefinition", "ownedPortConjugator")
	if conjugator.Kind != ElementValue || conjugator.Element.Key() != portConjugation.Key() {
		t.Fatalf("ConjugatedPortDefinition::ownedPortConjugator = %#v; want owned PortConjugation", conjugator)
	}
	conjugated := testProperty(t, evaluator, portDefinition, "PortDefinition", "conjugatedPortDefinition")
	if conjugated.Kind != ElementValue || conjugated.Element.Key() != conjugatedPortDefinition.Key() {
		t.Fatalf("PortDefinition::conjugatedPortDefinition = %#v; want owned ConjugatedPortDefinition", conjugated)
	}
	owningNamespace := testProperty(t, evaluator, conjugatedPortDefinition, "Element", "owningNamespace")
	if owningNamespace.Kind != ElementValue || owningNamespace.Element.Key() != portDefinition.Key() {
		t.Fatalf("ConjugatedPortDefinition::owningNamespace = %#v; want parent PortDefinition", owningNamespace)
	}
	originalPortDefinition := testProperty(t, evaluator, conjugatedPortDefinition, "ConjugatedPortDefinition", "originalPortDefinition")
	if originalPortDefinition.Kind != ElementValue || originalPortDefinition.Element.Key() != portDefinition.Key() {
		t.Fatalf("ConjugatedPortDefinition::originalPortDefinition = %#v; want parent PortDefinition", originalPortDefinition)
	}
	if got := testProperty(t, evaluator, conjugatedPortDefinition, "Element", "name"); got.Kind != StringValue || got.String != "~port" {
		t.Fatalf("ConjugatedPortDefinition::name = %#v; want %q", got, "~port")
	}
	typingTargets, targetsOK := evaluator.relationshipEndpoints(conjugatedTyping, "conjugatedPortDefinition")
	if !targetsOK || len(typingTargets) != 1 || typingTargets[0].Kind != ElementValue ||
		typingTargets[0].Element.Key() != conjugatedPortDefinition.Key() {
		t.Fatalf("ConjugatedPortTyping conjugatedPortDefinition endpoints = %#v, %t", typingTargets, targetsOK)
	}
	filteredTypingTargets, filterOK := evaluator.structureFilterValues(typingTargets, "ConjugatedPortDefinition")
	if !filterOK || len(filteredTypingTargets) != 1 {
		t.Fatalf("ConjugatedPortTyping filtered definition endpoints = %#v, %t", filteredTypingTargets, filterOK)
	}
	if got := testProperty(t, evaluator, conjugatedTyping, "ConjugatedPortTyping", "portDefinition"); got.Kind != ElementValue ||
		got.Element.Key() != portDefinition.Key() {
		t.Fatalf("ConjugatedPortTyping::portDefinition = %#v; want original PortDefinition", got)
	}
	if direction, known := evaluator.structureEffectiveDirection(conjugatedPortDefinition, portFeature); !known ||
		direction != ast.DirOut {
		t.Fatalf("conjugated port feature direction = %v, %t; want out", direction, known)
	}
	if got := testProperty(t, evaluator, subclassification, "Subclassification", "owningClassifier"); got.Kind != ElementValue ||
		got.Element.Key() != classifier.Key() {
		t.Fatalf("Subclassification::owningClassifier = %#v; want its owning Classifier", got)
	}
	if got := testProperty(t, evaluator, multiplicity, "MultiplicityRange", "lowerBound"); got.Kind != ElementValue ||
		got.Element.Key() != lowerBound.Key() {
		t.Fatalf("MultiplicityRange::lowerBound = %#v; want first owned Expression", got)
	}
	if got := testProperty(t, evaluator, multiplicity, "MultiplicityRange", "upperBound"); got.Kind != ElementValue ||
		got.Element.Key() != upperBound.Key() {
		t.Fatalf("MultiplicityRange::upperBound = %#v; want second owned Expression", got)
	}
}

func TestStructureBackedInvocationArguments(t *testing.T) {
	evaluator, root := newTestEvaluatorWithIndex(t, libs.NewModelIndex(), "invocation-arguments.sysml", `package P {
    function f { in x; }
    attribute call = f(1);
}`)
	pkg := testSymbol(t, root, "P")
	function := testSymbol(t, pkg.Scope, "f")
	input := testSymbol(t, function.Scope, "x")
	call := testSymbol(t, pkg.Scope, "call")
	callUsage, ok := call.Decl.(*ast.Usage)
	if !ok {
		t.Fatalf("call declaration = %T, want *ast.Usage", call.Decl)
	}
	var invocationNode *ast.InvocationExpr
	ast.Inspect(callUsage.Value, func(node ast.Node) bool {
		if invocation, ok := node.(*ast.InvocationExpr); ok {
			invocationNode = invocation
		}
		return true
	})
	if invocationNode == nil {
		t.Fatal("call value has no invocation expression")
	}
	invocation := Element{Node: invocationNode, Container: pkg, Aspect: "invocation"}
	operandNode := &ast.FeatureReference{}
	invocationWithOperand := Element{
		Node:      &ast.InvocationExpr{Operand: operandNode},
		Container: pkg,
		Aspect:    "invocation-with-operand",
	}
	operandElement := Element{Node: operandNode, Container: pkg}
	instantiatedType := ElementOf(function)
	inputFeature := ElementOf(input)
	calleeMembership := MembershipElement(Membership{Aspect: "callee-membership", kind: "OwningMembership"})
	inputMembership := MembershipElement(inputFeature.Membership)
	argumentFeature := Element{
		Symbol: &symbols.Symbol{Kind: symbols.SymbolAttributeUsage, Name: "argument"},
	}
	argumentMembership := MembershipElement(Membership{Aspect: "argument-membership"})
	redefinition := Element{Aspect: "argument-redefinition"}
	featureValue := MembershipElement(Membership{Aspect: "argument-value-membership"})
	argument := Element{Aspect: "argument-expression"}
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			invocation.Key():            "InvocationExpression",
			invocationWithOperand.Key(): "InvocationExpression",
			operandElement.Key():        "FeatureReferenceExpression",
			instantiatedType.Key():      "Function",
			calleeMembership.Key():      "OwningMembership",
			inputFeature.Key():          "Feature",
			inputMembership.Key():       "ParameterMembership",
			argumentFeature.Key():       "Feature",
			argumentMembership.Key():    "FeatureMembership",
			redefinition.Key():          "Redefinition",
			featureValue.Key():          "FeatureValue",
			argument.Key():              "Expression",
		},
		ownedRelationships: map[ElementKey][]Element{
			invocation.Key():            {calleeMembership, argumentMembership},
			invocationWithOperand.Key(): {},
			instantiatedType.Key():      {inputMembership},
			calleeMembership.Key():      {},
			inputFeature.Key():          {},
			argumentFeature.Key():       {redefinition, featureValue},
			argument.Key():              {},
		},
		owningRelationships: map[ElementKey]testOptionalElement{
			argumentFeature.Key(): {element: argumentMembership, present: true},
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			argumentMembership.Key(): {element: invocation, present: true},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: calleeMembership.Key(), property: "memberElement"}:   {instantiatedType},
			{element: inputMembership.Key(), property: "memberElement"}:    {inputFeature},
			{element: argumentMembership.Key(), property: "memberElement"}: {argumentFeature},
			{element: redefinition.Key(), property: "redefinedFeature"}:    {inputFeature},
			{element: featureValue.Key(), property: "memberElement"}:       {argument},
			{element: invocation.Key(), property: "operand"}:               {},
		},
		attributes: map[testRelatedKey]Value{
			{element: inputFeature.Key(), property: "direction"}:    enumValue(ast.DirIn.String()),
			{element: argumentFeature.Key(), property: "direction"}: enumValue(ast.DirIn.String()),
		},
	}
	evaluator.options.Structure = structure

	got := testProperty(t, evaluator, invocation, "InstantiationExpression", "argument")
	if got.Kind != SequenceValue || len(got.Values) != 1 || got.Values[0].Kind != ElementValue ||
		got.Values[0].Element.Key() != argument.Key() {
		t.Fatalf("InstantiationExpression::argument = %#v; want the argument Expression in input order", got)
	}
	namingFeature, handled, ok := evaluator.structureNamingFeature(argumentFeature)
	if !handled || !ok || namingFeature.Symbol != input {
		t.Fatalf("invocation argument namingFeature = %#v, %t; want parameter %q",
			namingFeature, ok, input.Name)
	}
	operands := testProperty(t, evaluator, invocation, "InvocationExpression", "operand")
	if operands.Kind != SequenceValue || len(operands.Values) != 0 {
		t.Fatalf("InvocationExpression::operand = %#v; want an empty sequence for a call without an operand", operands)
	}
	operands = testProperty(t, evaluator, invocationWithOperand, "InvocationExpression", "operand")
	if operands.Kind != SequenceValue || len(operands.Values) != 1 ||
		operands.Values[0].Kind != ElementValue || operands.Values[0].Element.Key() != operandElement.Key() {
		t.Fatalf("InvocationExpression::operand = %#v; want its AST FeatureReferenceExpression", operands)
	}
}

func TestInstantiationExpressionResolvesOperatorAndTriggerTypes(t *testing.T) {
	evaluator, root := newTestEvaluatorWithIndex(t, libs.NewModelIndex(), "instantiation-types.sysml", "package P {}")
	pkg := testSymbol(t, root, "P")
	operator := Element{
		Node:      &ast.OperatorExpr{Operator: ast.OpAdd, Operands: []ast.Node{&ast.LiteralInteger{}, &ast.LiteralInteger{}}},
		Container: pkg,
		Aspect:    "operator-expression",
	}
	operatorType, ok := evaluator.instantiationExpressionProperty(operator, "instantiatedType")
	if !ok || operatorType.Kind != ElementValue || operatorType.Element.Symbol == nil ||
		operatorType.Element.Symbol != evaluator.model.OperatorFunction(ast.OpAdd) {
		t.Fatalf("OperatorExpression::instantiatedType = %#v, %t; want the library '+' function", operatorType, ok)
	}
	wrappedOperator := Element{
		Node:      &ast.ConstraintMember{Expression: operator.Node},
		Container: pkg,
		Aspect:    "wrapped-operator-expression",
	}
	wrappedType, ok := evaluator.instantiationExpressionProperty(wrappedOperator, "instantiatedType")
	if !ok || wrappedType.Kind != ElementValue || wrappedType.Element.Symbol != operatorType.Element.Symbol {
		t.Fatalf("wrapped OperatorExpression::instantiatedType = %#v, %t; want the library '+' function", wrappedType, ok)
	}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{wrappedOperator.Key(): "OperatorExpression"},
	}
	functionValue := testProperty(t, evaluator, wrappedOperator, "Expression", "function")
	if functionValue.Kind != ElementValue || functionValue.Element.Symbol != operatorType.Element.Symbol {
		t.Fatalf("wrapped OperatorExpression::function = %#v; want the library '+' function", functionValue)
	}
	evaluator.options.Structure = testStructure{}
	inputs, ok := evaluator.Property(operatorType.Element, "Type", "input")
	wantInputs := evaluator.model.InputParametersOf(operatorType.Element.Symbol)
	if !ok || inputs.Kind != SequenceValue || len(inputs.Values) != len(wantInputs) || len(wantInputs) == 0 {
		t.Fatalf("library Type::input = %#v, %t; want the loaded operator's input parameters", inputs, ok)
	}
	for index, input := range inputs.Values {
		if input.Kind != ElementValue || input.Element.Symbol != wantInputs[index] {
			t.Fatalf("library Type::input[%d] = %#v; want %s", index, input, symbols.FQNOf(wantInputs[index]))
		}
	}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{operatorType.Element.Key(): "Function"},
	}
	inputs, ok = evaluator.Property(operatorType.Element, "Type", "input")
	if !ok || inputs.Kind != SequenceValue || len(inputs.Values) != len(wantInputs) {
		t.Fatalf("library-stub Type::input = %#v, %t; want the loaded operator's input parameters", inputs, ok)
	}
	for index, input := range inputs.Values {
		if input.Kind != ElementValue || input.Element.Symbol != wantInputs[index] {
			t.Fatalf("library-stub Type::input[%d] = %#v; want %s", index, input, symbols.FQNOf(wantInputs[index]))
		}
	}
	features, ok := evaluator.Property(operatorType.Element, "Type", "feature")
	if !ok || features.Kind != SequenceValue {
		t.Fatalf("library Type::feature = %#v, %t; want the loaded operator features", features, ok)
	}

	trigger := Element{Container: pkg, Aspect: "trigger-expression"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{trigger.Key(): "TriggerInvocationExpression"},
		attributes: map[testRelatedKey]Value{
			{element: trigger.Key(), property: "kind"}: stringValue("when"),
		},
	}
	triggerType, ok := evaluator.instantiationExpressionProperty(trigger, "instantiatedType")
	if !ok || triggerType.Kind != ElementValue || triggerType.Element.Symbol == nil ||
		triggerType.Element.Symbol.Name != "TriggerWhen" {
		t.Fatalf("TriggerInvocationExpression::instantiatedType = %#v, %t; want Triggers::TriggerWhen", triggerType, ok)
	}

	globalTrigger := Element{Aspect: "global-trigger-expression"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{globalTrigger.Key(): "TriggerInvocationExpression"},
		attributes: map[testRelatedKey]Value{
			{element: globalTrigger.Key(), property: "kind"}: stringValue("when"),
		},
	}
	triggerType, ok = evaluator.instantiationExpressionProperty(globalTrigger, "instantiatedType")
	if !ok || triggerType.Kind != ElementValue || triggerType.Element.Symbol == nil ||
		triggerType.Element.Symbol.Name != "TriggerWhen" {
		t.Fatalf("global TriggerInvocationExpression::instantiatedType = %#v, %t; want Triggers::TriggerWhen", triggerType, ok)
	}
}

func TestStructureEffectiveDirectionUsesSemanticParameterMembership(t *testing.T) {
	evaluator, _ := newTestEvaluatorWithIndex(
		t, libs.NewModelIndex(), "semantic-parameter-direction.sysml", "package P {}",
	)
	function := evaluator.model.OperatorFunction(ast.OpAdd)
	inputs := evaluator.model.InputParametersOf(function)
	if len(inputs) == 0 {
		t.Fatal("loaded operator has no input parameters")
	}
	evaluator.options.Structure = testStructure{}

	direction, ok := evaluator.structureEffectiveDirection(
		Element{Aspect: "unmapped-expression"}, ElementOf(inputs[0]),
	)
	if !ok || direction != ast.DirIn {
		t.Fatalf("effective direction = %v, %t; want in from semantic membership", direction, ok)
	}

	actions := evaluator.resolver.Index().LookupQualified("Actions::Action")
	if len(actions) == 0 {
		t.Fatal("Actions::Action is absent from the loaded library")
	}
	action := testSymbol(t, actions[0].Scope, "self")
	direction, ok = evaluator.structureEffectiveDirection(
		Element{Aspect: "unmapped-action-usage"}, ElementOf(action),
	)
	if !ok || direction != ast.DirNone {
		t.Fatalf("effective direction = %v, %t; want none for Actions::Action::self", direction, ok)
	}
}

func TestExhibitStateNamingUsesItsExhibitedState(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "exhibit-state-name.sysml", `package P {
    state def Idle;
    part def Vehicle {
        exhibit idling : Idle;
    }
}`)
	pkg := testSymbol(t, root, "P")
	vehicle := testSymbol(t, pkg.Scope, "Vehicle")
	exhibit := ElementOf(testSymbol(t, vehicle.Scope, "idling"))
	structure := evaluator.options.Structure.(testStructure)
	structure.metaclasses[exhibit.Key()] = "ExhibitStateUsage"
	structure.ownedRelationships[exhibit.Key()] = nil
	evaluator.options.Structure = structure
	target, handled, ok := evaluator.structurePerformedAction(exhibit)
	if !handled || !ok || target.Kind != ElementValue || target.Element.Symbol != exhibit.Symbol {
		t.Fatalf("ExhibitStateUsage::exhibitedState = %#v, %t; want the usage itself", target, ok)
	}

	declared := exhibit.Symbol.Decl.(*ast.Usage)
	namedDecl := *declared
	namedDecl.Ident.Name = "idling"
	namedSymbol := *exhibit.Symbol
	namedSymbol.Naming = symbols.NamedByRedefinition
	namedSymbol.Decl = &namedDecl
	named := ElementOf(&namedSymbol)
	structure.metaclasses[named.Key()] = "ExhibitStateUsage"
	structure.ownedRelationships[named.Key()] = nil
	evaluator.options.Structure = structure
	declaredName, handled, ok := evaluator.structureName(named, "name")
	if !handled || !ok || declaredName.Kind != StringValue || declaredName.String != "idling" {
		t.Fatalf("Element::name = %#v, %t; want the declared exhibit name", declaredName, ok)
	}

	unnamedDecl := *declared
	unnamedDecl.Ident = ast.Identification{}
	unnamedSymbol := *exhibit.Symbol
	unnamedSymbol.Name = ""
	unnamedSymbol.Naming = symbols.NamedByReference
	unnamedSymbol.Decl = &unnamedDecl
	unnamed := ElementOf(&unnamedSymbol)
	structure.metaclasses[unnamed.Key()] = "ExhibitStateUsage"
	structure.ownedRelationships[unnamed.Key()] = nil
	evaluator.options.Structure = structure

	name, handled, ok := evaluator.structureName(unnamed, "name")
	if !handled || !ok || name.Kind != NullValue {
		t.Fatalf("Element::name = %#v, %t; want null when namingFeature returns self", name, ok)
	}
}

func TestInstantiationExpressionUsesFirstNonFeatureMembership(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "instantiation-owned-type.sysml", `package P {
    function f { in x; }
}`)
	pkg := testSymbol(t, root, "P")
	function := testSymbol(t, pkg.Scope, "f")
	expression := Element{
		Node:   &ast.InvocationExpr{},
		Aspect: "invocation",
	}
	result := Element{Aspect: "result"}
	resultMembership := MembershipElement(Membership{Aspect: "result-membership"})
	calleeMembership := MembershipElement(Membership{Aspect: "callee-membership"})
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			expression.Key():          "InvocationExpression",
			result.Key():              "Feature",
			resultMembership.Key():    "ReturnParameterMembership",
			calleeMembership.Key():    "Membership",
			ElementOf(function).Key(): "Function",
		},
		ownedRelationships: map[ElementKey][]Element{
			expression.Key(): {resultMembership, calleeMembership},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: resultMembership.Key(), property: "memberElement"}: {result},
			{element: calleeMembership.Key(), property: "memberElement"}: {ElementOf(function)},
		},
	}

	got, ok := evaluator.instantiationExpressionProperty(expression, "instantiatedType")
	if !ok || got.Kind != ElementValue || got.Element.Symbol != function {
		t.Fatalf("InvocationExpression::instantiatedType = %#v, %t; want the Type member of the first non-FeatureMembership", got, ok)
	}
}

func TestInstantiationExpressionResolvesChainInvocationType(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "chain-invocation-type.sysml", "package P { function f {} }")
	pkg := testSymbol(t, root, "P")
	function := testSymbol(t, pkg.Scope, "f")
	target := Element{Aspect: "callable-feature"}
	typing := Element{Aspect: "callable-feature-typing"}
	chain := &ast.FeatureChainExpr{}
	invocation := Element{
		Node:      &ast.InvocationExpr{Operand: chain},
		Container: pkg,
		Aspect:    "chain-invocation",
	}
	chainElement := Element{Node: chain, Container: pkg}
	functionElement := ElementOf(function)
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			invocation.Key():      "InvocationExpression",
			chainElement.Key():    "FeatureChainExpression",
			target.Key():          "Feature",
			typing.Key():          "FeatureTyping",
			functionElement.Key(): "Function",
		},
		ownedRelationships: map[ElementKey][]Element{
			target.Key(): {typing},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: chainElement.Key(), property: "targetFeature"}: {target},
			{element: typing.Key(), property: "type"}:                {functionElement},
		},
		statedRelated: map[testRelatedKey]bool{
			{element: chainElement.Key(), property: "targetFeature"}: true,
		},
	}

	got, ok := evaluator.instantiationExpressionProperty(invocation, "instantiatedType")
	if !ok || got.Kind != ElementValue || got.Element.Symbol != function {
		t.Fatalf("InvocationExpression::instantiatedType = %#v, %t; want the Function type of its chain callee", got, ok)
	}
}

func TestInstantiationExpressionResolvesInvocationWithOnlyFeatureMemberships(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "instantiation-invocation-target.sysml", `package P {
    function f {}
}`)
	pkg := testSymbol(t, root, "P")
	function := testSymbol(t, pkg.Scope, "f")
	invocation := Element{
		Node:      &ast.InvocationExpr{Type: ast.QualifiedNameOf("f")},
		Container: pkg,
		Aspect:    "invocation",
	}
	parameterMembership := MembershipElement(Membership{
		Aspect:  "parameter-membership",
		Feature: true,
		kind:    "ParameterMembership",
	})
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			invocation.Key():          "InvocationExpression",
			parameterMembership.Key(): "ParameterMembership",
		},
		ownedRelationships: map[ElementKey][]Element{
			invocation.Key(): {parameterMembership},
		},
	}

	got, ok := evaluator.instantiationExpressionProperty(invocation, "instantiatedType")
	if !ok || got.Kind != ElementValue || got.Element.Symbol != function {
		t.Fatalf("InvocationExpression::instantiatedType = %#v, %t; want resolved invocation target", got, ok)
	}
}

func TestStructureBackedActionArguments(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "action-arguments.sysml", "package P {}")
	accept := Element{Aspect: "accept"}
	send := Element{Aspect: "send"}
	payloadArgument := Element{Aspect: "payload-argument"}
	receiverArgument := Element{Aspect: "receiver-argument"}
	senderArgument := Element{Aspect: "sender-argument"}
	sendPayloadArgument := Element{Aspect: "send-payload-argument"}
	sendReceiverArgument := Element{Aspect: "send-receiver-argument"}
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			accept.Key(): "AcceptActionUsage",
			send.Key():   "SendActionUsage",
		},
		ownedRelationships:   make(map[ElementKey][]Element),
		ownedRelatedElements: make(map[ElementKey][]Element),
		relatedElements:      make(map[testRelatedKey][]Element),
		attributes:           make(map[testRelatedKey]Value),
	}
	addInputs := func(owner Element, prefix string, arguments ...Element) {
		for _, argument := range arguments {
			feature := Element{Aspect: prefix + "-" + argument.Aspect + "-input"}
			membership := MembershipElement(Membership{
				Aspect:  prefix + "-" + argument.Aspect + "-membership",
				Feature: true,
				kind:    "ParameterMembership",
			})
			valuation := Element{Aspect: prefix + "-" + argument.Aspect + "-valuation"}
			structure.metaclasses[feature.Key()] = "Feature"
			structure.metaclasses[membership.Key()] = "ParameterMembership"
			structure.metaclasses[valuation.Key()] = "FeatureValue"
			structure.metaclasses[argument.Key()] = "Expression"
			structure.ownedRelationships[owner.Key()] =
				append(structure.ownedRelationships[owner.Key()], membership)
			structure.ownedRelationships[feature.Key()] = []Element{valuation}
			structure.ownedRelatedElements[membership.Key()] = []Element{feature}
			structure.relatedElements[testRelatedKey{element: valuation.Key(), property: "memberElement"}] =
				[]Element{argument}
			structure.attributes[testRelatedKey{element: feature.Key(), property: "direction"}] =
				enumValue(ast.DirIn.String())
		}
	}
	addInputs(accept, "accept", payloadArgument, receiverArgument)
	addInputs(send, "send", sendPayloadArgument, senderArgument, sendReceiverArgument)
	evaluator.options.Structure = structure

	for _, test := range []struct {
		element  Element
		class    string
		property string
		want     Element
	}{
		{accept, "AcceptActionUsage", "payloadArgument", payloadArgument},
		{accept, "AcceptActionUsage", "receiverArgument", receiverArgument},
		{send, "SendActionUsage", "payloadArgument", sendPayloadArgument},
		{send, "SendActionUsage", "senderArgument", senderArgument},
		{send, "SendActionUsage", "receiverArgument", sendReceiverArgument},
	} {
		got := testProperty(t, evaluator, test.element, test.class, test.property)
		if got.Kind != ElementValue || got.Element.Key() != test.want.Key() {
			t.Errorf("%s::%s = %#v; want FeatureValue of input %s", test.class, test.property, got, test.want.Aspect)
		}
	}
}

// KerML §8.3.3.1 derives Type::input from directed features; a SendActionUsage
// contributes both its owned payload and receiver input arguments (lines 9553–9562).
func TestStructureSendActionInputIncludesPayloadAndReceiver(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "send-action-input.sysml", "package P {}")
	send := Element{Aspect: "send"}
	payload := Element{Aspect: "payload"}
	receiver := Element{Aspect: "receiver"}
	payloadMembership := MembershipElement(Membership{Aspect: "payload-membership", kind: "OwningMembership"})
	receiverMembership := MembershipElement(Membership{Aspect: "receiver-membership", kind: "OwningMembership"})
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			send.Key():               "SendActionUsage",
			payload.Key():            "Expression",
			receiver.Key():           "Expression",
			payloadMembership.Key():  "OwningMembership",
			receiverMembership.Key(): "OwningMembership",
		},
		ownedRelationships: map[ElementKey][]Element{
			send.Key(): {payloadMembership, receiverMembership},
		},
		ownedRelatedElements: map[ElementKey][]Element{
			payloadMembership.Key():  {payload},
			receiverMembership.Key(): {receiver},
		},
	}

	got := testProperty(t, evaluator, send, "Type", "input")
	if got.Kind != SequenceValue || len(got.Values) != 2 ||
		got.Values[0].Kind != ElementValue || got.Values[0].Element.Key() != payload.Key() ||
		got.Values[1].Kind != ElementValue || got.Values[1].Element.Key() != receiver.Key() {
		t.Fatalf("SendActionUsage::input = %#v; want payload then receiver", got)
	}
}

func TestStructureOwningNamespaceRequiresOwningMembership(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "owning-namespace-membership.sysml", "package P {}")
	namespace := Element{Aspect: "namespace"}
	element := Element{Aspect: "element"}
	structure := testStructure{
		metaclasses:           map[ElementKey]string{element.Key(): "PartUsage", namespace.Key(): "Namespace"},
		ownedRelationships:    make(map[ElementKey][]Element),
		owningRelationships:   make(map[ElementKey]testOptionalElement),
		ownedRelatedElements:  make(map[ElementKey][]Element),
		owningRelatedElements: map[ElementKey]testOptionalElement{element.Key(): {element: namespace, present: true}},
		relatedElements:       make(map[testRelatedKey][]Element),
		attributes:            map[testRelatedKey]Value{{element: element.Key(), property: "declaredName"}: stringValue("orphan")},
	}
	evaluator.options.Structure = structure

	owningNamespace := testProperty(t, evaluator, element, "Element", "owningNamespace")
	if owningNamespace.Kind != NullValue {
		t.Fatalf("Element::owningNamespace = %#v, want null without owningMembership", owningNamespace)
	}
	qualifiedName := testProperty(t, evaluator, element, "Element", "qualifiedName")
	if qualifiedName.Kind != NullValue {
		t.Fatalf("Element::qualifiedName = %#v, want null without owningNamespace", qualifiedName)
	}
}

func TestStructureQualifiedNameUsesRedefinedFeaturesEffectiveName(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "redefined-qualified-name.sysml", `part def A {
		part <p1> original;
	}
	part def B specializes A {
		part :>> original;
	}`)
	base := testSymbol(t, root, "A")
	owner := testSymbol(t, root, "B")
	redefined := testSymbol(t, owner.Scope, "original")
	if redefined == testSymbol(t, base.Scope, "original") {
		t.Fatal("redefined feature resolved to the base feature")
	}
	if got := evaluator.model.EffectiveNameOf(redefined); got != "original" {
		t.Fatalf("EffectiveNameOf(redefined) = %q; want original", got)
	}
	structure := testDocumentStructure(evaluator, root, base, owner)
	addTestSemanticRelationships(evaluator, &structure, redefined)
	evaluator.options.Structure = structure

	got := testProperty(t, evaluator, ElementOf(redefined), "Element", "qualifiedName")
	if got.Kind != StringValue || got.String != "B::original" {
		element := ElementOf(redefined)
		name := testProperty(t, evaluator, element, "Element", "name")
		namespace := testProperty(t, evaluator, element, "Element", "owningNamespace")
		members := testProperty(t, evaluator, ElementOf(owner), "Namespace", "ownedMember")
		t.Fatalf("Element::qualifiedName = %#v; want %q (name=%#v, owningNamespace=%#v, owner.ownedMember=%#v)",
			got, "B::original", name, namespace, members)
	}
}

func TestStructureNameUsesDeclaredNameWhenSymbolNameIsUnavailable(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "structure-declared-name.sysml", "package P {}")
	element := Element{Symbol: &symbols.Symbol{}}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			element.Key(): "PartDefinition",
		},
		attributes: map[testRelatedKey]Value{
			{element: element.Key(), property: "declaredName"}: stringValue("ScalarValues"),
		},
	}

	got := testProperty(t, evaluator, element, "Element", "name")
	if got.Kind != StringValue || got.String != "ScalarValues" {
		t.Fatalf("Element::name = %#v; want declared name %q", got, "ScalarValues")
	}
}

func TestStructureNameUsesDeclaredNameBeforeRedefinition(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "declared-redefinition-name.sysml", `part def A {
		part original;
	}
	part def B specializes A {
		part renamed redefines original;
	}`)
	base := testSymbol(t, root, "A")
	owner := testSymbol(t, root, "B")
	renamed := testSymbol(t, owner.Scope, "renamed")
	evaluator.options.Structure = testDocumentStructure(evaluator, root, base, owner)

	if got := testProperty(t, evaluator, ElementOf(renamed), "Element", "name"); got.Kind != StringValue || got.String != "renamed" {
		t.Fatalf("Element::name = %#v; want declared name %q", got, "renamed")
	}
}

func TestStructureNameUsesDeclaredNameForRedefinedFeature(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "named-redefined-feature.sysml", `package P {
		part def Base {
			part b;
		}
		part def Sub specializes Base;
		part def Other specializes Sub {
			part r redefines b;
		}
	}`)
	base := testSymbol(t, testSymbol(t, root, "P").Scope, "Base")
	sub := testSymbol(t, testSymbol(t, root, "P").Scope, "Sub")
	other := testSymbol(t, testSymbol(t, root, "P").Scope, "Other")
	renamed := testSymbol(t, other.Scope, "r")
	evaluator.options.Structure = testDocumentStructure(evaluator, root, base, sub, other)

	if got := testProperty(t, evaluator, ElementOf(renamed), "Element", "name"); got.Kind != StringValue || got.String != "r" {
		t.Fatalf("Element::name = %#v; want declared name %q", got, "r")
	}
}

func TestStructureDeclaredNamePrecedesUnresolvedRedefinitionNaming(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "unresolved-implied-name.sysml", `part def A {
		part member;
	}`)
	owner := testSymbol(t, root, "A")
	member := testSymbol(t, owner.Scope, "member")
	member.Naming = symbols.NamedByRedefinition
	element := ElementOf(member)
	for _, property := range []struct {
		class string
		name  string
		want  Value
	}{
		{class: "Element", name: "name", want: stringValue("member")},
		{class: "Element", name: "qualifiedName", want: stringValue("member")},
	} {
		value, ok := evaluator.Property(element, property.class, property.name)
		if !ok || value.Kind != property.want.Kind ||
			(value.Kind == StringValue && value.String != property.want.String) {
			t.Errorf("%s::%s = %#v, %t; want %#v", property.class, property.name, value, ok, property.want)
		}
	}
	if _, ok := evaluator.Property(element, "Element", "shortName"); ok {
		t.Fatal("Element::shortName was emitted without a resolved naming feature")
	}
	if got, ok := evaluator.Explain(element, "Element", "shortName"); !ok ||
		got.Category != ReasonUnresolvedReference {
		t.Fatalf("Explain for unresolved shortName = %#v, %t; want unresolved-reference", got, ok)
	}
}

func TestStructureBackedRulesDoNotGuessWhenGraphFactsAreMissing(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "missing-structure.sysml", "package P {}")
	owner := Element{Aspect: "owner"}
	withoutStructure := New(evaluator.resolver, evaluator.model)
	if _, ok := withoutStructure.Property(owner, "Type", "ownedFeature"); ok {
		t.Fatal("Type::ownedFeature succeeded without Structure")
	}
	for _, property := range []struct {
		class string
		name  string
	}{
		{class: "Element", name: "name"},
		{class: "Relationship", name: "source"},
		{class: "Feature", name: "owningType"},
	} {
		if _, ok := withoutStructure.Property(owner, property.class, property.name); ok {
			t.Fatalf("%s::%s succeeded without Structure", property.class, property.name)
		}
	}
	evaluator = New(evaluator.resolver, evaluator.model, Options{Structure: testStructure{
		metaclasses:           map[ElementKey]string{owner.Key(): "PartDefinition"},
		unknownOwnedRelations: map[ElementKey]bool{owner.Key(): true},
	}})
	if _, ok := evaluator.Property(owner, "Type", "ownedFeature"); ok {
		t.Fatal("Type::ownedFeature succeeded without graph-owned relationship facts")
	}
}

func TestStructureBackedFiltersOmitUnknownMetaclasses(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "unknown-metaclass.sysml", "package P {}")
	owner := Element{Aspect: "owner"}
	membership := MembershipElement(Membership{Aspect: "membership"})
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			owner.Key(): "Type",
		},
		ownedRelationships: map[ElementKey][]Element{
			owner.Key(): {membership},
		},
	}
	evaluator.options.Structure = structure
	if _, ok := evaluator.Property(owner, "Type", "ownedFeature"); ok {
		t.Fatal("Type::ownedFeature guessed an unknown owned relationship metaclass")
	}
}

func TestFeatureChainingProjectsGraphStatedEndpoints(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "feature-chaining-endpoints.sysml", "package P {}")
	relationship := Element{Aspect: "feature-chaining"}
	source := Element{Aspect: "feature-chaining-source"}
	target := Element{Aspect: "feature-chaining-target"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			relationship.Key(): "FeatureChaining",
			source.Key():       "Feature",
			target.Key():       "Feature",
		},
		ownedRelationships: map[ElementKey][]Element{
			relationship.Key(): {},
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			relationship.Key(): {element: source, present: true},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: relationship.Key(), property: "chainingFeature"}: {target},
			{element: relationship.Key(), property: "relatedElement"}:  {target},
		},
	}

	chained := testProperty(t, evaluator, relationship, "FeatureChaining", "featureChained")
	if chained.Kind != ElementValue || chained.Element.Key() != source.Key() {
		t.Fatalf("FeatureChaining::featureChained = %#v, want owning source %#v", chained, source)
	}
	related := valueElements(t, testProperty(t, evaluator, relationship, "Relationship", "relatedElement"))
	if len(related) != 2 || related[0].Key() != source.Key() || related[1].Key() != target.Key() {
		t.Fatalf("Relationship::relatedElement = %#v, want source then target", related)
	}
}

func TestStructureOwnedMemberUsesOwningMembershipAndTypeMultiplicity(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "owned-member-projection.sysml", "package P {}")
	typeElement := Element{Aspect: "type"}
	multiplicity := Element{Aspect: "multiplicity"}
	unownedMember := Element{Aspect: "feature-membership-member"}
	owningMembership := MembershipElement(Membership{Aspect: "multiplicity-membership", kind: "OwningMembership"})
	featureMembership := MembershipElement(Membership{Aspect: "feature-membership", kind: "FeatureMembership"})
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			typeElement.Key():       "PartDefinition",
			multiplicity.Key():      "Multiplicity",
			unownedMember.Key():     "Feature",
			owningMembership.Key():  "OwningMembership",
			featureMembership.Key(): "FeatureMembership",
		},
		ownedRelationships: map[ElementKey][]Element{
			typeElement.Key():       {owningMembership, featureMembership},
			owningMembership.Key():  {},
			featureMembership.Key(): {},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: owningMembership.Key(), property: "memberElement"}:  {multiplicity},
			{element: featureMembership.Key(), property: "memberElement"}: {unownedMember},
		},
	}

	owned := valueElements(t, testProperty(t, evaluator, typeElement, "Namespace", "ownedMember"))
	if len(owned) != 2 || owned[0].Key() != multiplicity.Key() || owned[1].Key() != unownedMember.Key() {
		t.Fatalf("Namespace::ownedMember = %#v, want both OwningMembership members", owned)
	}
	got := testProperty(t, evaluator, typeElement, "Type", "multiplicity")
	if got.Kind != ElementValue || got.Element.Key() != multiplicity.Key() {
		t.Fatalf("Type::multiplicity = %#v, want the owned Multiplicity", got)
	}
}

func TestFeatureChainTargetFeatureUsesResolvedGraphEndpoint(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "feature-chain-graph-target.sysml", "package P {}")
	chain := Element{Aspect: "feature-chain"}
	target := Element{Aspect: "resolved-chain-target"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			chain.Key():  "FeatureChainExpression",
			target.Key(): "Feature",
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: chain.Key(), property: "targetFeature"}: {target},
		},
		statedRelated: map[testRelatedKey]bool{
			{element: chain.Key(), property: "targetFeature"}: true,
		},
	}

	got := testProperty(t, evaluator, chain, "FeatureChainExpression", "targetFeature")
	if got.Kind != ElementValue || got.Element.Key() != target.Key() {
		t.Fatalf("FeatureChainExpression::targetFeature = %#v; want resolved graph target", got)
	}
}

func TestConnectorRelatedFeaturesUseStructureEnds(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "connector-related-features.sysml", "package P {}")
	connector := Element{Aspect: "connector"}
	firstEnd := Element{Aspect: "connector-end-first"}
	secondEnd := Element{Aspect: "connector-end-second"}
	firstMembership := MembershipElement(Membership{Aspect: "connector-end-membership-first"})
	secondMembership := MembershipElement(Membership{Aspect: "connector-end-membership-second"})
	firstReference := Element{Aspect: "connector-end-reference-first"}
	secondReference := Element{Aspect: "connector-end-reference-second"}
	firstTarget := Element{Aspect: "connector-target-first"}
	secondTarget := Element{Aspect: "connector-target-second"}
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			connector.Key():        "ConnectorAsUsage",
			firstEnd.Key():         "Feature",
			secondEnd.Key():        "Feature",
			firstMembership.Key():  "EndFeatureMembership",
			secondMembership.Key(): "EndFeatureMembership",
			firstReference.Key():   "ReferenceSubsetting",
			secondReference.Key():  "ReferenceSubsetting",
			firstTarget.Key():      "Feature",
			secondTarget.Key():     "Feature",
		},
		ownedRelationships: map[ElementKey][]Element{
			connector.Key(): {firstMembership, secondMembership},
			firstEnd.Key():  {firstReference},
			secondEnd.Key(): {secondReference},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: firstMembership.Key(), property: "memberElement"}:     {firstEnd},
			{element: secondMembership.Key(), property: "memberElement"}:    {secondEnd},
			{element: firstReference.Key(), property: "referencedFeature"}:  {firstTarget},
			{element: secondReference.Key(), property: "referencedFeature"}: {secondTarget},
		},
		attributes: map[testRelatedKey]Value{
			{element: firstEnd.Key(), property: "isEnd"}:  booleanValue(true),
			{element: secondEnd.Key(), property: "isEnd"}: booleanValue(true),
		},
	}
	evaluator.options.Structure = structure

	related := valueElements(t, testProperty(t, evaluator, connector, "Connector", "relatedFeature"))
	if len(related) != 2 || related[0].Key() != firstTarget.Key() || related[1].Key() != secondTarget.Key() {
		t.Fatalf("Connector::relatedFeature = %#v, want referenced targets in end order", related)
	}
	source := testProperty(t, evaluator, connector, "Connector", "sourceFeature")
	if source.Kind != ElementValue || source.Element.Key() != firstTarget.Key() {
		t.Fatalf("Connector::sourceFeature = %#v, want first referenced target", source)
	}
	target := valueElements(t, testProperty(t, evaluator, connector, "Connector", "targetFeature"))
	if len(target) != 1 || target[0].Key() != secondTarget.Key() {
		t.Fatalf("Connector::targetFeature = %#v, want remaining referenced target", target)
	}
}

func TestConnectorRelatedFeaturesOmitWhenThereAreNoOwnedEnds(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "connector-without-ends.sysml", "package P {}")
	connector := Element{Aspect: "connector-without-owned-ends"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			connector.Key(): "ConnectorAsUsage",
		},
		ownedRelationships: map[ElementKey][]Element{
			connector.Key(): {},
		},
	}

	if _, ok := evaluator.Property(connector, "Connector", "relatedFeature"); ok {
		t.Fatal("Connector::relatedFeature was derived despite having no owned ends")
	}
	reason, ok := evaluator.Explain(connector, "Connector", "relatedFeature")
	if !ok || reason.Category != ReasonMissingStructure {
		t.Fatalf("Connector::relatedFeature reason = %#v, %t; want typed missing-structure omission",
			reason, ok)
	}
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

func TestConnectorRelatedFeaturesIgnoreInheritedEnds(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "connector-with-inherited-ends.sysml", `package P {
		part def A;
		part def B;
		connection def Base { end a : A; end b : B; }
		connection def Derived :> Base;
	}`)
	pkg := testSymbol(t, root, "P")
	derived := testSymbol(t, pkg.Scope, "Derived")
	if ends := evaluator.model.EndFeatures(derived); len(ends) != 2 {
		t.Fatalf("Derived effective ends = %v; want its two inherited ends", ends)
	}

	connector := ElementOf(derived)
	if _, ok := evaluator.Property(connector, "Connector", "relatedFeature"); ok {
		t.Fatal("Connector::relatedFeature was derived from inherited ends without owned ends")
	}
	reason, ok := evaluator.Explain(connector, "Connector", "relatedFeature")
	if !ok || reason.Category != ReasonMissingStructure {
		t.Fatalf("Connector::relatedFeature reason = %#v, %t; want typed missing-structure omission",
			reason, ok)
	}
}

// Connector::relatedFeature follows owned ends' ownedReferenceSubsetting targets
// (SysML.uml:3987–3995; ReferenceSubsetting::referencedFeature redefines subsettedFeature at 6172).
func TestConnectorRelatedFeaturesUseOnlyOwnedReferenceSubsettings(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "connector-related-features.sysml", "package P {}")
	connector := Element{Aspect: "connector-stated-related-features"}
	firstEnd := Element{Aspect: "connector-end-without-subsetting"}
	secondEnd := Element{Aspect: "connector-end-with-subsetting"}
	membership1 := MembershipElement(Membership{Aspect: "connector-end-membership-first", kind: "EndFeatureMembership"})
	membership2 := MembershipElement(Membership{Aspect: "connector-end-membership-second", kind: "EndFeatureMembership"})
	reference := Element{Aspect: "connector-end-reference"}
	first := Element{Aspect: "graph-stated-related-feature"}
	second := Element{Aspect: "referenced-feature"}
	third := Element{Aspect: "graph-stated-endpoint-only"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			connector.Key():   "FlowUsage",
			firstEnd.Key():    "FlowEnd",
			secondEnd.Key():   "FlowEnd",
			membership1.Key(): "EndFeatureMembership",
			membership2.Key(): "EndFeatureMembership",
			reference.Key():   "ReferenceSubsetting",
			first.Key():       "Feature",
			second.Key():      "Feature",
			third.Key():       "Feature",
		},
		ownedRelationships: map[ElementKey][]Element{
			connector.Key(): {membership1, membership2},
			secondEnd.Key(): {reference},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: membership1.Key(), property: "memberElement"}:   {firstEnd},
			{element: membership2.Key(), property: "memberElement"}:   {secondEnd},
			{element: reference.Key(), property: "referencedFeature"}: {second},
			{element: connector.Key(), property: "relatedFeature"}:    {first},
			{element: connector.Key(), property: "source"}:            {first},
			{element: connector.Key(), property: "target"}:            {third},
		},
		statedRelated: map[testRelatedKey]bool{
			{element: connector.Key(), property: "relatedFeature"}: true,
			{element: connector.Key(), property: "source"}:         true,
			{element: connector.Key(), property: "target"}:         true,
		},
	}

	related := valueElements(t, testProperty(t, evaluator, connector, "Connector", "relatedFeature"))
	if len(related) != 1 || related[0].Key() != second.Key() {
		t.Fatalf("Connector::relatedFeature = %#v, want only the owned referencedFeature", related)
	}
	source := testProperty(t, evaluator, connector, "Connector", "sourceFeature")
	if source.Kind != ElementValue || source.Element.Key() != second.Key() {
		t.Fatalf("Connector::sourceFeature = %#v, want first relatedFeature", source)
	}
	target := valueElements(t, testProperty(t, evaluator, connector, "Connector", "targetFeature"))
	if len(target) != 0 {
		t.Fatalf("Connector::targetFeature = %#v, want no remaining related features", target)
	}
}

func TestStructureConnectorEndsResolveAnonymousOwnedEnds(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "anonymous-connector-ends.sysml", `package P {
		part def Part;
		part x : Part;
		part y : Part;
		connection c connect x to y;
	}`)
	pkg := testSymbol(t, root, "P")
	connector := ElementOf(testSymbol(t, pkg.Scope, "c"))
	semanticEnds := evaluator.model.EndFeatures(connector.Symbol)
	if len(semanticEnds) != 2 {
		t.Fatalf("semantic connector ends = %#v, want two ends", semanticEnds)
	}

	firstEnd := Element{Aspect: "first-anonymous-end"}
	secondEnd := Element{Aspect: "second-anonymous-end"}
	firstMembership := MembershipElement(Membership{Aspect: "first-anonymous-end-membership", kind: "EndFeatureMembership"})
	secondMembership := MembershipElement(Membership{Aspect: "second-anonymous-end-membership", kind: "EndFeatureMembership"})
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			connector.Key():        "ConnectorAsUsage",
			firstEnd.Key():         "Feature",
			secondEnd.Key():        "Feature",
			firstMembership.Key():  "EndFeatureMembership",
			secondMembership.Key(): "EndFeatureMembership",
		},
		ownedRelationships: map[ElementKey][]Element{
			connector.Key(): {firstMembership, secondMembership},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: firstMembership.Key(), property: "memberElement"}:  {firstEnd},
			{element: secondMembership.Key(), property: "memberElement"}: {secondEnd},
		},
		attributes: map[testRelatedKey]Value{
			{element: firstEnd.Key(), property: "isEnd"}:  booleanValue(true),
			{element: secondEnd.Key(), property: "isEnd"}: booleanValue(true),
		},
	}

	ends := valueElements(t, testProperty(t, evaluator, connector, "Connector", "connectorEnd"))
	if len(ends) != 2 || ends[0].Key() != firstEnd.Key() || ends[1].Key() != secondEnd.Key() {
		t.Fatalf("Connector::connectorEnd = %#v, want the two graph-backed anonymous ends", ends)
	}
}

func TestStructureFlowEndsRedefineInheritedConnectorEnds(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "flow-connector-ends.sysml", `package P {
		flow def Transfer {
			end item source;
			end item target;
		}
		item x;
		item y;
		flow f : Transfer from x to y;
	}`)
	pkg := testSymbol(t, root, "P")
	flowSymbol := testSymbol(t, pkg.Scope, "f")
	semanticEnds := evaluator.model.EndFeatures(flowSymbol)
	if len(semanticEnds) != 2 || semanticEnds[0] == nil || semanticEnds[0].Name != "source" ||
		semanticEnds[1] == nil || semanticEnds[1].Name != "target" {
		t.Fatalf("semantic flow ends = %#v, want the inherited source and target features", semanticEnds)
	}

	flow := ElementOf(flowSymbol)
	firstEnd := Element{Aspect: "flow-end-source"}
	secondEnd := Element{Aspect: "flow-end-target"}
	firstMembership := MembershipElement(Membership{Aspect: "flow-end-membership-source", kind: "FeatureMembership"})
	secondMembership := MembershipElement(Membership{Aspect: "flow-end-membership-target", kind: "FeatureMembership"})
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			flow.Key():             "FlowUsage",
			firstEnd.Key():         "FlowEnd",
			secondEnd.Key():        "FlowEnd",
			firstMembership.Key():  "FeatureMembership",
			secondMembership.Key(): "FeatureMembership",
		},
		ownedRelationships: map[ElementKey][]Element{
			flow.Key(): {firstMembership, secondMembership},
		},
		owningRelationships: map[ElementKey]testOptionalElement{
			firstEnd.Key():  {element: firstMembership, present: true},
			secondEnd.Key(): {element: secondMembership, present: true},
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			firstMembership.Key():  {element: flow, present: true},
			secondMembership.Key(): {element: flow, present: true},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: firstMembership.Key(), property: "memberElement"}:  {firstEnd},
			{element: secondMembership.Key(), property: "memberElement"}: {secondEnd},
		},
		attributes: map[testRelatedKey]Value{
			{element: firstEnd.Key(), property: "isEnd"}:  booleanValue(true),
			{element: secondEnd.Key(), property: "isEnd"}: booleanValue(true),
		},
	}

	ends := valueElements(t, testProperty(t, evaluator, flow, "Connector", "connectorEnd"))
	if len(ends) != 2 || ends[0].Key() != firstEnd.Key() || ends[1].Key() != secondEnd.Key() {
		t.Fatalf("FlowUsage::connectorEnd = %#v, want the two graph-backed flow ends", ends)
	}
}

// KerML Feature::namingFeature is null without an owned Redefinition, and effectiveName uses it when no declared name exists (KerML_only.uml:4209–4221).
func TestInheritedConnectorEndWithoutRedefinitionHasNoName(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "unnamed-inherited-connector-ends.sysml", `package P {
		flow def Transfer {
			end item source;
			end item target;
		}
		item x;
		item y;
		flow f : Transfer from x to y;
	}`)
	pkg := testSymbol(t, root, "P")
	flowSymbol := testSymbol(t, pkg.Scope, "f")
	flow := ElementOf(flowSymbol)
	ends := []Element{
		{Aspect: "unnamed-inherited-flow-end-source"},
		{Aspect: "unnamed-inherited-flow-end-target"},
	}
	memberships := []Element{
		MembershipElement(Membership{Aspect: "unnamed-inherited-flow-membership-source", kind: "FeatureMembership"}),
		MembershipElement(Membership{Aspect: "unnamed-inherited-flow-membership-target", kind: "FeatureMembership"}),
	}
	metaclasses := map[ElementKey]string{
		flow.Key(): "FlowUsage",
	}
	ownedRelationships := map[ElementKey][]Element{flow.Key(): {}}
	owningRelationships := map[ElementKey]testOptionalElement{}
	owningRelatedElements := map[ElementKey]testOptionalElement{}
	relatedElements := map[testRelatedKey][]Element{}
	attributes := map[testRelatedKey]Value{}
	for i, end := range ends {
		membership := memberships[i]
		metaclasses[end.Key()] = "Feature"
		metaclasses[membership.Key()] = "FeatureMembership"
		ownedRelationships[flow.Key()] = append(ownedRelationships[flow.Key()], membership)
		ownedRelationships[end.Key()] = []Element{}
		owningRelationships[end.Key()] = testOptionalElement{element: membership, present: true}
		owningRelatedElements[membership.Key()] = testOptionalElement{element: flow, present: true}
		relatedElements[testRelatedKey{element: membership.Key(), property: "memberElement"}] = []Element{end}
		attributes[testRelatedKey{element: end.Key(), property: "isEnd"}] = booleanValue(true)
	}
	evaluator.options.Structure = testStructure{
		metaclasses:           metaclasses,
		ownedRelationships:    ownedRelationships,
		owningRelationships:   owningRelationships,
		owningRelatedElements: owningRelatedElements,
		relatedElements:       relatedElements,
		attributes:            attributes,
	}

	for _, end := range ends {
		for _, property := range []string{"name", "shortName"} {
			got := testProperty(t, evaluator, end, "Element", property)
			if got.Kind != NullValue {
				t.Errorf("inherited connector end %s = %#v, want null without an owned Redefinition", property, got)
			}
		}
	}
}

func TestStructureFlowUsageEndsRedefineStandardLibraryEnds(t *testing.T) {
	evaluator, root := newTestEvaluatorWithIndex(t, libs.NewModelIndex(), "library-flow-connector-ends.sysml", `package P {
		item x { feature i; }
		item y { feature j; }
		flow f from x.i to y.j;
	}`)
	pkg := testSymbol(t, root, "P")
	flowSymbol := testSymbol(t, pkg.Scope, "f")
	usage := flowSymbol.Decl.(*ast.Usage)
	if usage.FlowEnds == nil || usage.FlowEnds.From == nil || usage.FlowEnds.To == nil {
		t.Fatalf("flow syntax has no source and target: %#v", usage.FlowEnds)
	}
	semanticEnds := evaluator.model.EndFeatures(flowSymbol)
	ownedSemanticEnds, hasOwnedEnds := evaluator.model.OwnedConnectorEnds(flowSymbol)
	flow := ElementOf(flowSymbol)
	if len(semanticEnds) != 2 || semanticEnds[0] == nil || semanticEnds[1] == nil {
		t.Fatalf("semantic flow ends = %#v, want the two inherited library ends", semanticEnds)
	}
	firstEnd := Element{Aspect: "library-flow-end-source"}
	secondEnd := Element{Aspect: "library-flow-end-target"}
	firstMembership := MembershipElement(Membership{Aspect: "library-flow-membership-source", kind: "EndFeatureMembership"})
	secondMembership := MembershipElement(Membership{Aspect: "library-flow-membership-target", kind: "EndFeatureMembership"})
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			flow.Key():             "FlowUsage",
			firstEnd.Key():         "FlowEnd",
			secondEnd.Key():        "FlowEnd",
			firstMembership.Key():  "EndFeatureMembership",
			secondMembership.Key(): "EndFeatureMembership",
		},
		ownedRelationships: map[ElementKey][]Element{
			flow.Key(): {firstMembership, secondMembership},
		},
		owningRelationships: map[ElementKey]testOptionalElement{
			firstEnd.Key():  {element: firstMembership, present: true},
			secondEnd.Key(): {element: secondMembership, present: true},
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			firstMembership.Key():  {element: flow, present: true},
			secondMembership.Key(): {element: flow, present: true},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: firstMembership.Key(), property: "memberElement"}:  {firstEnd},
			{element: secondMembership.Key(), property: "memberElement"}: {secondEnd},
		},
		attributes: map[testRelatedKey]Value{
			{element: firstEnd.Key(), property: "isEnd"}:  booleanValue(true),
			{element: secondEnd.Key(), property: "isEnd"}: booleanValue(true),
		},
	}

	ends := valueElements(t, testProperty(t, evaluator, flow, "Connector", "connectorEnd"))
	if len(ends) != 2 || ends[0].Key() != firstEnd.Key() || ends[1].Key() != secondEnd.Key() {
		names := func(features []*symbols.Symbol) []string {
			out := make([]string, len(features))
			for index, feature := range features {
				if feature != nil {
					out[index] = symbols.FQNOf(feature)
				}
			}
			return out
		}
		t.Fatalf("FlowUsage::connectorEnd = %#v, want graph ends; EndFeatures=%v OwnedConnectorEnds=(%v, %t)",
			ends, names(semanticEnds), names(ownedSemanticEnds), hasOwnedEnds)
	}
	for index, end := range []Element{firstEnd, secondEnd} {
		for _, property := range []string{"name", "shortName"} {
			if _, handled, ok := evaluator.structureName(end, property); !handled || ok {
				t.Errorf("FlowEnd[%d] %s is handled=%t, ok=%t; want unsupported until its implied Redefinition is materialized",
					index, property, handled, ok)
			}
			reason, ok := evaluator.Explain(end, "Element", property)
			if !ok || reason.Category != ReasonImpliedRelationshipNotMaterialized {
				t.Errorf("FlowEnd[%d] %s omission reason = %#v, %t; want implied-relationship-not-materialized",
					index, property, reason, ok)
			}
		}
	}
	for _, membership := range []Element{firstMembership, secondMembership} {
		if _, handled, ok := evaluator.structureMemberName(membership, "", "shortName"); !handled || ok {
			t.Errorf("%s::ownedMemberShortName is handled=%t, ok=%t; want unsupported while the member's implied Redefinition is absent",
				membership.Membership.Metaclass(), handled, ok)
		}
	}
}

func TestStructureFlowEndsOmitUnrepresentedLibraryEndpointsWithExplicitFlowEnds(t *testing.T) {
	evaluator, root := newTestEvaluatorWithIndex(t, libs.NewModelIndex(), "unrepresented-library-flow-ends.sysml", `package P {
		item x { feature i; }
		item y { feature j; }
		flow f from x.i to y.j;
	}`)
	pkg := testSymbol(t, root, "P")
	flow := ElementOf(testSymbol(t, pkg.Scope, "f"))
	features := evaluator.model.EndFeatures(flow.Symbol)
	if len(features) != 2 || !evaluator.resolver.Index().Library(features[0]) ||
		!evaluator.resolver.Index().Library(features[1]) {
		t.Fatalf("flow semantic ends = %#v, want two library ends", features)
	}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			flow.Key(): "FlowUsage",
		},
		ownedRelationships: map[ElementKey][]Element{
			flow.Key(): {},
		},
	}

	value, handled := evaluator.Property(flow, "Connector", "connectorEnd")
	if handled || value.Success {
		t.Fatalf("FlowUsage::connectorEnd = %#v, %t; want unsupported for library ends absent from Structure", value, handled)
	}
}

func TestOperatorInvocationArgumentsBindPositionally(t *testing.T) {
	evaluator, root := newTestEvaluatorWithIndex(
		t, libs.NewModelIndex(), "operator-argument-memberships.sysml", "package P {}",
	)
	pkg := testSymbol(t, root, "P")
	operator := Element{
		Node:      &ast.OperatorExpr{Operator: ast.OpAdd, Operands: []ast.Node{&ast.LiteralInteger{}, &ast.LiteralInteger{}}},
		Container: pkg,
		Aspect:    "operator-expression",
	}
	operatorFunction := evaluator.model.OperatorFunction(ast.OpAdd)
	if operatorFunction == nil {
		t.Fatal("the loaded library has no '+' operator function")
	}
	inputs := evaluator.model.InputParametersOf(operatorFunction)
	if len(inputs) < 2 {
		t.Fatalf("operator inputs = %v, want at least two", inputs)
	}

	metaclasses := map[ElementKey]string{
		operator.Key():                    "OperatorExpression",
		ElementOf(operatorFunction).Key(): "Function",
	}
	ownedRelationships := map[ElementKey][]Element{operator.Key(): {}}
	owningRelationships := make(map[ElementKey]testOptionalElement)
	relatedElements := make(map[testRelatedKey][]Element)
	for index := range inputs[:2] {
		suffix := string(rune('0' + index))
		feature := Element{Aspect: "operator-input-" + suffix}
		membership := MembershipElement(Membership{
			Aspect:  "operator-input-membership-" + suffix,
			Feature: true,
			kind:    "ParameterMembership",
		})
		valuation := MembershipElement(Membership{Aspect: "operator-input-valuation-" + suffix, kind: "FeatureValue"})
		argument := Element{Aspect: "operator-input-expression-" + suffix}
		metaclasses[feature.Key()] = "Feature"
		metaclasses[membership.Key()] = "ParameterMembership"
		metaclasses[valuation.Key()] = "FeatureValue"
		metaclasses[argument.Key()] = "Expression"
		ownedRelationships[operator.Key()] = append(ownedRelationships[operator.Key()], membership)
		ownedRelationships[feature.Key()] = []Element{valuation}
		owningRelationships[feature.Key()] = testOptionalElement{element: membership, present: true}
		relatedElements[testRelatedKey{element: membership.Key(), property: "memberElement"}] = []Element{feature}
		relatedElements[testRelatedKey{element: valuation.Key(), property: "memberElement"}] = []Element{argument}
	}
	evaluator.options.Structure = testStructure{
		metaclasses:         metaclasses,
		ownedRelationships:  ownedRelationships,
		owningRelationships: owningRelationships,
		relatedElements:     relatedElements,
		unknownOwnedRelations: map[ElementKey]bool{
			ElementOf(operatorFunction).Key(): true,
		},
	}

	instantiatedType, ok := evaluator.Property(operator, "InstantiationExpression", "instantiatedType")
	if !ok || instantiatedType.Kind != ElementValue {
		t.Fatalf("OperatorExpression::instantiatedType = %#v, %t", instantiatedType, ok)
	}
	inputValues, ok := evaluator.Property(instantiatedType.Element, "Type", "input")
	if !ok || inputValues.Kind != SequenceValue || len(inputValues.Values) < 2 {
		t.Fatalf("operator function Type::input = %#v, %t", inputValues, ok)
	}
	ownedFeatures, handled, ok := evaluator.structureOwnedFeatures(operator)
	if !handled || !ok || len(ownedFeatures.Values) != 2 {
		t.Fatalf("OperatorExpression owned features = %#v, handled=%t, supported=%t", ownedFeatures, handled, ok)
	}
	operatorInputs, ok := evaluator.structureOperatorInputs(operator, ownedFeatures.Values)
	if !ok || len(operatorInputs) != 2 {
		t.Fatalf("OperatorExpression input features = %#v, supported=%t", operatorInputs, ok)
	}
	if _, known := evaluator.isMetaclass(operator, "ConstructorExpression"); !known {
		t.Fatal("OperatorExpression metaclass is not classifiable")
	}
	for _, feature := range ownedFeatures.Values {
		redefinitions, handled, ok := evaluator.structureOwnedFilter(feature.Element, "Redefinition", false)
		if !handled || !ok || len(redefinitions.Values) != 0 {
			t.Fatalf("operator input Redefinition = %#v, handled=%t, supported=%t", redefinitions, handled, ok)
		}
		valuations, handled, ok := evaluator.structureOwnedFilter(feature.Element, "FeatureValue", false)
		if !handled || !ok || len(valuations.Values) != 1 {
			t.Fatalf("operator input valuation = %#v, handled=%t, supported=%t", valuations, handled, ok)
		}
		valuation := MembershipElement(valuations.Values[0].Membership)
		values, ok := evaluator.relationshipEndpoints(valuation, "value")
		if !ok {
			t.Fatalf("operator input valuation value is unavailable: %#v", valuation)
		}
		filtered, ok := evaluator.structureFilterValues(values, "Expression")
		if !ok || len(filtered) != 1 {
			t.Fatalf("operator input valuation value = %#v, supported=%t", filtered, ok)
		}
	}

	arguments, ok := evaluator.Property(operator, "InstantiationExpression", "argument")
	if !ok || arguments.Kind != SequenceValue || len(arguments.Values) != 2 {
		t.Fatalf("OperatorExpression::argument = %#v, %t; want two positional values", arguments, ok)
	}
	for index, value := range arguments.Values {
		if value.Kind != ElementValue || value.Element.Aspect != "operator-input-expression-"+string(rune('0'+index)) {
			t.Fatalf("OperatorExpression::argument[%d] = %#v; want positional expression", index, value)
		}
	}
}

func TestFeatureChainInvocationArgumentsBindPositionally(t *testing.T) {
	evaluator, root := newTestEvaluatorWithIndex(
		t, libs.NewModelIndex(), "feature-chain-arguments.sysml", "package P {}",
	)
	pkg := testSymbol(t, root, "P")
	chain := Element{
		Node:      &ast.FeatureChainExpr{Operand: &ast.LiteralInteger{}, Member: ast.QualifiedNameOf("member")},
		Container: pkg,
		Aspect:    "feature-chain-expression",
	}
	function := evaluator.model.OperatorFunctionNamed(".")
	if function == nil || symbols.FQNOf(function) != "ControlFunctions::." {
		t.Fatalf("feature-chain function = %v; want ControlFunctions::.", function)
	}
	inputs := evaluator.model.InputParametersOf(function)
	if len(inputs) != 1 {
		t.Fatalf("dot function inputs = %v; want one source input", inputs)
	}

	input := Element{Aspect: "feature-chain-input"}
	inputMembership := MembershipElement(Membership{
		Aspect:  "feature-chain-input-membership",
		Feature: true,
		kind:    "ParameterMembership",
	})
	valuation := MembershipElement(Membership{Aspect: "feature-chain-input-valuation", kind: "FeatureValue"})
	argument := Element{Aspect: "feature-chain-argument"}
	metaclasses := map[ElementKey]string{
		chain.Key():               "FeatureChainExpression",
		ElementOf(function).Key(): "Function",
		input.Key():               "Feature",
		inputMembership.Key():     "ParameterMembership",
		valuation.Key():           "FeatureValue",
		argument.Key():            "Expression",
	}
	evaluator.options.Structure = testStructure{
		metaclasses: metaclasses,
		ownedRelationships: map[ElementKey][]Element{
			chain.Key(): {inputMembership},
			input.Key(): {valuation},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: inputMembership.Key(), property: "memberElement"}: {input},
			{element: valuation.Key(), property: "memberElement"}:       {argument},
		},
		attributes: map[testRelatedKey]Value{
			{element: input.Key(), property: "direction"}: enumValue(ast.DirIn.String()),
		},
		unknownOwnedRelations: map[ElementKey]bool{
			ElementOf(function).Key(): true,
		},
	}

	instantiatedType, ok := evaluator.Property(chain, "InstantiationExpression", "instantiatedType")
	if !ok || instantiatedType.Kind != ElementValue || instantiatedType.Element.Symbol != function {
		t.Fatalf("FeatureChainExpression::instantiatedType = %#v, %t; want the dot function", instantiatedType, ok)
	}
	arguments, ok := evaluator.Property(chain, "InstantiationExpression", "argument")
	if !ok || arguments.Kind != SequenceValue || len(arguments.Values) != 1 {
		t.Fatalf("FeatureChainExpression::argument = %#v, %t; want one positional value", arguments, ok)
	}
	if arguments.Values[0].Kind != ElementValue ||
		arguments.Values[0].Element.Key() != argument.Key() {
		t.Fatalf("FeatureChainExpression::argument = %#v; want the positional expression", arguments)
	}
}

func TestInvocationArgumentsUseNamedRedefinitions(t *testing.T) {
	evaluator, root := newTestEvaluatorWithIndex(
		t, libs.NewModelIndex(), "named-invocation-arguments.sysml",
		"package P { action def F { in left : Integer; in right : Integer; } }",
	)
	pkg := testSymbol(t, root, "P")
	function := testSymbol(t, pkg.Scope, "F")
	inputs := evaluator.model.InputParametersOf(function)
	if len(inputs) != 2 {
		t.Fatalf("function inputs = %v; want two", inputs)
	}
	invocation := Element{
		Node: &ast.InvocationExpr{
			Type: ast.QualifiedNameOf("F"),
			NamedArgs: []ast.NamedArg{
				{Name: ast.QualifiedNameOf("right"), Value: &ast.LiteralInteger{}},
				{Name: ast.QualifiedNameOf("left"), Value: &ast.LiteralInteger{}},
			},
		},
		Container: pkg,
		Aspect:    "named-invocation",
	}
	structure := evaluator.options.Structure.(testStructure)
	typeMembership := MembershipElement(Membership{Aspect: "invoked-type", kind: "OwningMembership"})
	structure.metaclasses[invocation.Key()] = "InvocationExpression"
	structure.metaclasses[typeMembership.Key()] = "OwningMembership"
	structure.ownedRelationships[invocation.Key()] = []Element{typeMembership}
	structure.relatedElements[testRelatedKey{element: typeMembership.Key(), property: "memberElement"}] =
		[]Element{ElementOf(function)}

	features := []Element{
		{Aspect: "named-second"},
		{Aspect: "named-first"},
	}
	arguments := []Element{
		{Aspect: "named-second-value"},
		{Aspect: "named-first-value"},
	}
	for index, feature := range features {
		membership := MembershipElement(Membership{
			Aspect:  "named-membership-" + string(rune('0'+index)),
			Feature: true,
			kind:    "ParameterMembership",
		})
		redefinition := Element{Aspect: "named-redefinition-" + string(rune('0'+index))}
		valuation := MembershipElement(Membership{
			Aspect: "named-valuation-" + string(rune('0'+index)),
			kind:   "FeatureValue",
		})
		structure.metaclasses[feature.Key()] = "Feature"
		structure.metaclasses[membership.Key()] = "ParameterMembership"
		structure.metaclasses[redefinition.Key()] = "Redefinition"
		structure.metaclasses[valuation.Key()] = "FeatureValue"
		structure.metaclasses[arguments[index].Key()] = "Expression"
		structure.ownedRelationships[invocation.Key()] =
			append(structure.ownedRelationships[invocation.Key()], membership)
		structure.ownedRelationships[feature.Key()] = []Element{redefinition, valuation}
		structure.ownedRelationships[redefinition.Key()] = nil
		structure.relatedElements[testRelatedKey{element: membership.Key(), property: "memberElement"}] =
			[]Element{feature}
		structure.relatedElements[testRelatedKey{element: redefinition.Key(), property: "redefinedFeature"}] =
			[]Element{ElementOf(inputs[1-index])}
		structure.relatedElements[testRelatedKey{element: valuation.Key(), property: "memberElement"}] =
			[]Element{arguments[index]}
		structure.attributes[testRelatedKey{element: feature.Key(), property: "direction"}] =
			enumValue(ast.DirIn.String())
	}
	evaluator.options.Structure = structure

	got, ok := evaluator.Property(invocation, "InstantiationExpression", "argument")
	if !ok || got.Kind != SequenceValue || len(got.Values) != 2 {
		t.Fatalf("InvocationExpression::argument = %#v, %t; want two named values", got, ok)
	}
	if got.Values[0].Kind != ElementValue || got.Values[0].Element.Key() != arguments[1].Key() ||
		got.Values[1].Kind != ElementValue || got.Values[1].Element.Key() != arguments[0].Key() {
		t.Fatalf("InvocationExpression::argument = %#v; want formal input order", got)
	}
}

func TestConstructorArgumentsBindPositionally(t *testing.T) {
	evaluator, root := newTestEvaluator(
		t, "constructor-arguments.sysml",
		"package P { part def Pair { attribute left : Integer; attribute right : Integer; } }",
	)
	pkg := testSymbol(t, root, "P")
	pair := testSymbol(t, pkg.Scope, "Pair")
	constructor := Element{
		Node:      &ast.ConstructorExpr{Type: ast.QualifiedNameOf("Pair"), Args: []ast.Node{&ast.LiteralInteger{}, &ast.LiteralInteger{}}},
		Container: pkg,
		Aspect:    "constructor-expression",
	}
	typeElement := Element{Aspect: "constructor-type"}
	typeMembership := MembershipElement(Membership{Aspect: "constructor-type", kind: "OwningMembership"})
	resultMembership := MembershipElement(Membership{Aspect: "constructor-result", kind: "ReturnParameterMembership"})
	result := Element{Aspect: "constructor-result-feature"}
	structure := evaluator.options.Structure.(testStructure)
	structure.metaclasses[constructor.Key()] = "ConstructorExpression"
	structure.metaclasses[typeElement.Key()] = "Type"
	structure.metaclasses[typeMembership.Key()] = "OwningMembership"
	structure.metaclasses[resultMembership.Key()] = "ReturnParameterMembership"
	structure.metaclasses[result.Key()] = "Feature"
	structure.ownedRelationships[constructor.Key()] = []Element{typeMembership, resultMembership}
	structure.ownedRelationships[typeElement.Key()] = nil
	structure.relatedElements[testRelatedKey{element: typeMembership.Key(), property: "memberElement"}] =
		[]Element{typeElement}
	structure.relatedElements[testRelatedKey{element: resultMembership.Key(), property: "memberElement"}] =
		[]Element{result}

	resultFeatures := []Element{
		ElementOf(testSymbol(t, pair.Scope, "left")),
		ElementOf(testSymbol(t, pair.Scope, "right")),
	}
	for index := 0; index < 2; index++ {
		feature := resultFeatures[index]
		membership := MembershipElement(Membership{
			Aspect:  "constructor-feature-membership-" + string(rune('0'+index)),
			Feature: true,
			kind:    "FeatureMembership",
		})
		valuation := MembershipElement(Membership{
			Aspect: "constructor-valuation-" + string(rune('0'+index)),
			kind:   "FeatureValue",
		})
		argument := Element{Aspect: "constructor-argument-" + string(rune('0'+index))}
		structure.metaclasses[feature.Key()] = "Feature"
		structure.metaclasses[membership.Key()] = "FeatureMembership"
		structure.metaclasses[valuation.Key()] = "FeatureValue"
		structure.metaclasses[argument.Key()] = "Expression"
		structure.ownedRelationships[result.Key()] =
			append(structure.ownedRelationships[result.Key()], membership)
		structure.ownedRelationships[typeElement.Key()] =
			append(structure.ownedRelationships[typeElement.Key()], membership)
		structure.ownedRelationships[feature.Key()] = []Element{valuation}
		structure.relatedElements[testRelatedKey{element: membership.Key(), property: "memberElement"}] =
			[]Element{feature}
		structure.relatedElements[testRelatedKey{element: valuation.Key(), property: "memberElement"}] =
			[]Element{argument}
	}
	evaluator.options.Structure = structure

	got, ok := evaluator.Property(constructor, "InstantiationExpression", "argument")
	if !ok || got.Kind != SequenceValue || len(got.Values) != 2 {
		t.Fatalf("ConstructorExpression::argument = %#v, %t; want two positional values", got, ok)
	}
	for index, value := range got.Values {
		if value.Kind != ElementValue || value.Element.Aspect != "constructor-argument-"+string(rune('0'+index)) {
			t.Fatalf("ConstructorExpression::argument[%d] = %#v; want positional argument", index, value)
		}
	}
}

func TestConstructorArgumentsRejectExpressionOwnedInputs(t *testing.T) {
	evaluator, root := newTestEvaluator(
		t, "constructor-expression-owned-inputs.sysml",
		"package P { part def Pair { attribute left : Integer; } }",
	)
	pkg := testSymbol(t, root, "P")
	constructor := Element{
		Node:      &ast.ConstructorExpr{Type: ast.QualifiedNameOf("Pair"), Args: []ast.Node{&ast.LiteralInteger{}}},
		Container: pkg,
		Aspect:    "constructor-expression-owned-inputs",
	}
	typeElement := Element{Aspect: "constructor-expression-owned-inputs-type"}
	typeMembership := MembershipElement(Membership{
		Aspect: "constructor-expression-owned-inputs-type-membership",
		kind:   "OwningMembership",
	})
	inputMembership := MembershipElement(Membership{
		Aspect: "constructor-expression-owned-input-membership",
		kind:   "ParameterMembership",
	})
	resultMembership := MembershipElement(Membership{
		Aspect: "constructor-expression-result-membership",
		kind:   "ReturnParameterMembership",
	})
	input := Element{Aspect: "constructor-expression-owned-input"}
	result := Element{Aspect: "constructor-expression-result"}
	structure := evaluator.options.Structure.(testStructure)
	structure.metaclasses[constructor.Key()] = "ConstructorExpression"
	structure.metaclasses[typeElement.Key()] = "Type"
	structure.metaclasses[typeMembership.Key()] = "OwningMembership"
	structure.metaclasses[inputMembership.Key()] = "ParameterMembership"
	structure.metaclasses[resultMembership.Key()] = "ReturnParameterMembership"
	structure.metaclasses[input.Key()] = "Feature"
	structure.metaclasses[result.Key()] = "Feature"
	structure.ownedRelationships[constructor.Key()] = []Element{
		typeMembership,
		inputMembership,
		resultMembership,
	}
	structure.relatedElements[testRelatedKey{element: typeMembership.Key(), property: "memberElement"}] =
		[]Element{typeElement}
	structure.relatedElements[testRelatedKey{element: inputMembership.Key(), property: "memberElement"}] =
		[]Element{input}
	structure.relatedElements[testRelatedKey{element: resultMembership.Key(), property: "memberElement"}] =
		[]Element{result}
	evaluator.options.Structure = structure

	if _, ok := evaluator.Property(constructor, "InstantiationExpression", "argument"); ok {
		t.Fatal("ConstructorExpression::argument succeeded with a non-result expression-owned feature")
	}
	wantReason := Reason{
		Category: ReasonNotApplicable,
		Detail:   "ConstructorExpression owns non-result features; KerML requires its owned features to be the result",
	}
	if got, ok := evaluator.Explain(constructor, "InstantiationExpression", "argument"); !ok || got != wantReason {
		t.Fatalf("unsupported reason = %#v, %t; want %#v", got, ok, wantReason)
	}
}

func TestOperatorInvocationArgumentsRejectOverArity(t *testing.T) {
	evaluator, root := newTestEvaluatorWithIndex(
		t, libs.NewModelIndex(), "operator-overarity.sysml", "package P {}",
	)
	pkg := testSymbol(t, root, "P")
	operator := Element{
		Node:      &ast.OperatorExpr{Operator: ast.OpAdd, Operands: []ast.Node{&ast.LiteralInteger{}, &ast.LiteralInteger{}, &ast.LiteralInteger{}}},
		Container: pkg,
		Aspect:    "overarity-operator",
	}
	structure := evaluator.options.Structure.(testStructure)
	structure.metaclasses[operator.Key()] = "OperatorExpression"
	structure.ownedRelationships[operator.Key()] = nil
	for index := 0; index < 3; index++ {
		feature := Element{Aspect: "overarity-input-" + string(rune('0'+index))}
		membership := MembershipElement(Membership{
			Aspect:  "overarity-membership-" + string(rune('0'+index)),
			Feature: true,
			kind:    "ParameterMembership",
		})
		structure.metaclasses[feature.Key()] = "Feature"
		structure.metaclasses[membership.Key()] = "ParameterMembership"
		structure.ownedRelationships[operator.Key()] =
			append(structure.ownedRelationships[operator.Key()], membership)
		structure.relatedElements[testRelatedKey{element: membership.Key(), property: "memberElement"}] =
			[]Element{feature}
		structure.attributes[testRelatedKey{element: feature.Key(), property: "direction"}] =
			enumValue(ast.DirIn.String())
	}
	evaluator.options.Structure = structure
	if _, ok := evaluator.Property(operator, "InstantiationExpression", "argument"); ok {
		t.Fatal("OperatorExpression::argument succeeded for more positional parameters than inputs")
	}
}

func TestShorthandSuccessionEndpointsUseSemanticModel(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "shorthand-succession-endpoints.sysml", `package P {
		action def A {
			action first;
			action second;
			then second;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	owner := testSymbol(t, pkg.Scope, "A")
	var succession semantics.ActionSuccession
	for _, candidate := range evaluator.model.ActionSuccessions(owner) {
		if _, ok := candidate.Decl.(*ast.SuccessionEdge); ok {
			succession = candidate
			break
		}
	}
	if succession.Decl == nil || succession.Source.Symbol == nil || succession.Target.Symbol == nil {
		t.Fatalf("shorthand succession endpoints = %#v, want resolved semantic endpoints", succession)
	}

	element := NodeOf(succession.Decl)
	ownerElement := NodeOf(owner.Decl)
	firstEnd := Element{Aspect: "succession-first-end"}
	secondEnd := Element{Aspect: "succession-second-end"}
	firstMembership := MembershipElement(Membership{Aspect: "succession-first-end-membership", kind: "EndFeatureMembership"})
	secondMembership := MembershipElement(Membership{Aspect: "succession-second-end-membership", kind: "EndFeatureMembership"})
	secondReference := Element{Aspect: "succession-second-end-reference"}
	secondTarget := Element{Aspect: "succession-second-end-target"}
	inheritedFirstEnd := Element{Aspect: "inherited-first-end"}
	inheritedSecondEnd := Element{Aspect: "inherited-second-end"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			element.Key():            "SuccessionAsUsage",
			firstEnd.Key():           "FlowEnd",
			secondEnd.Key():          "FlowEnd",
			firstMembership.Key():    "EndFeatureMembership",
			secondMembership.Key():   "EndFeatureMembership",
			secondReference.Key():    "ReferenceSubsetting",
			secondTarget.Key():       "Feature",
			inheritedFirstEnd.Key():  "FlowEnd",
			inheritedSecondEnd.Key(): "FlowEnd",
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			element.Key(): {element: ownerElement, present: true},
		},
		ownedRelationships: map[ElementKey][]Element{
			element.Key():   {firstMembership, secondMembership},
			secondEnd.Key(): {secondReference},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: firstMembership.Key(), property: "memberElement"}:     {firstEnd},
			{element: secondMembership.Key(), property: "memberElement"}:    {secondEnd},
			{element: element.Key(), property: "feature"}:                   {inheritedFirstEnd, inheritedSecondEnd},
			{element: secondReference.Key(), property: "referencedFeature"}: {secondTarget},
		},
		attributes: map[testRelatedKey]Value{
			{element: firstEnd.Key(), property: "isEnd"}:           booleanValue(true),
			{element: secondEnd.Key(), property: "isEnd"}:          booleanValue(true),
			{element: inheritedFirstEnd.Key(), property: "isEnd"}:  booleanValue(true),
			{element: inheritedSecondEnd.Key(), property: "isEnd"}: booleanValue(true),
		},
	}

	ends := valueElements(t, testProperty(t, evaluator, element, "Connector", "connectorEnd"))
	if len(ends) != 2 || ends[0].Key() != firstEnd.Key() || ends[1].Key() != secondEnd.Key() {
		t.Fatalf("SuccessionAsUsage::connectorEnd = %#v, want owned ends replacing inherited ends by position", ends)
	}
	related := valueElements(t, testProperty(t, evaluator, element, "Connector", "relatedFeature"))
	if len(related) != 1 || related[0].Key() != secondTarget.Key() {
		t.Fatalf("SuccessionAsUsage::relatedFeature = %#v, want the explicitly subsetted target", related)
	}
	source := testProperty(t, evaluator, element, "Connector", "sourceFeature")
	if source.Kind != ElementValue || source.Element.Key() != secondTarget.Key() {
		t.Fatalf("SuccessionAsUsage::sourceFeature = %#v, want first relatedFeature", source)
	}
	target := valueElements(t, testProperty(t, evaluator, element, "Connector", "targetFeature"))
	if len(target) != 0 {
		t.Fatalf("SuccessionAsUsage::targetFeature = %#v, want no remaining related features", target)
	}
}

func TestShorthandSuccessionConnectorProjectionsRequireReferenceSubsetting(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "shorthand-succession-owner.sysml", `package P {
		action def A {
			action first;
			action second;
			then second;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	owner := testSymbol(t, pkg.Scope, "A")
	var succession semantics.ActionSuccession
	for _, candidate := range evaluator.model.ActionSuccessions(owner) {
		if _, ok := candidate.Decl.(*ast.SuccessionEdge); ok {
			succession = candidate
			break
		}
	}
	if succession.Decl == nil || succession.Source.Symbol == nil || succession.Target.Symbol == nil {
		t.Fatalf("shorthand succession endpoints = %#v, want resolved semantic endpoints", succession)
	}
	element := NodeOf(succession.Decl)
	evaluator.options.Structure = testStructure{
		metaclasses:        map[ElementKey]string{element.Key(): "SuccessionAsUsage"},
		ownedRelationships: map[ElementKey][]Element{element.Key(): {}},
	}

	for _, property := range []string{"relatedFeature", "sourceFeature", "targetFeature"} {
		assertConnectorProjectionOmitted(t, evaluator, element, property)
	}
}

func TestConnectorEndpointsUseGraphEndsWhenSuccessionIsUnresolved(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "unresolved-succession-graph-ends.sysml", "package P {}")
	connector := Element{Node: &ast.SuccessionEdge{}, Aspect: "graph-backed-succession"}
	sourceEnd := Element{Aspect: "source-end"}
	targetEnd := Element{Aspect: "target-end"}
	sourceMembership := MembershipElement(Membership{Aspect: "source-end-membership", kind: "EndFeatureMembership"})
	targetMembership := MembershipElement(Membership{Aspect: "target-end-membership", kind: "EndFeatureMembership"})
	sourceReference := Element{Aspect: "source-end-reference"}
	targetReference := Element{Aspect: "target-end-reference"}
	source := Element{Aspect: "resolved-source"}
	target := Element{Aspect: "resolved-target"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			connector.Key():        "SuccessionAsUsage",
			sourceEnd.Key():        "FlowEnd",
			targetEnd.Key():        "FlowEnd",
			sourceMembership.Key(): "EndFeatureMembership",
			targetMembership.Key(): "EndFeatureMembership",
			sourceReference.Key():  "ReferenceSubsetting",
			targetReference.Key():  "ReferenceSubsetting",
			source.Key():           "Feature",
			target.Key():           "Feature",
		},
		ownedRelationships: map[ElementKey][]Element{
			connector.Key(): {sourceMembership, targetMembership},
			sourceEnd.Key(): {sourceReference},
			targetEnd.Key(): {targetReference},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: sourceMembership.Key(), property: "memberElement"}:    {sourceEnd},
			{element: targetMembership.Key(), property: "memberElement"}:    {targetEnd},
			{element: sourceReference.Key(), property: "referencedFeature"}: {source},
			{element: targetReference.Key(), property: "referencedFeature"}: {target},
		},
	}

	gotSource := testProperty(t, evaluator, connector, "Connector", "sourceFeature")
	if gotSource.Kind != ElementValue || gotSource.Element.Key() != source.Key() {
		t.Fatalf("Connector::sourceFeature = %#v; want graph source end", gotSource)
	}
	gotTarget := valueElements(t, testProperty(t, evaluator, connector, "Connector", "targetFeature"))
	if len(gotTarget) != 1 || gotTarget[0].Key() != target.Key() {
		t.Fatalf("Connector::targetFeature = %#v; want graph target end", gotTarget)
	}
}

func TestInitialNodeSuccessionTargetFeatureRequiresReferenceSubsetting(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "initial-node-succession-endpoint.sysml", `package P {
		state def Lamp {
			entry; then off;
			state off;
		}
	}`)
	state := testSymbol(t, testSymbol(t, root, "P").Scope, "Lamp")
	var succession semantics.ActionSuccession
	for _, candidate := range evaluator.model.ActionSuccessions(state) {
		if _, ok := candidate.Decl.(*ast.SuccessionEdge); ok {
			succession = candidate
			break
		}
	}
	if succession.Decl == nil || succession.Target.Symbol != testSymbol(t, state.Scope, "off") {
		t.Fatalf("initial-node succession = %#v; want target state off", succession)
	}
	element := NodeOf(succession.Decl)
	if owner, found := evaluator.structureActionSuccessionOwner(structureElementNode(element)); !found || owner != state {
		t.Fatalf("semantic owner = %v, %t; want state %s", owner, found, symbols.FQNOf(state))
	}
	evaluator.options.Structure = testStructure{
		metaclasses:        map[ElementKey]string{element.Key(): "SuccessionAsUsage"},
		ownedRelationships: map[ElementKey][]Element{element.Key(): {}},
	}
	endpoints, found, known := evaluator.structureActionSuccessionFeatures(element)
	if !known || !found || len(endpoints.Values) != 2 {
		t.Fatalf("semantic succession endpoints = %#v, found %t, known %t; want two endpoints", endpoints, found, known)
	}

	assertConnectorProjectionOmitted(t, evaluator, element, "targetFeature")
}

func TestInitialNodeSuccessionRelatedFeatureRequiresReferenceSubsetting(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "initial-node-related-feature.sysml", `package P {
		state def Lamp {
			entry; then off;
			state off;
		}
	}`)
	state := testSymbol(t, testSymbol(t, root, "P").Scope, "Lamp")
	var succession semantics.ActionSuccession
	for _, candidate := range evaluator.model.ActionSuccessions(state) {
		if _, ok := candidate.Decl.(*ast.SuccessionEdge); ok {
			succession = candidate
			break
		}
	}
	if succession.Decl == nil || succession.Target.Symbol == nil {
		t.Fatalf("initial-node succession = %#v; want a resolved target", succession)
	}

	element := NodeOf(succession.Decl)
	target := ElementOf(succession.Target.Symbol)
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			element.Key(): "SuccessionAsUsage",
			target.Key():  "StateUsage",
		},
		ownedRelationships: map[ElementKey][]Element{element.Key(): {}},
	}

	assertConnectorProjectionOmitted(t, evaluator, element, "relatedFeature")
}

func TestTransitionMemberConnectorEndUsesOwnedStructureWithoutSemanticOwner(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "transition-member-owned-ends.sysml", `package P {
		state def S {
			state a;
			state b;
			transition t first a then b;
		}
	}`)
	state := testSymbol(t, testSymbol(t, root, "P").Scope, "S")
	var node ast.Node
	for _, succession := range evaluator.model.ActionSuccessions(state) {
		if _, ok := succession.Decl.(*ast.TransitionMember); ok {
			node = succession.Decl
			break
		}
	}
	if node == nil {
		t.Fatal("state definition has no parsed transition")
	}
	connector := NodeOf(node)
	firstEnd := Element{Aspect: "first-end"}
	secondEnd := Element{Aspect: "second-end"}
	firstMembership := MembershipElement(Membership{Aspect: "first-end-membership", kind: "EndFeatureMembership"})
	secondMembership := MembershipElement(Membership{Aspect: "second-end-membership", kind: "EndFeatureMembership"})
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			connector.Key():        "SuccessionAsUsage",
			firstEnd.Key():         "FlowEnd",
			secondEnd.Key():        "FlowEnd",
			firstMembership.Key():  "EndFeatureMembership",
			secondMembership.Key(): "EndFeatureMembership",
		},
		ownedRelationships: map[ElementKey][]Element{
			connector.Key(): {firstMembership, secondMembership},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: firstMembership.Key(), property: "memberElement"}:  {firstEnd},
			{element: secondMembership.Key(), property: "memberElement"}: {secondEnd},
		},
	}

	ends, ok := evaluator.Property(connector, "Connector", "connectorEnd")
	if !ok || ends.Kind != SequenceValue || len(ends.Values) != 2 {
		t.Fatalf("TransitionMember::connectorEnd = %#v, %t; want the two graph-owned ends without a semantic owner", ends, ok)
	}
}

func TestTransitionMemberConnectorProjectionsRequireReferenceSubsetting(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "transition-member-semantic-endpoints.sysml", `package P {
		state def S {
			state a;
			state b;
			transition t first a then b;
		}
	}`)
	state := testSymbol(t, testSymbol(t, root, "P").Scope, "S")
	transition := testSymbol(t, state.Scope, "t")
	var node ast.Node
	for _, succession := range evaluator.model.ActionSuccessions(state) {
		if _, ok := succession.Decl.(*ast.TransitionMember); ok {
			node = succession.Decl
			break
		}
	}
	if node == nil {
		t.Fatal("state definition has no parsed transition")
	}
	connector := Element{Node: node, Container: transition, Aspect: "succession"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{connector.Key(): "SuccessionAsUsage"},
	}

	for _, property := range []string{"sourceFeature", "targetFeature"} {
		assertConnectorProjectionOmitted(t, evaluator, connector, property)
	}
}

func TestTransitionMemberConnectorProjectionsDoNotUseSemanticEndpoints(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "transition-member-owner.sysml", `package P {
		state def S {
			state a;
			state b;
			transition t first a then b;
		}
	}`)
	state := testSymbol(t, testSymbol(t, root, "P").Scope, "S")
	var node ast.Node
	for _, succession := range evaluator.model.ActionSuccessions(state) {
		if _, ok := succession.Decl.(*ast.TransitionMember); ok {
			node = succession.Decl
			break
		}
	}
	if node == nil {
		t.Fatal("state definition has no parsed transition")
	}
	connector := NodeOf(node)
	evaluator.options.Structure = testStructure{
		metaclasses:        map[ElementKey]string{connector.Key(): "SuccessionAsUsage"},
		ownedRelationships: map[ElementKey][]Element{connector.Key(): {}},
	}

	for _, property := range []string{"sourceFeature", "targetFeature"} {
		assertConnectorProjectionOmitted(t, evaluator, connector, property)
	}
	relationshipSource := testProperty(t, evaluator, connector, "TransitionUsage", "source")
	if relationshipSource.Kind != ElementValue || relationshipSource.Element.Symbol != testSymbol(t, state.Scope, "a") {
		t.Fatalf("TransitionUsage::source = %#v; want state a", relationshipSource)
	}
	relationshipTarget := testProperty(t, evaluator, connector, "TransitionUsage", "target")
	if relationshipTarget.Kind != ElementValue || relationshipTarget.Element.Symbol != testSymbol(t, state.Scope, "b") {
		t.Fatalf("TransitionUsage::target = %#v; want state b", relationshipTarget)
	}
}

func TestStructureNamesDeclaredElements(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "declared-name-without-metaclass.sysml", `package P {
		part named;
		state def Machine {
			state active;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	named := testSymbol(t, pkg.Scope, "named")
	if got, _, ok := structureDeclaredNames(ElementOf(named)); !ok || got != "named" {
		t.Fatalf("declared names = %q, _, %t; want named", got, ok)
	}
	element := ElementOf(named)
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{element.Key(): "PartUsage"},
	}

	name := testProperty(t, evaluator, element, "Element", "name")
	if name.Kind != StringValue || name.String != "named" {
		t.Fatalf("Element::name = %#v; want the declared name", name)
	}
}

func TestStructureDerivedElementDoesNotInheritParentName(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "derived-element-name.sysml", "package P { part parent; }")
	derived := Element{
		Node:   &ast.Usage{Ident: ast.Identification{Name: "parent"}},
		Aspect: "synthetic:subject",
	}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{derived.Key(): "ReferenceUsage"},
		ownedRelationships: map[ElementKey][]Element{
			derived.Key(): {},
		},
	}

	name := testProperty(t, evaluator, derived, "Element", "name")
	if name.Kind != NullValue {
		t.Fatalf("Element::name = %#v; want null for a mapping-minted element", name)
	}
}

func TestTransitionEndpointsUseSymbolDeclarationWhenNodeIsOmitted(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "transition-symbol-endpoints.sysml", `package P {
		state def S {
			state a;
			state b;
			transition t first a then b;
		}
	}`)
	state := testSymbol(t, testSymbol(t, root, "P").Scope, "S")
	transition := testSymbol(t, state.Scope, "t")
	element := Element{Symbol: transition}
	evaluator.options.Structure = testStructure{
		metaclasses:        map[ElementKey]string{element.Key(): "TransitionUsage"},
		ownedRelationships: map[ElementKey][]Element{element.Key(): {}},
	}

	source := testProperty(t, evaluator, element, "TransitionUsage", "source")
	target := testProperty(t, evaluator, element, "TransitionUsage", "target")
	if source.Kind != ElementValue || source.Element.Symbol != testSymbol(t, state.Scope, "a") {
		t.Fatalf("TransitionUsage::source = %#v; want state a", source)
	}
	if target.Kind != ElementValue || target.Element.Symbol != testSymbol(t, state.Scope, "b") {
		t.Fatalf("TransitionUsage::target = %#v; want state b", target)
	}
}

func TestTransitionEndpointsUseResolvedGraphReferences(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "transition-graph-endpoints.sysml", "package P {}")
	transition := Element{Aspect: "graph-transition"}
	source := Element{Aspect: "graph-source"}
	target := Element{Aspect: "graph-target"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			transition.Key(): "TransitionUsage",
			source.Key():     "ActionUsage",
			target.Key():     "ActionUsage",
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: transition.Key(), property: "source"}: {source},
			{element: transition.Key(), property: "target"}: {target},
		},
		statedRelated: map[testRelatedKey]bool{
			{element: transition.Key(), property: "source"}: true,
			{element: transition.Key(), property: "target"}: true,
		},
	}

	for property, want := range map[string]Element{"source": source, "target": target} {
		got := testProperty(t, evaluator, transition, "TransitionUsage", property)
		if got.Kind != ElementValue || got.Element.Key() != want.Key() {
			t.Errorf("TransitionUsage::%s = %#v; want graph endpoint %s", property, got, want.Aspect)
		}
	}
}

func TestStructureEffectiveDirectionResolvesNodeOwnerSymbol(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "node-owner-direction.sysml", `package P {
		action def A {
			in x : Integer;
		}
	}`)
	owner := testSymbol(t, testSymbol(t, root, "P").Scope, "A")
	input := testSymbol(t, owner.Scope, "x")

	direction, known := evaluator.structureEffectiveDirection(NodeOf(owner.Decl), ElementOf(input))
	if !known || direction != ast.DirIn {
		t.Fatalf("effective direction = %v, %t; want input from the AST-backed owner", direction, known)
	}
}

func TestStructureTypeInputUsesNodeBackedBehaviorParameters(t *testing.T) {
	evaluator, root := newTestEvaluatorWithIndex(t, libs.NewModelIndex(), "node-backed-type-input.sysml", `package P {
		action def A {
			in x : Integer;
			out y : Integer;
		}
	}`)
	owner := testSymbol(t, testSymbol(t, root, "P").Scope, "A")
	element := Element{Node: owner.Decl, Container: owner.Owner()}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{element.Key(): "Function"},
	}

	inputs := valueElements(t, testProperty(t, evaluator, element, "Type", "input"))
	if len(inputs) != 1 || inputs[0].Symbol != testSymbol(t, owner.Scope, "x") {
		t.Fatalf("node-backed Type::input = %#v; want input x", inputs)
	}
}

func TestStructureEffectiveDirectionUsesGraphWhenSemanticDirectionIsNone(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "graph-direction.sysml", `package P {
		action def A {
			attribute x;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	owner := testSymbol(t, pkg.Scope, "A")
	feature := ElementOf(testSymbol(t, owner.Scope, "x"))
	structure := testDocumentStructure(evaluator, root, pkg, owner)
	structure.attributes[testRelatedKey{element: feature.Key(), property: "direction"}] = enumValue(ast.DirIn.String())
	evaluator.options.Structure = structure

	direction, known := evaluator.structureEffectiveDirection(NodeOf(owner.Decl), feature)
	if !known || direction != ast.DirIn {
		t.Fatalf("effective direction = %v, %t; want graph-stated input direction", direction, known)
	}
}

func TestInitialNodeSuccessionUsesGraphOwnedConnectorEnds(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "initial-node-succession-ends.sysml", `package P {
		action def A {
			action a;
			action b;
			first a then b;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	owner := testSymbol(t, pkg.Scope, "A")
	var succession semantics.ActionSuccession
	for _, candidate := range evaluator.model.ActionSuccessions(owner) {
		if _, ok := candidate.Decl.(*ast.InitialNode); ok {
			succession = candidate
			break
		}
	}
	if succession.Decl == nil || succession.Source.Symbol == nil || succession.Target.Symbol == nil {
		t.Fatalf("initial-node succession endpoints = %#v, want resolved semantic endpoints", succession)
	}

	element := NodeOf(succession.Decl)
	ownerElement := NodeOf(owner.Decl)
	firstEnd := Element{Aspect: "initial-node-first-end"}
	secondEnd := Element{Aspect: "initial-node-second-end"}
	firstMembership := MembershipElement(Membership{Aspect: "initial-node-first-membership", kind: "EndFeatureMembership"})
	secondMembership := MembershipElement(Membership{Aspect: "initial-node-second-membership", kind: "EndFeatureMembership"})
	firstReference := Element{Aspect: "initial-node-first-reference"}
	secondReference := Element{Aspect: "initial-node-second-reference"}
	source := ElementOf(succession.Source.Symbol)
	target := ElementOf(succession.Target.Symbol)
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			element.Key():          "SuccessionAsUsage",
			firstEnd.Key():         "FlowEnd",
			secondEnd.Key():        "FlowEnd",
			firstMembership.Key():  "EndFeatureMembership",
			secondMembership.Key(): "EndFeatureMembership",
			firstReference.Key():   "ReferenceSubsetting",
			secondReference.Key():  "ReferenceSubsetting",
			source.Key():           "ActionUsage",
			target.Key():           "ActionUsage",
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			element.Key(): {element: ownerElement, present: true},
		},
		ownedRelationships: map[ElementKey][]Element{
			element.Key():   {firstMembership, secondMembership},
			firstEnd.Key():  {firstReference},
			secondEnd.Key(): {secondReference},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: firstMembership.Key(), property: "memberElement"}:     {firstEnd},
			{element: secondMembership.Key(), property: "memberElement"}:    {secondEnd},
			{element: firstReference.Key(), property: "referencedFeature"}:  {source},
			{element: secondReference.Key(), property: "referencedFeature"}: {target},
		},
	}

	ends := valueElements(t, testProperty(t, evaluator, element, "Connector", "connectorEnd"))
	if len(ends) != 2 || ends[0].Key() != firstEnd.Key() || ends[1].Key() != secondEnd.Key() {
		t.Fatalf("SuccessionAsUsage::connectorEnd = %#v, want the two graph-owned ends", ends)
	}
	related := valueElements(t, testProperty(t, evaluator, element, "Connector", "relatedFeature"))
	if len(related) != 2 || related[0].Key() != source.Key() || related[1].Key() != target.Key() {
		t.Fatalf("SuccessionAsUsage::relatedFeature = %#v, want both referenced ends", related)
	}
}

func TestStructureFeatureValueUsesOwnedRelatedValue(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "feature-value-owned-related.sysml", `package P { attribute value = 1; }`)
	valuation := MembershipElement(Membership{Aspect: "feature-value", kind: "FeatureValue"})
	value := Element{Aspect: "feature-value-target"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			valuation.Key(): "FeatureValue",
			value.Key():     "LiteralInteger",
		},
		ownedRelatedElements: map[ElementKey][]Element{
			valuation.Key(): {value},
		},
	}

	got, ok := evaluator.relationshipEndpoints(valuation, "value")
	if !ok || len(got) != 1 || got[0].Kind != ElementValue || got[0].Element.Key() != value.Key() {
		t.Fatalf("FeatureValue::value = %#v, supported=%t; want its owned related element", got, ok)
	}
}

func TestStructureFeatureReferenceResolvesNameOnlyTarget(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "name-only-feature-reference.sysml", `package P {
		part target;
		attribute reference = target;
		attribute unresolved = missing;
	}`)
	pkg := testSymbol(t, root, "P")
	referenceNode := func(name string) *ast.FeatureReference {
		usage := testSymbol(t, pkg.Scope, name).Decl.(*ast.Usage)
		var found *ast.FeatureReference
		ast.Inspect(usage.Value, func(node ast.Node) bool {
			if reference, ok := node.(*ast.FeatureReference); ok {
				found = reference
			}
			return true
		})
		if found == nil {
			t.Fatalf("%s value has no FeatureReference", name)
		}
		return found
	}
	expression := Element{Node: referenceNode("reference"), Container: pkg}
	unresolved := Element{Node: referenceNode("unresolved"), Container: pkg}
	target := ElementOf(testSymbol(t, pkg.Scope, "target"))
	referenceMembership := MembershipElement(Membership{Aspect: "reference-membership"})
	if resolved, ok := evaluator.resolver.ResolveTarget(pkg.Scope, expression.Node); !ok || resolved != target.Symbol {
		t.Fatalf("name-only reference resolved to %v, %t; want %v", resolved, ok, target.Symbol)
	}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			expression.Key():          "FeatureReferenceExpression",
			unresolved.Key():          "FeatureReferenceExpression",
			target.Key():              "PartUsage",
			referenceMembership.Key(): "Membership",
		},
		ownedRelationships: map[ElementKey][]Element{
			expression.Key(): {referenceMembership},
			unresolved.Key(): {},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: referenceMembership.Key(), property: "memberElement"}: {target},
		},
	}

	got := testProperty(t, evaluator, expression, "FeatureReferenceExpression", "referent")
	if got.Kind != ElementValue || got.Element.Key() != target.Key() {
		t.Fatalf("FeatureReferenceExpression::referent = %#v; want target", got)
	}
	if _, ok := evaluator.Property(unresolved, "FeatureReferenceExpression", "referent"); ok {
		t.Fatal("unresolved name-only reference reported a value")
	}

	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			unresolved.Key():          "FeatureReferenceExpression",
			referenceMembership.Key(): "Membership",
		},
		ownedRelationships: map[ElementKey][]Element{
			unresolved.Key(): {referenceMembership},
		},
		unresolvedRelated: map[testRelatedKey]bool{
			{element: referenceMembership.Key(), property: "memberElement"}: true,
		},
	}
	if _, ok := evaluator.Property(unresolved, "FeatureReferenceExpression", "referent"); ok {
		t.Fatal("unresolved graph member was re-resolved from the source name")
	}
}

func TestStructureFeatureReferenceRejectsSelfReferent(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "self-feature-reference.sysml", `package P {
		part target;
		attribute reference = target;
	}`)
	pkg := testSymbol(t, root, "P")
	usage := testSymbol(t, pkg.Scope, "reference").Decl.(*ast.Usage)
	var reference *ast.FeatureReference
	ast.Inspect(usage.Value, func(node ast.Node) bool {
		if candidate, ok := node.(*ast.FeatureReference); ok {
			reference = candidate
		}
		return true
	})
	if reference == nil {
		t.Fatal("reference value has no FeatureReference")
	}
	expression := Element{Node: reference, Container: pkg, Aspect: "self-referent"}
	membership := MembershipElement(Membership{Aspect: "self-membership", kind: "Membership"})
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			expression.Key(): "FeatureReferenceExpression",
			membership.Key(): "Membership",
		},
		ownedRelationships: map[ElementKey][]Element{
			expression.Key(): {membership},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: membership.Key(), property: "memberElement"}: {expression},
		},
	}
	if _, ok := evaluator.Property(expression, "FeatureReferenceExpression", "referent"); ok {
		t.Fatal("FeatureReferenceExpression::referent resolved to its enclosing expression")
	}
}

func TestStructureFeatureReferenceNameDoesNotUseReferent(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "feature-reference-name.sysml", `package P {
		part target;
		attribute value = target;
	}`)
	pkg := testSymbol(t, root, "P")
	usage := testSymbol(t, pkg.Scope, "value").Decl.(*ast.Usage)
	var reference *ast.FeatureReference
	ast.Inspect(usage.Value, func(node ast.Node) bool {
		if candidate, ok := node.(*ast.FeatureReference); ok {
			reference = candidate
		}
		return true
	})
	if reference == nil {
		t.Fatal("attribute value has no FeatureReference")
	}
	element := Element{
		Node:      reference,
		Container: pkg,
		Symbol:    testSymbol(t, pkg.Scope, "target"),
	}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			element.Key(): "FeatureReferenceExpression",
		},
	}

	if got := testProperty(t, evaluator, element, "Element", "name"); got.Kind != NullValue {
		t.Fatalf("Element::name = %#v; want null for FeatureReferenceExpression", got)
	}
}

func TestStructureUnresolvedRelationshipTargetIsUnsupported(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "unresolved-relationship-target.sysml", "package P {}")
	relationship := Element{Node: &ast.Relationship{Target: ast.QualifiedNameOf("Integer")}}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			relationship.Key(): "FeatureTyping",
		},
		ownedRelatedElements: map[ElementKey][]Element{
			relationship.Key(): {},
		},
	}
	if _, ok := evaluator.Property(relationship, "FeatureTyping", "type"); ok {
		t.Fatal("FeatureTyping::type reported a value without a graph-resolved target")
	}
}

func TestEvaluatorExplainCategorizesUnsupportedProperties(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "explain-unsupported.sysml", "package P {}")
	pkg := testSymbol(t, root, "P")
	if got, ok := evaluator.Explain(ElementOf(pkg), "Element", "name"); ok {
		t.Fatalf("Explain for a supported property = %#v; want no recorded reason", got)
	}

	unregistered := Element{Aspect: "explain-unregistered"}
	if _, ok := evaluator.Property(unregistered, "CalculationDefinition", "calculation"); ok {
		t.Fatal("an unregistered property unexpectedly succeeded")
	}
	noRule := Reason{Category: ReasonNoRule, Detail: "CalculationDefinition::calculation"}
	if got, ok := evaluator.Explain(unregistered, "CalculationDefinition", "calculation"); !ok || got != noRule {
		t.Fatalf("Explain for an unregistered property = %#v, %t; want %#v", got, ok, noRule)
	}
	if got, want := noRule.String(), "no-rule CalculationDefinition::calculation"; got != want {
		t.Fatalf("no-rule reason string = %q; want %q", got, want)
	}

	dependent := Element{Aspect: "explain-dependent"}
	dependency := Reason{Category: ReasonDependsOnUnsupported, Dependency: "Element::name"}
	evaluator.recordUnsupportedReason(dependent, "Element", "qualifiedName", dependency)
	if got, ok := evaluator.Explain(dependent, "Element", "qualifiedName"); !ok || got != dependency {
		t.Fatalf("Explain for an unsupported dependency = %#v, %t; want %#v", got, ok, dependency)
	}

	reference := Element{Node: &ast.FeatureReference{}}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			reference.Key(): "FeatureReferenceExpression",
		},
		unresolvedRelated: map[testRelatedKey]bool{
			{element: reference.Key(), property: "referent"}: true,
		},
	}
	if _, ok := evaluator.Explain(reference, "FeatureReferenceExpression", "referent"); ok {
		t.Fatal("Explain returned a cause before an unsupported read was requested")
	}
	if _, ok := evaluator.Property(reference, "FeatureReferenceExpression", "referent"); ok {
		t.Fatal("an unresolved reference unexpectedly succeeded")
	}
	unresolved := Reason{
		Category: ReasonUnresolvedReference,
		Detail:   "compact reference endpoint is unresolved",
	}
	if got, ok := evaluator.Explain(reference, "FeatureReferenceExpression", "referent"); !ok || got != unresolved {
		t.Fatalf("Explain for an unresolved reference = %#v, %t; want %#v", got, ok, unresolved)
	}

	endpoint := Element{Node: &ast.Relationship{}}
	unrepresented := Reason{Category: ReasonUnrepresentedEndpoint, Detail: "connector source endpoint is unrepresented"}
	evaluator.recordUnsupportedReason(endpoint, "Connector", "sourceFeature", unrepresented)
	if got, ok := evaluator.Explain(endpoint, "Connector", "sourceFeature"); !ok || got != unrepresented {
		t.Fatalf("Explain for an unrepresented endpoint = %#v, %t; want %#v", got, ok, unrepresented)
	}

	libraryTarget := Element{Aspect: "explain-library-target"}
	libraryIdentity := Reason{
		Category: ReasonLibraryTargetWithoutCatalogIdentity,
		Detail:   "Base::Anything::",
	}
	evaluator.recordUnsupportedReason(
		libraryTarget, "Element", "ownedElement", libraryIdentity,
	)
	if got, ok := evaluator.Explain(libraryTarget, "Element", "ownedElement"); !ok || got != libraryIdentity {
		t.Fatalf("Explain for a library target without catalog identity = %#v, %t; want %#v", got, ok, libraryIdentity)
	}
}

func TestEvaluatorExplainIdentifiesUnresolvedOperatorFunction(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "explain-operator.sysml", "package P {}")
	pkg := testSymbol(t, root, "P")
	operator := Element{
		Node:      &ast.OperatorExpr{Operator: ast.OpAdd},
		Container: pkg,
		Aspect:    "unresolved-operator-function",
	}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			operator.Key(): "OperatorExpression",
		},
	}
	if _, ok := evaluator.Property(operator, "InstantiationExpression", "instantiatedType"); ok {
		t.Fatal("an unresolved operator unexpectedly succeeded")
	}
	want := Reason{Category: ReasonUnresolvedOperatorFunction, Detail: "+"}
	if got, ok := evaluator.Explain(operator, "InstantiationExpression", "instantiatedType"); !ok || got != want {
		t.Fatalf("Explain for an unresolved operator = %#v, %t; want %#v", got, ok, want)
	}
}

func TestEvaluatorExplainRecordsMissingFeatureTypeEndpoint(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "explain-feature-type.sysml", "package P {}")
	feature := Element{Aspect: "explain-feature-type"}
	typing := Element{Aspect: "explain-feature-type-typing"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			feature.Key(): "Feature",
			typing.Key():  "FeatureTyping",
		},
		ownedRelationships: map[ElementKey][]Element{
			feature.Key(): {typing},
		},
	}
	if _, ok := evaluator.Property(feature, "Feature", "type"); ok {
		t.Fatal("Feature::type unexpectedly succeeded without a typing target")
	}
	want := Reason{
		Category: ReasonUnrepresentedEndpoint,
		Detail:   "Feature::type a FeatureTyping, Subsetting, or FeatureChaining target is unavailable in Structure",
	}
	if got, ok := evaluator.Explain(feature, "Feature", "type"); !ok || got != want {
		t.Fatalf("Explain for a missing feature type endpoint = %#v, %t; want %#v", got, ok, want)
	}
}

func TestEvaluatorExplainIdentifiesUnresolvedNamingRedefinition(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "explain-naming-redefinition.sysml", "package P {}")
	usage := &ast.Usage{
		Relationships: []*ast.Relationship{{
			Kind:   ast.RelRedefines,
			Target: ast.QualifiedNameOf("missing"),
		}},
	}
	feature := Element{Node: usage, Aspect: "unresolved-naming-redefinition"}
	redefinition := Element{Node: usage, Aspect: "unresolved-redefinition"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			feature.Key():      "ReferenceUsage",
			redefinition.Key(): "Redefinition",
		},
		ownedRelationships: map[ElementKey][]Element{
			feature.Key(): {redefinition},
		},
		unresolvedRelated: map[testRelatedKey]bool{
			{element: redefinition.Key(), property: "redefinedFeature"}: true,
		},
	}

	if _, ok := evaluator.Property(feature, "Element", "shortName"); ok {
		t.Fatal("shortName unexpectedly succeeded with an unresolved naming redefinition")
	}
	want := Reason{
		Category: ReasonUnresolvedReference,
		Detail:   "Element::shortName naming Redefinition::redefinedFeature target is unresolved",
	}
	if got, ok := evaluator.Explain(feature, "Element", "shortName"); !ok || got != want {
		t.Fatalf("Explain for an unresolved naming redefinition = %#v, %t; want %#v", got, ok, want)
	}
}

func TestEvaluatorFeatureDirectionAndDirectedFeature(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "direction.sysml", `package Base {
		part def Anything;
	}
	package Parts {
		part def Part;
	}
	package Actions {
		action def Action;
	}
	package Calculations {
		calc def Calculation;
	}
	package P {
		part def Base {
			in attribute directed;
		}
		part def Derived specializes Base {
			attribute :>> directed;
		}
		calc def Compute {
			in attribute input;
			return attribute result;
		}
		action def Loop {
			loop action charging { } until true;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	base := testSymbol(t, pkg.Scope, "Base")
	derived := testSymbol(t, pkg.Scope, "Derived")
	compute := testSymbol(t, pkg.Scope, "Compute")
	loop := testSymbol(t, pkg.Scope, "Loop")
	baseDirection := testProperty(t, evaluator, ElementOf(testSymbol(t, base.Scope, "directed")), "Feature", "direction")
	if baseDirection.Kind != EnumValue || baseDirection.Enum != "in" {
		t.Fatalf("declared Feature::direction = %#v, want in", baseDirection)
	}
	redefinedDirection := testProperty(t, evaluator, ElementOf(testSymbol(t, derived.Scope, "directed")), "Feature", "direction")
	if redefinedDirection.Kind != NullValue {
		t.Fatalf("redefined Feature::direction = %#v, want null", redefinedDirection)
	}
	input := testProperty(t, evaluator, ElementOf(testSymbol(t, compute.Scope, "input")), "Feature", "direction")
	if input.Kind != EnumValue || input.Enum != "in" {
		t.Fatalf("input Feature::direction = %#v, want in", input)
	}
	result := testProperty(t, evaluator, ElementOf(testSymbol(t, compute.Scope, "result")), "Feature", "direction")
	if result.Kind != EnumValue || result.Enum != "out" {
		t.Fatalf("return Feature::direction = %#v, want out", result)
	}
	loopDecl := loop.Decl.(*ast.Definition)
	loopNode := loopDecl.Members[0].(*ast.WhileLoopActionNode)
	bodyMembership := loopNode.Body[0].(*ast.Membership)
	body := bodyMembership.Member.(*ast.Usage)
	structure := evaluator.options.Structure.(testStructure)
	bodyElement := Element{
		Node:      body,
		Container: loop,
		Membership: Membership{
			Node:    body,
			Owner:   loop,
			Feature: true,
			kind:    "ParameterMembership",
		},
	}
	structure.metaclasses[MembershipElement(bodyElement.Membership).Key()] = "ParameterMembership"
	evaluator.options.Structure = structure
	bodyDirection := testProperty(t, evaluator, bodyElement, "Feature", "direction")
	if bodyDirection.Kind != EnumValue || bodyDirection.Enum != "in" {
		t.Fatalf("body parameter Feature::direction = %#v, want in", bodyDirection)
	}
	directed := valueElements(t, testProperty(t, evaluator, ElementOf(compute), "Type", "directedFeature"))
	if !containsElementSymbol(directed, testSymbol(t, compute.Scope, "input")) ||
		!containsElementSymbol(directed, testSymbol(t, compute.Scope, "result")) {
		t.Fatalf("Type::directedFeature = %v, want input and return parameters", directed)
	}
}

func TestStructureBackedDirectionsUseGraphOwnedFeatures(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "structure-directions.sysml", `package Calculations {
		calc def Calculation;
	}
	package P {
		calc def Compute {
			in attribute input;
			out attribute output;
			inout attribute both;
			return attribute result;
			attribute ordinary;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	compute := testSymbol(t, pkg.Scope, "Compute")
	calculations := testSymbol(t, root, "Calculations")
	calculation := testSymbol(t, calculations.Scope, "Calculation")
	features := map[string]string{
		"input":    "ParameterMembership",
		"output":   "ParameterMembership",
		"both":     "ParameterMembership",
		"result":   "ReturnParameterMembership",
		"ordinary": "FeatureMembership",
	}
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			ElementOf(compute).Key():     "CalculationDefinition",
			ElementOf(calculation).Key(): "CalculationDefinition",
		},
		ownedRelationships:    map[ElementKey][]Element{ElementOf(compute).Key(): {}},
		owningRelationships:   make(map[ElementKey]testOptionalElement),
		ownedRelatedElements:  make(map[ElementKey][]Element),
		owningRelatedElements: make(map[ElementKey]testOptionalElement),
		relatedElements:       make(map[testRelatedKey][]Element),
		attributes:            make(map[testRelatedKey]Value),
	}
	for name, membershipClass := range features {
		feature := testSymbol(t, compute.Scope, name)
		featureElement := ElementOf(feature)
		membership := Element{Membership: featureElement.Membership, IsMembership: true}
		structure.metaclasses[featureElement.Key()] = "Feature"
		structure.metaclasses[membership.Key()] = membershipClass
		structure.ownedRelationships[ElementOf(compute).Key()] = append(
			structure.ownedRelationships[ElementOf(compute).Key()], membership,
		)
		structure.owningRelationships[featureElement.Key()] = testOptionalElement{
			element: membership, present: true,
		}
		structure.owningRelatedElements[membership.Key()] = testOptionalElement{
			element: ElementOf(compute), present: true,
		}
		structure.ownedRelatedElements[membership.Key()] = []Element{featureElement}
		structure.relatedElements[testRelatedKey{element: membership.Key(), property: "memberElement"}] = []Element{featureElement}
		direction := nullValue()
		switch name {
		case "input":
			direction = enumValue("in")
		case "output", "result":
			direction = enumValue("out")
		case "both":
			direction = enumValue("inout")
		}
		structure.attributes[testRelatedKey{element: featureElement.Key(), property: "direction"}] = direction
	}
	evaluator.options.Structure = structure

	directed := valueElements(t, testProperty(t, evaluator, ElementOf(compute), "Type", "directedFeature"))
	inputs := valueElements(t, testProperty(t, evaluator, ElementOf(compute), "Type", "input"))
	outputs := valueElements(t, testProperty(t, evaluator, ElementOf(compute), "Type", "output"))
	for _, name := range []string{"input", "output", "both", "result"} {
		if !containsElementSymbol(directed, testSymbol(t, compute.Scope, name)) {
			t.Errorf("directedFeature omits %s", name)
		}
	}
	if containsElementSymbol(directed, testSymbol(t, compute.Scope, "ordinary")) {
		t.Error("directedFeature includes a feature whose graph direction is null")
	}
	if !containsElementSymbol(inputs, testSymbol(t, compute.Scope, "input")) ||
		!containsElementSymbol(inputs, testSymbol(t, compute.Scope, "both")) {
		t.Errorf("input = %v, want input and both", inputs)
	}
	if !containsElementSymbol(outputs, testSymbol(t, compute.Scope, "output")) ||
		!containsElementSymbol(outputs, testSymbol(t, compute.Scope, "both")) ||
		!containsElementSymbol(outputs, testSymbol(t, compute.Scope, "result")) {
		t.Errorf("output = %v, want output, both, and result", outputs)
	}
}

func TestStructureBackedVariantPropertiesUseVariantMemberships(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "structure-variants.sysml", `package Items {
		abstract item def Item;
	}
	package Parts {
		abstract part def Part :> Items::Item;
	}
	package P {
		part def Choice {
			part ordinary;
			variant part selected references ordinary;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	choice := testSymbol(t, pkg.Scope, "Choice")
	variant := testSymbol(t, choice.Scope, "selected")
	ordinary := testSymbol(t, choice.Scope, "ordinary")
	variantMembership := Element{Membership: ElementOf(variant).Membership, IsMembership: true}
	ordinaryMembership := Element{Membership: ElementOf(ordinary).Membership, IsMembership: true}
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			ElementOf(choice).Key():   "PartDefinition",
			variantMembership.Key():   "VariantMembership",
			ordinaryMembership.Key():  "FeatureMembership",
			ElementOf(variant).Key():  "PartUsage",
			ElementOf(ordinary).Key(): "PartUsage",
		},
		ownedRelationships: map[ElementKey][]Element{
			ElementOf(choice).Key(): {variantMembership, ordinaryMembership},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: variantMembership.Key(), property: "memberElement"}:  {ElementOf(variant)},
			{element: ordinaryMembership.Key(), property: "memberElement"}: {ElementOf(ordinary)},
		},
	}
	evaluator.options.Structure = structure

	variants := valueElements(t, testProperty(t, evaluator, ElementOf(choice), "Usage", "variant"))
	if len(variants) != 1 || variants[0].Key() != ElementOf(variant).Key() {
		t.Fatalf("Usage::variant = %#v; want selected only", variants)
	}
	variantMemberships := valueMemberships(t, testProperty(t, evaluator, ElementOf(choice), "Usage", "variantMembership"))
	if len(variantMemberships) != 1 || variantMemberships[0].Key() != variantMembership.Membership.Key() {
		t.Fatalf("Usage::variantMembership = %#v; want selected's VariantMembership", variantMemberships)
	}
	namingFeature, handled, ok := evaluator.structureNamingFeature(ElementOf(variant))
	if !handled || !ok || namingFeature.Symbol != ordinary {
		t.Fatalf("VariantMembership namingFeature = %#v, %t; want referencedFeature %q", namingFeature, ok, ordinary.Name)
	}
	ownedUsages := valueElements(t, testProperty(t, evaluator, ElementOf(choice), "Definition", "ownedUsage"))
	if len(ownedUsages) != 2 || !containsElementSymbol(ownedUsages, variant) || !containsElementSymbol(ownedUsages, ordinary) {
		t.Fatalf("Definition::ownedUsage = %#v; want both owned Usage members", ownedUsages)
	}
}

func TestTransitionPayloadNamingUsesSecondInputParameter(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "transition-payload-naming.sysml", "package P {}")
	transition := Element{Aspect: "transition"}
	trigger := Element{Aspect: "trigger"}
	payload := Element{Symbol: &symbols.Symbol{Name: "payload"}}
	receiver := Element{Aspect: "receiver"}
	payloadMembership := MembershipElement(Membership{Aspect: "transition-payload-membership", kind: "ParameterMembership"})
	receiverMembership := MembershipElement(Membership{Aspect: "transition-receiver-membership", kind: "ParameterMembership"})
	triggerMembership := MembershipElement(Membership{Aspect: "transition-trigger-membership", kind: "TransitionFeatureMembership"})
	triggerPayloadMembership := MembershipElement(Membership{Aspect: "trigger-payload-membership", kind: "ParameterMembership"})
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			transition.Key():               "TestTransitionUsage",
			trigger.Key():                  "AcceptActionUsage",
			payload.Key():                  "ReferenceUsage",
			receiver.Key():                 "Feature",
			payloadMembership.Key():        "ParameterMembership",
			receiverMembership.Key():       "ParameterMembership",
			triggerMembership.Key():        "TransitionFeatureMembership",
			triggerPayloadMembership.Key(): "ParameterMembership",
		},
		ownedRelationships: map[ElementKey][]Element{
			transition.Key(): {receiverMembership, payloadMembership, triggerMembership},
			trigger.Key():    {triggerPayloadMembership},
		},
		owningRelationships: map[ElementKey]testOptionalElement{
			payload.Key(): {element: payloadMembership, present: true},
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			payloadMembership.Key(): {element: transition, present: true},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: receiverMembership.Key(), property: "memberElement"}:       {receiver},
			{element: payloadMembership.Key(), property: "memberElement"}:        {payload},
			{element: triggerMembership.Key(), property: "memberElement"}:        {trigger},
			{element: triggerPayloadMembership.Key(), property: "memberElement"}: {payload},
		},
		attributes: map[testRelatedKey]Value{
			{element: receiver.Key(), property: "direction"}:     enumValue("in"),
			{element: payload.Key(), property: "direction"}:      enumValue("in"),
			{element: triggerMembership.Key(), property: "kind"}: stringValue("trigger"),
		},
	}
	evaluator.options.Structure = structure
	namingFeature, handled, ok := evaluator.structureNamingFeature(payload)
	if !handled || !ok || namingFeature.Key() != payload.Key() {
		owningType, owningTypeOK := evaluator.Property(payload, "Feature", "owningType")
		inputs, inputsOK := evaluator.Property(transition, "Type", "input")
		actions, actionsOK := evaluator.Property(transition, "TransitionUsage", "triggerAction")
		inputReason, inputReasonRecorded := evaluator.Explain(transition, "Type", "input")
		t.Fatalf("transition payload namingFeature = %#v, %t; want trigger payload %v (owningType=%#v %t inputs=%#v %t actions=%#v %t inputReason=%#v %t)",
			namingFeature, ok, payload, owningType, owningTypeOK, inputs, inputsOK, actions, actionsOK,
			inputReason, inputReasonRecorded)
	}
}

func TestStructureBackedNestedFiltersUseActualMetaclass(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "nested-filter.sysml", "package P {}")
	owner := Element{Aspect: "owner"}
	item := Element{Aspect: "item"}
	reference := Element{Aspect: "reference"}
	itemMembership := MembershipElement(Membership{Aspect: "item-membership"})
	referenceMembership := MembershipElement(Membership{Aspect: "reference-membership"})
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			owner.Key():               "Usage",
			item.Key():                "ItemUsage",
			reference.Key():           "ReferenceUsage",
			itemMembership.Key():      "FeatureMembership",
			referenceMembership.Key(): "FeatureMembership",
		},
		ownedRelationships: map[ElementKey][]Element{
			owner.Key(): {itemMembership, referenceMembership},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: itemMembership.Key(), property: "memberElement"}:      {item},
			{element: referenceMembership.Key(), property: "memberElement"}: {reference},
		},
	}
	evaluator.options.Structure = structure
	got := valueElements(t, testProperty(t, evaluator, owner, "Usage", "nestedItem"))
	if len(got) != 1 || got[0].Key() != item.Key() {
		t.Fatalf("Usage::nestedItem = %v; want only the ItemUsage member", got)
	}
}

func TestEvaluatorChainedFeatureFeaturingType(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "chaining.sysml", `package Base {
		part def Anything;
		attribute def DataValue;
		attribute dataValues : DataValue;
	}
	package Parts {
		part def Part;
		feature parts : Part;
	}
	package P {
		part def Container {
			part source {
				part target;
			}
private feature chainedValue chains source.target;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	container := testSymbol(t, pkg.Scope, "Container")
	source := testSymbol(t, container.Scope, "source")
	chain := testSymbol(t, container.Scope, "chainedValue")
	target := testSymbol(t, source.Scope, "target")
	var chainingRelationship Element
	for _, relationship := range semantics.RelationshipsOf(chain) {
		if relationship.Kind == ast.RelChains {
			chainingRelationship = Element{Node: relationship, Container: chain}
			break
		}
	}
	chainMembership := MembershipElement(ElementOf(chain).Membership)
	sourceMembership := MembershipElement(ElementOf(source).Membership)
	targetMembership := MembershipElement(ElementOf(target).Membership)
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			ElementOf(container).Key(): "PartDefinition",
			ElementOf(chain).Key():     "Feature",
			ElementOf(source).Key():    "PartUsage",
			ElementOf(target).Key():    "PartUsage",
			chainMembership.Key():      "FeatureMembership",
			sourceMembership.Key():     "FeatureMembership",
			targetMembership.Key():     "FeatureMembership",
			chainingRelationship.Key(): "FeatureChaining",
		},
		ownedRelationships: map[ElementKey][]Element{
			ElementOf(container).Key(): {chainMembership, sourceMembership},
			ElementOf(source).Key():    {targetMembership},
			ElementOf(chain).Key():     {chainingRelationship},
			ElementOf(target).Key():    {},
		},
		owningRelationships: map[ElementKey]testOptionalElement{
			ElementOf(chain).Key():  {element: chainMembership, present: true},
			ElementOf(source).Key(): {element: sourceMembership, present: true},
			ElementOf(target).Key(): {element: targetMembership, present: true},
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			chainMembership.Key():  {element: ElementOf(container), present: true},
			sourceMembership.Key(): {element: ElementOf(container), present: true},
			targetMembership.Key(): {element: ElementOf(source), present: true},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: chainingRelationship.Key(), property: "featureChained"}:  {ElementOf(chain)},
			{element: chainingRelationship.Key(), property: "chainingFeature"}: {ElementOf(target)},
			{element: chainMembership.Key(), property: "memberElement"}:        {ElementOf(chain)},
			{element: sourceMembership.Key(), property: "memberElement"}:       {ElementOf(source)},
			{element: targetMembership.Key(), property: "memberElement"}:       {ElementOf(target)},
		},
		attributes: map[testRelatedKey]Value{
			{element: ElementOf(chain).Key(), property: "isVariable"}:  booleanValue(false),
			{element: ElementOf(target).Key(), property: "isVariable"}: booleanValue(false),
		},
	}
	evaluator.options.Structure = structure
	chainingFeatures := valueElements(t, testProperty(t, evaluator, ElementOf(chain), "Feature", "chainingFeature"))
	if len(chainingFeatures) != 1 || chainingFeatures[0].Symbol != target {
		t.Fatalf("Feature::chainingFeature = %v, want source.target", chainingFeatures)
	}
	featuringTypes := valueElements(t, testProperty(t, evaluator, ElementOf(chain), "Feature", "featuringType"))
	if !containsElementSymbol(featuringTypes, container) || !containsElementSymbol(featuringTypes, source) {
		t.Fatalf("Feature::featuringType = %v, want Container and source", featuringTypes)
	}
}

func TestStructureFeatureTargetUsesLastChainingFeature(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "feature-target.sysml", "package P {}")
	plain := Element{Aspect: "plain-feature"}
	chained := Element{Aspect: "chained-feature"}
	first := Element{Aspect: "first-chaining-feature"}
	last := Element{Aspect: "last-chaining-feature"}
	firstRelationship := Element{Aspect: "first-feature-chaining"}
	lastRelationship := Element{Aspect: "last-feature-chaining"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			plain.Key():             "Feature",
			chained.Key():           "Feature",
			first.Key():             "Feature",
			last.Key():              "Feature",
			firstRelationship.Key(): "FeatureChaining",
			lastRelationship.Key():  "FeatureChaining",
		},
		ownedRelationships: map[ElementKey][]Element{
			plain.Key():   {},
			chained.Key(): {firstRelationship, lastRelationship},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: firstRelationship.Key(), property: "featureChained"}:  {chained},
			{element: firstRelationship.Key(), property: "chainingFeature"}: {first},
			{element: lastRelationship.Key(), property: "featureChained"}:   {chained},
			{element: lastRelationship.Key(), property: "chainingFeature"}:  {last},
		},
	}
	for element, want := range map[Element]Element{
		plain:   plain,
		chained: last,
	} {
		got := testProperty(t, evaluator, element, "Feature", "featureTarget")
		if got.Kind != ElementValue || got.Element.Key() != want.Key() {
			t.Errorf("%v.featureTarget = %v, want %v", element.Key(), got, want.Key())
		}
	}
}

func TestStructureFeaturingTypesOnlyImplyFeatureMembershipOwner(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "featuring-type-ownership.sysml", `package P {
		part def Owner { part member; }
		part def PlainOwner { part plainMember; }
	}`)
	pkg := testSymbol(t, root, "P")
	ownerSymbol := testSymbol(t, pkg.Scope, "Owner")
	featureSymbol := testSymbol(t, ownerSymbol.Scope, "member")
	plainOwnerSymbol := testSymbol(t, pkg.Scope, "PlainOwner")
	plainFeatureSymbol := testSymbol(t, plainOwnerSymbol.Scope, "plainMember")
	owner := ElementOf(ownerSymbol)
	feature := ElementOf(featureSymbol)
	membership := MembershipElement(Membership{Aspect: "feature-membership", kind: "FeatureMembership"})
	structure := evaluator.options.Structure.(testStructure)
	structure.metaclasses[owner.Key()] = "PartDefinition"
	structure.metaclasses[feature.Key()] = "PartUsage"
	structure.metaclasses[membership.Key()] = "FeatureMembership"
	structure.ownedRelationships[owner.Key()] = []Element{membership}
	structure.ownedRelationships[feature.Key()] = []Element{}
	structure.owningRelationships[feature.Key()] = testOptionalElement{element: membership, present: true}
	structure.owningRelatedElements[membership.Key()] = testOptionalElement{element: owner, present: true}
	evaluator.options.Structure = structure
	got := valueElements(t, testProperty(t, evaluator, feature, "Feature", "featuringType"))
	if len(got) != 1 || got[0].Key() != owner.Key() {
		t.Fatalf("FeatureMembership-owned featuringType = %v; want owner %v", got, owner)
	}

	plainOwner := ElementOf(plainOwnerSymbol)
	plainFeature := ElementOf(plainFeatureSymbol)
	plainMembership := MembershipElement(Membership{Aspect: "plain-membership", kind: "OwningMembership"})
	structure.metaclasses[plainOwner.Key()] = "PartDefinition"
	structure.metaclasses[plainFeature.Key()] = "PartUsage"
	structure.metaclasses[plainMembership.Key()] = "OwningMembership"
	structure.ownedRelationships[plainOwner.Key()] = []Element{plainMembership}
	structure.ownedRelationships[plainFeature.Key()] = []Element{}
	structure.owningRelationships[plainFeature.Key()] = testOptionalElement{element: plainMembership, present: true}
	structure.owningRelatedElements[plainMembership.Key()] = testOptionalElement{element: plainOwner, present: true}
	evaluator.options.Structure = structure
	got = valueElements(t, testProperty(t, evaluator, plainFeature, "Feature", "featuringType"))
	if len(got) != 0 {
		t.Fatalf("OwningMembership-owned featuringType = %v; want no implied owner", got)
	}
}

func TestStructureFeaturingTypesUsesDefaultVariabilityForASTOnlyFeature(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "ast-only-featuring-type.sysml", "package P;")
	feature := Element{Node: &ast.LiteralInteger{}}
	evaluator.options.Structure = testStructure{
		specializes: func(sub, super string) bool {
			return sub == "LiteralInteger" && super == "Feature"
		},
		metaclasses: map[ElementKey]string{feature.Key(): "LiteralInteger"},
	}

	if got := testProperty(t, evaluator, feature, "Feature", "isVariable"); got.Kind != BooleanValue || got.Boolean {
		t.Fatalf("Feature::isVariable = %v, want default false", got)
	}
	if got := testProperty(t, evaluator, feature, "Feature", "featuringType"); got.Kind != SequenceValue || len(got.Values) != 0 {
		t.Fatalf("Feature::featuringType = %v, want empty", got)
	}
}

func TestStructureFeaturingTypesUsesExplicitTypeFeaturing(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "explicit-featuring-type.sysml", `package P {
		part def ExplicitType;
		part def Owner { part feature; }
	}`)
	pkg := testSymbol(t, root, "P")
	feature := ElementOf(testSymbol(t, testSymbol(t, pkg.Scope, "Owner").Scope, "feature"))
	explicitType := ElementOf(testSymbol(t, pkg.Scope, "ExplicitType"))
	typeFeaturing := Element{Aspect: "type-featuring"}
	structure := evaluator.options.Structure.(testStructure)
	structure.metaclasses[feature.Key()] = "PartUsage"
	structure.metaclasses[explicitType.Key()] = "PartDefinition"
	structure.metaclasses[typeFeaturing.Key()] = "TypeFeaturing"
	structure.ownedRelationships[feature.Key()] = []Element{typeFeaturing}
	structure.owningRelationships[feature.Key()] = testOptionalElement{present: false}
	structure.relatedElements[testRelatedKey{element: typeFeaturing.Key(), property: "featuringType"}] = []Element{explicitType}
	evaluator.options.Structure = structure
	got := valueElements(t, testProperty(t, evaluator, feature, "Feature", "featuringType"))
	if len(got) != 1 || got[0].Key() != explicitType.Key() {
		t.Fatalf("explicit featuringType = %v; want %v", got, explicitType)
	}

}

// KerML 1.1 Feature::featuringType includes the featuring Types of its typeFeaturings (lines 10211–10213).
func TestStructureFeaturingTypesIncludesEffectiveVariableUsage(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "variable-featuring-type.sysml", `package Occurrences {
		occurrence def Occurrence;
	}
	package P {
		occurrence def Vehicle :> Occurrences::Occurrence {
			part mass;
		}
	}`)
	vehicle := testSymbol(t, testSymbol(t, root, "P").Scope, "Vehicle")
	mass := testSymbol(t, vehicle.Scope, "mass")
	element := ElementOf(mass)
	if got := testProperty(t, evaluator, element, "Usage", "mayTimeVary"); got.Kind != BooleanValue || !got.Boolean {
		t.Fatalf("Usage::mayTimeVary = %v, want true", got)
	}
	if got := testProperty(t, evaluator, element, "Feature", "isVariable"); got.Kind != BooleanValue || !got.Boolean {
		t.Fatalf("Feature::isVariable = %v, want the effective mayTimeVary value true", got)
	}
	featuringTypes := valueElements(t, testProperty(t, evaluator, element, "Feature", "featuringType"))
	if len(featuringTypes) != 1 || featuringTypes[0].Key() != ElementOf(vehicle).Key() {
		t.Fatalf("variable Usage featuringType = %v, want owning Type %v",
			featuringTypes, ElementOf(vehicle))
	}
	definition, ok := ontology.PropertyOf("Usage", "mayTimeVary")
	redefinesVariable := false
	for _, property := range definition.Redefines {
		redefinesVariable = redefinesVariable || property == "Feature::isVariable"
	}
	if !ok || !redefinesVariable {
		t.Fatal("Usage::mayTimeVary must redefine Feature::isVariable in the ontology")
	}
}

func TestUsageMayTimeVaryForReferentialActionsAndStates(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "usage-may-time-vary.sysml", `package Occurrences {
		occurrence def Occurrence;
	}
	package Actions {
		action def Action;
	}
	package States {
		state def State;
	}
	package P {
		part def Vehicle :> Occurrences::Occurrence {
			perform action performed : Actions::Action;
			exhibit state exhibited : States::State;
			composite action compositeAction : Actions::Action;
		}
		perform action packageAction : Actions::Action;
	}`)
	pkg := testSymbol(t, root, "P")
	vehicle := testSymbol(t, pkg.Scope, "Vehicle")
	cases := []struct {
		symbol          *symbols.Symbol
		wantComposite   bool
		wantMayTimeVary bool
	}{
		{testSymbol(t, vehicle.Scope, "performed"), false, true},
		{testSymbol(t, vehicle.Scope, "exhibited"), false, true},
		{testSymbol(t, vehicle.Scope, "compositeAction"), true, false},
		{testSymbol(t, pkg.Scope, "packageAction"), false, false},
	}
	for _, tc := range cases {
		if got := evaluator.model.EffectiveIsComposite(tc.symbol); got != tc.wantComposite {
			t.Errorf("%s.isComposite = %t, want %t",
				symbols.FQNOf(tc.symbol), got, tc.wantComposite)
		}
		got := testProperty(t, evaluator, ElementOf(tc.symbol), "Usage", "mayTimeVary")
		if got.Kind != BooleanValue || got.Boolean != tc.wantMayTimeVary {
			t.Errorf("%s.mayTimeVary = %v, want %t",
				symbols.FQNOf(tc.symbol), got, tc.wantMayTimeVary)
		}
	}
}

func TestStructureIndividualDefinitionSelectsOnlyIndividuals(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "individual-definition.sysml", "package P {}")
	structure := evaluator.options.Structure.(testStructure)
	for _, test := range []struct {
		name         string
		isIndividual bool
	}{
		{name: "ordinary"},
		{name: "individual", isIndividual: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			usage := Element{Aspect: test.name + "-usage"}
			typing := Element{Aspect: test.name + "-typing"}
			definition := Element{Aspect: test.name + "-definition"}
			structure.metaclasses[usage.Key()] = "OccurrenceUsage"
			structure.metaclasses[typing.Key()] = "FeatureTyping"
			structure.metaclasses[definition.Key()] = "OccurrenceDefinition"
			structure.ownedRelationships[usage.Key()] = []Element{typing}
			structure.relatedElements[testRelatedKey{element: typing.Key(), property: "type"}] = []Element{definition}
			structure.attributes[testRelatedKey{element: definition.Key(), property: "isIndividual"}] =
				booleanValue(test.isIndividual)
			evaluator.options.Structure = structure

			got := testProperty(t, evaluator, usage, "OccurrenceUsage", "individualDefinition")
			if test.isIndividual {
				if got.Kind != ElementValue || got.Element.Key() != definition.Key() {
					t.Fatalf("OccurrenceUsage::individualDefinition = %v, want %v", got, definition.Key())
				}
			} else if got.Kind != NullValue {
				t.Fatalf("OccurrenceUsage::individualDefinition = %v, want null", got)
			}
		})
	}
}

func TestStructureFeatureTypesIncludeSubsettingAndLastChainingFeature(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "feature-types.sysml", `package P {
		part def A;
		part def B;
		part def Holder {
			part source : A;
			part last : B;
			part subset;
			part chained;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	holder := testSymbol(t, pkg.Scope, "Holder")
	source := ElementOf(testSymbol(t, holder.Scope, "source"))
	last := ElementOf(testSymbol(t, holder.Scope, "last"))
	subset := ElementOf(testSymbol(t, holder.Scope, "subset"))
	chained := ElementOf(testSymbol(t, holder.Scope, "chained"))
	typeA := ElementOf(testSymbol(t, pkg.Scope, "A"))
	typeB := ElementOf(testSymbol(t, pkg.Scope, "B"))
	subsetting := Element{Aspect: "feature-type-subsetting"}
	chaining := Element{Aspect: "feature-type-chaining"}
	structure := evaluator.options.Structure.(testStructure)
	structure.metaclasses[subsetting.Key()] = "Subsetting"
	structure.metaclasses[chaining.Key()] = "FeatureChaining"
	structure.metaclasses[source.Key()] = "PartUsage"
	structure.metaclasses[last.Key()] = "PartUsage"
	structure.metaclasses[subset.Key()] = "PartUsage"
	structure.metaclasses[chained.Key()] = "PartUsage"
	structure.metaclasses[typeA.Key()] = "PartDefinition"
	structure.metaclasses[typeB.Key()] = "PartDefinition"
	structure.ownedRelationships[subset.Key()] = []Element{subsetting}
	structure.ownedRelationships[chained.Key()] = []Element{chaining}
	structure.ownedRelationships[source.Key()] = []Element{}
	structure.ownedRelationships[last.Key()] = []Element{}
	structure.relatedElements[testRelatedKey{element: subsetting.Key(), property: "subsettedFeature"}] =
		[]Element{source}
	structure.relatedElements[testRelatedKey{element: chaining.Key(), property: "chainingFeature"}] =
		[]Element{source, last}
	evaluator.options.Structure = structure

	for _, test := range []struct {
		feature Element
		want    Element
	}{
		{feature: subset, want: typeA},
		{feature: chained, want: typeB},
	} {
		_, handled, ok := evaluator.structureFeatureTypes(test.feature)
		if !handled || !ok {
			t.Fatalf("Feature::type for %v is not computable", test.feature.Key())
		}
		types := valueElements(t, testProperty(t, evaluator, test.feature, "Feature", "type"))
		found := false
		for _, got := range types {
			if got.Key() == test.want.Key() {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Feature::type for %v = %v, want %v", test.feature.Key(), types, test.want.Key())
		}
	}
}

func TestStructureFeatureTypesExcludeCrossSubsetting(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "feature-type-cross-subsetting.sysml", "package P {}")
	feature := Element{Aspect: "crossing-feature"}
	crossedFeature := Element{Aspect: "crossed-feature"}
	crossSubsetting := Element{Aspect: "cross-subsetting"}
	featureTyping := Element{Aspect: "crossing-feature-typing"}
	crossedTyping := Element{Aspect: "crossed-feature-typing"}
	crossingType := Element{Aspect: "crossing-type"}
	crossedType := Element{Aspect: "crossed-type"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			feature.Key():         "Feature",
			crossedFeature.Key():  "Feature",
			crossSubsetting.Key(): "CrossSubsetting",
			featureTyping.Key():   "FeatureTyping",
			crossedTyping.Key():   "FeatureTyping",
			crossingType.Key():    "ItemDefinition",
			crossedType.Key():     "ItemDefinition",
		},
		ownedRelationships: map[ElementKey][]Element{
			feature.Key():        {featureTyping, crossSubsetting},
			crossedFeature.Key(): {crossedTyping},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: featureTyping.Key(), property: "type"}:               {crossingType},
			{element: crossSubsetting.Key(), property: "subsettedFeature"}: {crossedFeature},
			{element: crossedTyping.Key(), property: "type"}:               {crossedType},
		},
	}

	types := valueElements(t, testProperty(t, evaluator, feature, "Feature", "type"))
	if len(types) != 1 || types[0].Key() != crossingType.Key() {
		t.Fatalf("Feature::type = %#v; want only the crossing feature's own type", types)
	}
}

// KerML 1.1 typingFeatures uses only conjugator.originalType for a conjugated
// Feature (lines 10542–10556).
func TestStructureConjugatedFeatureTypeUsesOriginalType(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "conjugated-feature-type.sysml", "package P {}")
	feature := Element{Aspect: "conjugated-feature"}
	original := Element{Aspect: "original-feature"}
	conjugation := Element{Aspect: "feature-conjugation"}
	originalTyping := Element{Aspect: "original-feature-typing"}
	unrelatedTyping := Element{Aspect: "conjugated-feature-typing"}
	originalType := Element{Aspect: "original-type"}
	unrelatedType := Element{Aspect: "unrelated-type"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			feature.Key():         "Feature",
			original.Key():        "Feature",
			conjugation.Key():     "Conjugation",
			originalTyping.Key():  "FeatureTyping",
			unrelatedTyping.Key(): "FeatureTyping",
			originalType.Key():    "ItemDefinition",
			unrelatedType.Key():   "ItemDefinition",
		},
		ownedRelationships: map[ElementKey][]Element{
			feature.Key():  {conjugation, unrelatedTyping},
			original.Key(): {originalTyping},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: conjugation.Key(), property: "originalType"}: {original},
			{element: originalTyping.Key(), property: "type"}:      {originalType},
			{element: unrelatedTyping.Key(), property: "type"}:     {unrelatedType},
		},
	}

	types := valueElements(t, testProperty(t, evaluator, feature, "Feature", "type"))
	if len(types) != 1 || types[0].Key() != originalType.Key() {
		t.Fatalf("Feature::type = %#v; want only the original Feature's type", types)
	}
}

func TestStructureFeatureTypeIsUnsupportedWhenSubsettingInputHasNoType(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "feature-type-untyped-subset.sysml", "package P {}")
	feature := Element{Aspect: "feature-type-untyped-subset"}
	target := Element{Aspect: "feature-type-untyped-target"}
	subsetting := Element{Aspect: "feature-type-untyped-relationship"}
	structure := evaluator.options.Structure.(testStructure)
	structure.metaclasses[feature.Key()] = "PartUsage"
	structure.metaclasses[target.Key()] = "PartUsage"
	structure.metaclasses[subsetting.Key()] = "Subsetting"
	structure.ownedRelationships[feature.Key()] = []Element{subsetting}
	structure.ownedRelationships[target.Key()] = []Element{}
	structure.relatedElements[testRelatedKey{element: subsetting.Key(), property: "subsettedFeature"}] = []Element{target}
	evaluator.options.Structure = structure

	if _, ok := evaluator.Property(feature, "Feature", "type"); ok {
		t.Fatal("Feature::type is supported despite an untyped subsetting input")
	}
}

func TestStructureFeatureTypeIsUnsupportedWhenTypeRedundancyIsUnknown(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "feature-type-unknown-redundancy.sysml", "package P {}")
	feature := Element{Aspect: "feature-type-unknown-redundancy"}
	firstType := Element{Aspect: "feature-type-first"}
	secondType := Element{Aspect: "feature-type-second"}
	firstTyping := Element{Aspect: "feature-type-first-typing"}
	secondTyping := Element{Aspect: "feature-type-second-typing"}
	structure := evaluator.options.Structure.(testStructure)
	structure.metaclasses[feature.Key()] = "PartUsage"
	structure.metaclasses[firstType.Key()] = "PartDefinition"
	structure.metaclasses[secondType.Key()] = "PartDefinition"
	structure.metaclasses[firstTyping.Key()] = "FeatureTyping"
	structure.metaclasses[secondTyping.Key()] = "FeatureTyping"
	structure.ownedRelationships[feature.Key()] = []Element{firstTyping, secondTyping}
	structure.relatedElements[testRelatedKey{element: firstTyping.Key(), property: "type"}] = []Element{firstType}
	structure.relatedElements[testRelatedKey{element: secondTyping.Key(), property: "type"}] = []Element{secondType}
	evaluator.options.Structure = structure

	if _, ok := evaluator.Property(feature, "Feature", "type"); ok {
		t.Fatal("Feature::type is supported despite unknown type redundancy")
	}
}

func TestKerMLTypeUsageMetaclassesAreTypes(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "kerml-type-usage.sysml", `package P {}`)
	element := Element{Aspect: "kerml-type-usage"}
	structure := testStructure{metaclasses: map[ElementKey]string{
		element.Key(): "ClassUsage",
	}}
	evaluator.options.Structure = structure
	for _, metaclass := range []string{
		"AssociationUsage", "BehaviorUsage", "ClassUsage",
		"InteractionUsage", "PredicateUsage", "StructureUsage",
	} {
		structure.metaclasses[element.Key()] = metaclass
		evaluator.options.Structure = structure
		for _, expected := range []string{"Type", "Namespace", "Element"} {
			got, known := evaluator.isMetaclass(element, expected)
			if !known || !got {
				t.Errorf("%s is a %s = %t, known = %t; want true, true", metaclass, expected, got, known)
			}
		}
		if got, known := evaluator.isMetaclass(element, "Feature"); !known || got {
			t.Errorf("%s is a Feature = %t, known = %t; want false, true", metaclass, got, known)
		}
	}
}

func TestUsageReferenceAndCompositeConstraints(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "usage-composition.sysml", `package P {
		part def Vehicle {
			port ignitionCmdPort;
			attribute mass;
			part child;
			ref part spare;
			event occurrence arrived;
			port outer {
				port inner;
			}
			state machine {
				state operatingStates {
					state off;
				}
			}
		}
		port def PortShape {
			port nested;
		}
		action def Flow {
			fork f;
		}
	}`)
	vehicle := testSymbol(t, testSymbol(t, root, "P").Scope, "Vehicle")
	portShape := testSymbol(t, testSymbol(t, root, "P").Scope, "PortShape")
	flow := testSymbol(t, testSymbol(t, root, "P").Scope, "Flow")
	machine := testSymbol(t, vehicle.Scope, "machine")
	states := testSymbol(t, machine.Scope, "operatingStates")
	cases := []struct {
		symbol      *symbols.Symbol
		composite   bool
		referential bool
	}{
		{testSymbol(t, vehicle.Scope, "ignitionCmdPort"), false, true},
		{testSymbol(t, vehicle.Scope, "mass"), false, true},
		{testSymbol(t, vehicle.Scope, "child"), true, false},
		{testSymbol(t, vehicle.Scope, "spare"), false, true},
		{testSymbol(t, vehicle.Scope, "arrived"), false, true},
		{testSymbol(t, vehicle.Scope, "outer"), false, true},
		{testSymbol(t, testSymbol(t, vehicle.Scope, "outer").Scope, "inner"), true, false},
		{testSymbol(t, portShape.Scope, "nested"), true, false},
		{testSymbol(t, flow.Scope, "f"), true, false},
		{testSymbol(t, states.Scope, "off"), true, false},
	}
	evaluator.options.Structure = nil
	for _, tc := range cases {
		element := ElementOf(tc.symbol)
		composite := testProperty(t, evaluator, element, "Feature", "isComposite")
		if composite.Kind != BooleanValue || composite.Boolean != tc.composite {
			t.Errorf("%s.isComposite = %v, want %t", symbols.FQNOf(tc.symbol), composite, tc.composite)
		}
		reference := testProperty(t, evaluator, element, "Usage", "isReference")
		if reference.Kind != BooleanValue || reference.Boolean != tc.referential {
			t.Errorf("%s.isReference = %v, want %t", symbols.FQNOf(tc.symbol), reference, tc.referential)
		}
	}
}

func TestStructurePortUsageCompositionUsesOwningType(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "port-usage-composition.sysml", `package P {
		part def Vehicle { port command; }
		port def PortShape { port nested; }
		port external;
	}`)
	pkg := testSymbol(t, root, "P")
	vehicle := ElementOf(testSymbol(t, pkg.Scope, "Vehicle"))
	command := ElementOf(testSymbol(t, testSymbol(t, pkg.Scope, "Vehicle").Scope, "command"))
	portShapeSymbol := testSymbol(t, pkg.Scope, "PortShape")
	portShape := ElementOf(portShapeSymbol)
	nested := ElementOf(testSymbol(t, portShapeSymbol.Scope, "nested"))
	external := ElementOf(testSymbol(t, pkg.Scope, "external"))
	commandMembership := MembershipElement(Membership{Aspect: "command-membership", kind: "FeatureMembership"})
	nestedMembership := MembershipElement(Membership{Aspect: "nested-membership", kind: "FeatureMembership"})
	externalMembership := MembershipElement(Membership{Aspect: "external-membership", kind: "OwningMembership"})
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			vehicle.Key():            "PartDefinition",
			command.Key():            "PortUsage",
			commandMembership.Key():  "FeatureMembership",
			portShape.Key():          "PortDefinition",
			nested.Key():             "PortUsage",
			nestedMembership.Key():   "FeatureMembership",
			external.Key():           "PortUsage",
			externalMembership.Key(): "OwningMembership",
		},
		owningRelationships: map[ElementKey]testOptionalElement{
			command.Key():  {element: commandMembership, present: true},
			nested.Key():   {element: nestedMembership, present: true},
			external.Key(): {element: externalMembership, present: true},
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			commandMembership.Key():  {element: vehicle, present: true},
			nestedMembership.Key():   {element: portShape, present: true},
			externalMembership.Key(): {element: ElementOf(testSymbol(t, pkg.Scope, "external").Owner()), present: true},
		},
	}
	evaluator.options.Structure = structure
	for _, tc := range []struct {
		element   Element
		composite bool
		reference bool
	}{
		{element: command, composite: false, reference: true},
		{element: nested, composite: true, reference: false},
		{element: external, composite: false, reference: true},
	} {
		if got := testProperty(t, evaluator, tc.element, "Feature", "isComposite"); got.Kind != BooleanValue || got.Boolean != tc.composite {
			t.Errorf("%v.isComposite = %v, want %t", tc.element.Key(), got, tc.composite)
		}
		if got := testProperty(t, evaluator, tc.element, "Usage", "isReference"); got.Kind != BooleanValue || got.Boolean != tc.reference {
			t.Errorf("%v.isReference = %v, want %t", tc.element.Key(), got, tc.reference)
		}
	}
}

func TestStructureUsageReferenceUsesCompositeWithoutSymbol(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "structure-usage-reference.sysml", "package P {}")
	reference := Element{Aspect: "reference"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{reference.Key(): "ReferenceUsage"},
	}

	got := testProperty(t, evaluator, reference, "Usage", "isReference")
	if got.Kind != BooleanValue || !got.Boolean {
		t.Fatalf("Usage::isReference = %#v, want true for a non-composite ReferenceUsage", got)
	}
}

func TestStructureUsageCompositeUsesReferentialMetaclasses(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "structure-usage-referential-metaclasses.sysml", "package P {}")
	// EnumerationUsage inherits AttributeUsage's validateAttributeUsageIsReference rule.
	for _, metaclass := range []string{
		"AttributeUsage",
		"EnumerationUsage",
		"EventOccurrenceUsage",
		"PerformActionUsage",
		"ExhibitStateUsage",
		"IncludeUseCaseUsage",
		"ReferenceUsage",
	} {
		t.Run(metaclass, func(t *testing.T) {
			element := Element{Aspect: metaclass}
			evaluator.options.Structure = testStructure{
				metaclasses: map[ElementKey]string{element.Key(): metaclass},
				specializes: ontology.IsAncestorOrSelf,
			}
			got := testProperty(t, evaluator, element, "Feature", "isComposite")
			if got.Kind != BooleanValue || got.Boolean {
				t.Fatalf("Feature::isComposite = %#v, want false for %s", got, metaclass)
			}
		})
	}
}

func TestStructureUsageCompositeControlNodeRule(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "structure-usage-control-node.sysml", "package P {}")
	element := Element{Aspect: "ControlNode"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{element.Key(): "ControlNode"},
	}
	composite, handled, ok := evaluator.structureUsageComposite(element)
	if !handled || !ok || !composite {
		t.Fatalf("ControlNode.isComposite = %t, handled %t, known %t; want true (SysML.uml:11197–11203)",
			composite, handled, ok)
	}
}

func TestStructureUsageCompositeDoesNotAssumeUnruledMetaclasses(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "structure-usage-undecidable-metaclasses.sysml", "package P {}")
	for _, metaclass := range []string{
		"BindingConnectorAsUsage",
		"ConcernUsage",
		"AssertConstraintUsage",
		"ForLoopActionUsage",
		"MetadataUsage",
		"RenderingUsage",
		"SuccessionAsUsage",
	} {
		t.Run(metaclass, func(t *testing.T) {
			element := Element{Aspect: metaclass}
			evaluator.options.Structure = testStructure{
				metaclasses: map[ElementKey]string{element.Key(): metaclass},
			}
			if matches, known := evaluator.isMetaclass(element, metaclass); !known || !matches {
				t.Fatalf("%s metaclass = %t, known %t", metaclass, matches, known)
			}
			_, handled, ok := evaluator.structureUsageComposite(element)
			if !handled || ok {
				t.Fatalf("Usage::isComposite was decidable for unruled %s", metaclass)
			}
		})
	}
}

func TestStructureUsageCompositeUsesEffectiveFlags(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "structure-usage-effective-composite.sysml", `package P {
		metadata def M;
		metadata packageMarker : M;
		part topLevel;
		state failureModes[*];
		metadata m : M about topLevel;
		part def Vehicle {
			part nested;
			ref part spare;
			metadata marker : M;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	vehicle := testSymbol(t, pkg.Scope, "Vehicle")
	cases := []struct {
		symbol      *symbols.Symbol
		composite   bool
		referential bool
	}{
		{testSymbol(t, pkg.Scope, "topLevel"), false, true},
		{testSymbol(t, pkg.Scope, "failureModes"), false, true},
		{testSymbol(t, pkg.Scope, "packageMarker"), false, true},
		{testSymbol(t, pkg.Scope, "m"), false, true},
		{testSymbol(t, vehicle.Scope, "nested"), true, false},
		{testSymbol(t, vehicle.Scope, "spare"), false, true},
		{testSymbol(t, vehicle.Scope, "marker"), true, false},
	}
	for _, tc := range cases {
		element := ElementOf(tc.symbol)
		if got := testProperty(t, evaluator, element, "Feature", "isComposite"); got.Kind != BooleanValue || got.Boolean != tc.composite {
			t.Errorf("%s.isComposite = %v, want %t", symbols.FQNOf(tc.symbol), got, tc.composite)
		}
		if got := testProperty(t, evaluator, element, "Usage", "isReference"); got.Kind != BooleanValue || got.Boolean != tc.referential {
			t.Errorf("%s.isReference = %v, want %t", symbols.FQNOf(tc.symbol), got, tc.referential)
		}
	}
}

func TestEvaluatorIncludesImpliedSpecializations(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "implicit.sysml", `package Base {
		part def Anything;
	}
	package Parts {
		part def Part {
			attribute marker;
		}
	}
	package P {
		part def Local;
	}`)
	parts := testSymbol(t, root, "Parts")
	pkg := testSymbol(t, root, "P")
	base := testSymbol(t, parts.Scope, "Part")
	local := testSymbol(t, pkg.Scope, "Local")
	marker := testSymbol(t, base.Scope, "marker")
	implied := valueElements(t, testProperty(t, evaluator, ElementOf(local), "Type", "feature"))
	if !containsElementSymbol(implied, marker) {
		t.Fatalf("Local::feature = %v, want implied Part::marker", implied)
	}
}

func TestEvaluatorImportMemberships(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "imports.sysml", `package Library {
		part def Imported;
	}
	package Source {
		import Library::*;
		part def Exported {
			part child;
		}
	}
	package NamespaceImport {
		import Source::*;
	}
	package MembershipImport {
		import Source::Exported;
	}
	package RecursiveImport {
		import Source::**;
	}`)
	sourcePackage := testSymbol(t, root, "Source")
	libraryPackage := testSymbol(t, root, "Library")
	imported := testSymbol(t, libraryPackage.Scope, "Imported")
	exported := testSymbol(t, sourcePackage.Scope, "Exported")
	child := testSymbol(t, exported.Scope, "child")
	namespaceImport := testSymbol(t, root, "NamespaceImport")
	membershipImport := testSymbol(t, root, "MembershipImport")
	recursiveImport := testSymbol(t, root, "RecursiveImport")
	evaluator.options.Structure = testNamespaceStructure(
		evaluator, sourcePackage, libraryPackage, namespaceImport, membershipImport, recursiveImport,
	)

	sourceMemberships := valueMemberships(t, testProperty(t, evaluator, ElementOf(sourcePackage), "Namespace", "membership"))
	assertMembershipOrder(t, sourceMemberships, []*symbols.Symbol{exported, imported})
	namespaceMemberships := valueMemberships(t, testProperty(t, evaluator, ElementOf(namespaceImport), "Namespace", "importedMembership"))
	membershipMemberships := valueMemberships(t, testProperty(t, evaluator, ElementOf(membershipImport), "Namespace", "importedMembership"))
	recursiveMemberships := valueMemberships(t, testProperty(t, evaluator, ElementOf(recursiveImport), "Namespace", "importedMembership"))
	if !membershipHasMember(namespaceMemberships, exported) || !membershipHasMember(membershipMemberships, exported) ||
		!membershipHasMember(recursiveMemberships, exported) || !membershipHasMember(recursiveMemberships, child) ||
		!membershipHasMember(recursiveMemberships, imported) {
		t.Fatalf("import memberships omit expected elements: namespace=%v membership=%v recursive=%v",
			namespaceMemberships, membershipMemberships, recursiveMemberships)
	}
	ownedImport := valueElements(t, testProperty(t, evaluator, ElementOf(namespaceImport), "Namespace", "ownedImport"))
	if len(ownedImport) != 1 {
		t.Fatalf("Namespace::ownedImport = %v, want one import", ownedImport)
	}
	if _, ok := ownedImport[0].Node.(*ast.Import); !ok {
		t.Fatalf("owned import node = %T, want *ast.Import", ownedImport[0].Node)
	}
}

func TestEvaluatorFilteredImportMemberships(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "filtered-import.sysml", `package Library {
		part def Safe;
		part def Hidden;
	}
	package Client {
		import Library::*[@Safety];
	}`)
	library := testSymbol(t, root, "Library")
	client := testSymbol(t, root, "Client")
	safe := testSymbol(t, library.Scope, "Safe")
	evaluator.resolver.SetModel(testFilterModel{
		Model:   evaluator.model,
		allowed: map[*symbols.Symbol]bool{safe: true},
	})
	evaluator.options.Structure = testNamespaceStructure(evaluator, library, client)

	got := valueMemberships(t, testProperty(t, evaluator, ElementOf(client), "Namespace", "importedMembership"))
	if !membershipHasMember(got, safe) || membershipHasMember(got, testSymbol(t, library.Scope, "Hidden")) {
		t.Fatalf("filtered imported memberships = %v; want only Safe", got)
	}
}

func TestEvaluatorIncompleteRecursiveImportIsUnsupported(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "incomplete-import.sysml", `package Broken {
		import Missing::*;
	}
	package Client {
		import Broken::**;
	}`)
	broken := testSymbol(t, root, "Broken")
	client := testSymbol(t, root, "Client")
	evaluator.options.Structure = testNamespaceStructure(evaluator, broken, client)

	if _, ok := evaluator.Property(ElementOf(client), "Namespace", "importedMembership"); ok {
		t.Fatal("Namespace::importedMembership succeeded with an incomplete recursive import")
	}
}

func TestImportImportedElementIsSingleValued(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "imported-element.sysml", `package Library {
		part def Imported;
	}
	package Client {
		import Library::Imported;
	}`)
	client := testSymbol(t, root, "Client")
	library := testSymbol(t, root, "Library")
	evaluator.options.Structure = testNamespaceStructure(evaluator, client, library)
	imports := valueElements(t, testProperty(t, evaluator, ElementOf(client), "Namespace", "ownedImport"))
	if len(imports) != 1 {
		t.Fatalf("Namespace::ownedImport = %v, want one import", imports)
	}
	value := testProperty(t, evaluator, imports[0], "Import", "importedElement")
	if value.Kind != ElementValue || value.Element.Symbol != testSymbol(t, testSymbol(t, root, "Library").Scope, "Imported") {
		t.Fatalf("Import::importedElement = %#v, want the Imported definition", value)
	}
}

func TestEvaluatorRelationshipProperties(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "relationships.sysml", `package P {
		part def Base;
		part def Derived specializes Base;
	}`)
	pkg := testSymbol(t, root, "P")
	derived := testSymbol(t, pkg.Scope, "Derived")
	relationships := semantics.RelationshipsOf(derived)
	if len(relationships) != 1 {
		t.Fatalf("Derived relationships = %d, want one", len(relationships))
	}
	element := Element{Node: relationships[0], Container: derived}
	for _, property := range []string{"source", "target", "relatedElement", "owningRelatedElement", "ownedRelatedElement"} {
		testProperty(t, evaluator, element, "Relationship", property)
	}
	target := valueElements(t, testProperty(t, evaluator, element, "Relationship", "target"))
	if len(target) != 1 || target[0].Symbol != testSymbol(t, pkg.Scope, "Base") {
		t.Fatalf("Relationship::target = %v, want Base", target)
	}
	owningType := testProperty(t, evaluator, element, "Specialization", "owningType")
	if owningType.Kind != ElementValue || owningType.Element.Symbol != derived {
		t.Fatalf("Specialization::owningType = %#v, want Derived", owningType)
	}
}

func TestRelationshipEndpointsPreserveRelatedElementOccurrences(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "repeated-related-element.sysml", "package P {}")
	relationship := Element{Aspect: "repeated-related-element"}
	endpoint := Element{Aspect: "self-loop-endpoint"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			relationship.Key(): "Relationship",
			endpoint.Key():     "Feature",
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: relationship.Key(), property: "source"}: {endpoint},
			{element: relationship.Key(), property: "target"}: {endpoint},
		},
	}

	related, ok := evaluator.relationshipEndpoints(relationship, "relatedElement")
	if !ok || len(related) != 2 ||
		related[0].Element.Key() != endpoint.Key() ||
		related[1].Element.Key() != endpoint.Key() {
		t.Fatalf("Relationship::relatedElement = %#v, %t; want the endpoint twice", related, ok)
	}
}

func TestRelationshipEndpointsOnMembershipUseNamespaceAndMember(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "membership-endpoints.sysml", `package P {
		part def Owner {
			part member;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	owner := testSymbol(t, pkg.Scope, "Owner")
	member := testSymbol(t, owner.Scope, "member")
	membership := MembershipElement(ElementOf(member).Membership)

	source := valueElements(t, testProperty(t, evaluator, membership, "Relationship", "source"))
	if len(source) != 1 || source[0].Symbol != owner {
		t.Fatalf("Relationship::source on membership = %v, want Owner", source)
	}
	target := valueElements(t, testProperty(t, evaluator, membership, "Relationship", "target"))
	if len(target) != 1 || target[0].Symbol != member {
		t.Fatalf("Relationship::target on membership = %v, want member", target)
	}
}

func TestEvaluatorDoesNotReturnPartialValuesForUnresolvedSupertype(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "unresolved.sysml", `package P {
		part def Broken specializes Missing {
			attribute own;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	broken := testSymbol(t, pkg.Scope, "Broken")
	if _, ok := evaluator.Property(ElementOf(broken), "Type", "feature"); ok {
		t.Fatal("Type::feature succeeded with an unresolved supertype")
	}
}

func TestFeatureOwningTypeRequiresAFeatureMembershipOwnedByAType(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "owning-type.sysml", `package P {
		part loose;
		part def Owner {
			part nested;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	loose := testSymbol(t, pkg.Scope, "loose")
	owner := testSymbol(t, pkg.Scope, "Owner")
	nested := testSymbol(t, owner.Scope, "nested")

	if got := testProperty(t, evaluator, ElementOf(loose), "Feature", "owningType"); got.Kind != NullValue {
		t.Fatalf("package-owned Feature::owningType = %#v, want null", got)
	}
	if got := testProperty(t, evaluator, ElementOf(nested), "Feature", "owningType"); got.Kind != ElementValue || got.Element.Symbol != owner {
		t.Fatalf("type-owned Feature::owningType = %#v, want Owner", got)
	}
}

func TestSpecializationOwningTypeUsesATypeOwner(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "specialization-owning-type.sysml", `package P {
		part def Base;
		part value : Base;
	}`)
	pkg := testSymbol(t, root, "P")
	value := testSymbol(t, pkg.Scope, "value")
	relationships := semantics.RelationshipsOf(value)
	if len(relationships) == 0 {
		t.Fatal("typed feature has no relationships")
	}

	typing := Element{Node: relationships[0], Container: value}
	got := testProperty(t, evaluator, typing, "Specialization", "owningType")
	if got.Kind != ElementValue || got.Element.Symbol != value {
		t.Fatalf("FeatureTyping::owningType = %#v, want the typed feature", got)
	}

	unownedType := testProperty(t, evaluator, Element{Node: &ast.Relationship{}, Container: pkg}, "Specialization", "owningType")
	if unownedType.Kind != NullValue {
		t.Fatalf("package-owned Specialization::owningType = %#v, want null", unownedType)
	}
}

func TestUsageOwningUsageUsesOwningTypeMetaclass(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "owning-usage-metaclass.sysml", `package P {
		part def outer {
			part nested;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	outer := testSymbol(t, pkg.Scope, "outer")
	nested := testSymbol(t, outer.Scope, "nested")
	nestedMembership := MembershipElement(ElementOf(nested).Membership)
	evaluator = New(evaluator.resolver, evaluator.model, Options{
		Structure: testStructure{
			metaclasses: map[ElementKey]string{
				ElementOf(outer).Key():  "PartDefinition",
				ElementOf(nested).Key(): "PartUsage",
				nestedMembership.Key():  "FeatureMembership",
			},
			owningRelationships: map[ElementKey]testOptionalElement{
				ElementOf(nested).Key(): {element: nestedMembership, present: true},
			},
			owningRelatedElements: map[ElementKey]testOptionalElement{
				nestedMembership.Key(): {element: ElementOf(outer), present: true},
			},
		},
	})

	got := testProperty(t, evaluator, ElementOf(nested), "Usage", "owningUsage")
	if got.Kind != NullValue {
		t.Fatalf("Usage::owningUsage = %#v, want null because owningType is a Definition", got)
	}
}

func TestStructureMembershipNamesUseGraphAttributes(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "membership-names.sysml", "package P {}")
	named := MembershipElement(Membership{Aspect: "named-membership", kind: "Membership"})
	unnamed := MembershipElement(Membership{Aspect: "unnamed-membership", kind: "Membership"})
	root := Element{Aspect: "root-membership-namespace"}
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			named.Key():   "Membership",
			unnamed.Key(): "Membership",
			root.Key():    "Namespace",
		},
		ownedRelationships: map[ElementKey][]Element{
			root.Key(): {named, unnamed},
		},
		owningRelationships: map[ElementKey]testOptionalElement{
			root.Key(): {present: false},
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			named.Key():   {element: root, present: true},
			unnamed.Key(): {element: root, present: true},
			root.Key():    {present: false},
		},
		attributes: map[testRelatedKey]Value{
			{element: named.Key(), property: "memberName"}:      stringValue("membershipName"),
			{element: named.Key(), property: "memberShortName"}: stringValue("mn"),
		},
	}
	evaluator.options.Structure = structure

	for _, test := range []struct {
		property string
		want     string
	}{
		{"memberName", "membershipName"},
		{"memberShortName", "mn"},
	} {
		got := testProperty(t, evaluator, named, "Membership", test.property)
		if got.Kind != StringValue || got.String != test.want {
			t.Errorf("Membership::%s = %#v; want %q", test.property, got, test.want)
		}
	}
	for _, property := range []string{"name", "shortName"} {
		got := testProperty(t, evaluator, named, "Element", property)
		if got.Kind != NullValue {
			t.Errorf("Membership Element::%s = %#v; want null", property, got)
		}
	}
	qualifiedName := testProperty(t, evaluator, named, "Element", "qualifiedName")
	if qualifiedName.Kind != NullValue {
		t.Fatalf("Membership::qualifiedName = %#v; want null without owningNamespace", qualifiedName)
	}
	for _, property := range []string{"name", "shortName"} {
		got := testProperty(t, evaluator, unnamed, "Element", property)
		if got.Kind != NullValue {
			t.Errorf("unnamed Membership::%s = %#v; want null", property, got)
		}
	}
}

func TestOwnedAnnotationSelectsAnnotationRelationships(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "owned-annotation.sysml", `package P {
		metadata def M {
			attribute value;
		}
		part annotated {
			@M { value = 1; }
		}
	}`)
	pkg := testSymbol(t, root, "P")
	annotated := testSymbol(t, pkg.Scope, "annotated")
	got := valueElements(t, testProperty(t, evaluator, ElementOf(annotated), "Element", "ownedAnnotation"))
	if len(got) != 0 {
		t.Fatalf("Element::ownedAnnotation = %v, want no Annotation relationship for a metadata usage", got)
	}
}

func TestStructureAboutTargetsOverrideCollapsedAnnotatedElement(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "annotation-about-targets.sysml", `package P {
		metadata def M;
		part def A;
		part a : A;
		metadata m : M about A, a;
	}`)
	pkg := testSymbol(t, root, "P")
	metadata := testSymbol(t, pkg.Scope, "m")
	definition := testSymbol(t, pkg.Scope, "A")
	usage := testSymbol(t, pkg.Scope, "a")
	element := ElementOf(metadata)
	element.Symbol = nil
	element.Container = pkg
	collapsedTarget := Element{Aspect: "collapsed-about-target"}
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			element.Key():         "MetadataUsage",
			collapsedTarget.Key(): "Element",
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: element.Key(), property: "annotatedElement"}: {collapsedTarget},
		},
		statedRelated: map[testRelatedKey]bool{
			{element: element.Key(), property: "annotatedElement"}: true,
		},
	}

	got := valueElements(t, testProperty(t, evaluator, element, "AnnotatingElement", "annotatedElement"))
	if len(got) != 2 || got[0].Key() != ElementOf(definition).Key() || got[1].Key() != ElementOf(usage).Key() {
		t.Fatalf("AnnotatingElement::annotatedElement = %#v; want the resolved about targets in order", got)
	}
}

func TestStructureAnnotatingElementFallsBackToOwningNamespace(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "annotation-fallback.sysml", "package P {}")
	element := Element{Aspect: "test-annotating-element"}
	namespace := Element{Aspect: "test-namespace"}
	membership := MembershipElement(Membership{Aspect: "test-membership", kind: "OwningMembership"})
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			element.Key():    "Comment",
			namespace.Key():  "Package",
			membership.Key(): "OwningMembership",
		},
		ownedRelationships: map[ElementKey][]Element{
			element.Key():    {},
			namespace.Key():  {},
			membership.Key(): {},
		},
		owningRelationships: map[ElementKey]testOptionalElement{
			element.Key(): {element: membership, present: true},
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			membership.Key(): {element: namespace, present: true},
		},
		specializes: func(sub, super string) bool {
			return sub == "Package" && super == "Namespace" || testSpecializes(sub, super)
		},
	}

	value, ok := evaluator.Property(element, "AnnotatingElement", "annotatedElement")
	if !ok || value.Kind != SequenceValue || len(value.Values) != 1 ||
		value.Values[0].Kind != ElementValue || value.Values[0].Element.Key() != namespace.Key() {
		t.Fatalf("AnnotatingElement::annotatedElement = %#v, %t; want owning namespace %v", value, ok, namespace)
	}
}

func TestStructureAnnotatedElementUsesStatedTargetsBeforeNamespaceFallback(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "annotation-targets.sysml", "package P {}")
	element := Element{Aspect: "metadata"}
	namespace := Element{Aspect: "namespace"}
	target := Element{Aspect: "about-target"}
	membership := MembershipElement(Membership{Aspect: "metadata-membership", kind: "OwningMembership"})
	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			element.Key():    "MetadataUsage",
			namespace.Key():  "Package",
			target.Key():     "PartDefinition",
			membership.Key(): "OwningMembership",
		},
		ownedRelationships: map[ElementKey][]Element{
			element.Key():    {},
			membership.Key(): {},
		},
		owningRelationships: map[ElementKey]testOptionalElement{
			element.Key(): {element: membership, present: true},
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			membership.Key(): {element: namespace, present: true},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: element.Key(), property: "annotatedElement"}: {target},
		},
		statedRelated: map[testRelatedKey]bool{
			{element: element.Key(), property: "annotatedElement"}: true,
		},
		specializes: func(sub, super string) bool {
			return sub == "MetadataUsage" && super == "AnnotatingElement" ||
				sub == "Package" && super == "Namespace" || testSpecializes(sub, super)
		},
	}

	got, ok := evaluator.Property(element, "AnnotatingElement", "annotatedElement")
	if !ok || got.Kind != SequenceValue || len(got.Values) != 1 ||
		got.Values[0].Kind != ElementValue || got.Values[0].Element.Key() != target.Key() {
		t.Fatalf("AnnotatingElement::annotatedElement = %#v, %t; want stated about target %s", got, ok, target.Aspect)
	}
}

func TestStructureAboutMetadataUsesAnnotatedTargets(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "metadata-about-targets.sysml", `package P {
		metadata def Meta { attribute flag : Boolean; }
		part def Target;
		part a : Target;
		part b : Target;
		@Meta about a, b;
	}`)
	pkg := testSymbol(t, root, "P")
	var metadata *symbols.Symbol
	for _, candidate := range pkg.Scope.AllMembers() {
		prefix, ok := candidate.Decl.(*ast.PrefixMetadata)
		if ok && len(prefix.About) > 0 {
			metadata = candidate
			break
		}
	}
	if metadata == nil {
		t.Fatal("package has no about-target metadata usage")
	}
	value, handled, ok := evaluator.structureAnnotatedElements(ElementOf(metadata))
	if !handled || !ok || value.Kind != SequenceValue || len(value.Values) != 2 {
		t.Fatalf("AnnotatingElement::annotatedElement = %#v, %t, %t; want both about targets", value, handled, ok)
	}
	want := []*symbols.Symbol{
		testSymbol(t, pkg.Scope, "a"),
		testSymbol(t, pkg.Scope, "b"),
	}
	for i, symbol := range want {
		if value.Values[i].Kind != ElementValue || value.Values[i].Element.Key() != ElementOf(symbol).Key() {
			t.Fatalf("AnnotatingElement::annotatedElement[%d] = %#v; want %s", i, value.Values[i], symbols.FQNOf(symbol))
		}
	}
}

func TestFeatureMembershipPropertiesExcludeVariantsAndNonFeatureMemberships(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "feature-membership.sysml", `package Parts {
		part def Part;
	}
	package P {
		part def Base {
			attribute inherited;
		}
		variation part def Choice specializes Base {
			variant part alternative;
			attribute local;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	choice := testSymbol(t, pkg.Scope, "Choice")
	inherited := testSymbol(t, testSymbol(t, pkg.Scope, "Base").Scope, "inherited")
	local := testSymbol(t, choice.Scope, "local")
	alternative := testSymbol(t, choice.Scope, "alternative")

	owned := valueMemberships(t, testProperty(t, evaluator, ElementOf(choice), "Type", "ownedFeatureMembership"))
	if !membershipHasMember(owned, local) || membershipHasMember(owned, alternative) {
		t.Fatalf("Type::ownedFeatureMembership = %v, want local only", membershipMembers(owned))
	}
	effective := valueMemberships(t, testProperty(t, evaluator, ElementOf(choice), "Type", "featureMembership"))
	if !membershipHasMember(effective, inherited) || !membershipHasMember(effective, local) || membershipHasMember(effective, alternative) {
		t.Fatalf("Type::featureMembership = %v, want inherited and local only", membershipMembers(effective))
	}
}

func TestInheritedFeatureMembershipFilterUsesStructureMetaclass(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "inherited-feature-membership.sysml", `package P {
		part def Base {
			attribute inherited;
		}
		part def Choice specializes Base;
	}`)
	pkg := testSymbol(t, root, "P")
	base := testSymbol(t, pkg.Scope, "Base")
	inherited := testSymbol(t, base.Scope, "inherited")
	membership := ElementOf(inherited).Membership
	if !membership.Feature {
		t.Fatal("test fixture must identify the inherited member as a FeatureMembership")
	}

	structure := evaluator.options.Structure.(testStructure)
	structure.metaclasses[MembershipElement(membership).Key()] = "VariantMembership"
	evaluator.options.Structure = structure

	got, ok := evaluator.structureFeatureMemberships([]*symbols.Symbol{inherited})
	if !ok {
		t.Fatal("Structure did not classify the inherited membership")
	}
	if len(got) != 0 {
		t.Fatalf("inherited feature memberships = %v, want Structure's VariantMembership classification to exclude inherited", membershipMembers(got))
	}
}

func TestNestedMetaclassFiltersUseTheInjectedClassifier(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "nested-filter.sysml", `package Parts {
		part def Part;
	}
	package P {
		item def Item;
		part outer {
			ref reference : Item;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	outer := testSymbol(t, pkg.Scope, "outer")
	reference := testSymbol(t, outer.Scope, "reference")
	if got := valueElements(t, testProperty(t, evaluator, ElementOf(outer), "Usage", "nestedItem")); len(got) != 0 {
		t.Fatalf("Usage::nestedItem = %v, want no ReferenceUsage", got)
	}
	if got := valueElements(t, testProperty(t, evaluator, ElementOf(outer), "Usage", "nestedUsage")); len(got) != 1 || got[0].Symbol != reference {
		t.Fatalf("Usage::nestedUsage = %v, want the ReferenceUsage", got)
	}

	p := parser.New(source.New("unclassified.sysml", []byte(`package Parts { part def Part; } package P { part outer { part nested; } }`)))
	rootNode := p.ParseFile()
	index := symbols.NewIndex()
	index.AddDocumentWithKind("unclassified.sysml", rootNode, source.KindOf("unclassified.sysml"))
	resolver := resolve.New(index)
	model := semantics.NewModel(resolver)
	resolver.SetModel(model)
	resolver.ResolveDocument("unclassified.sysml", rootNode)
	unclassified := New(resolver, model)
	outer = testSymbol(t, index.DocumentRoot("unclassified.sysml"), "P")
	outer = testSymbol(t, outer.Scope, "outer")
	if _, ok := unclassified.Property(ElementOf(outer), "Usage", "nestedPart"); ok {
		t.Fatal("nested metaclass filter succeeded without a classifier")
	}
}

func TestDefinitionOwnedItemFiltersByActualMetaclass(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "definition-owned-item.sysml", `package P {
		item def Item;
		part def Owner {
			item itemChild;
			ref referenceChild : Item;
		}
	}`)
	owner := testSymbol(t, testSymbol(t, root, "P").Scope, "Owner")
	itemChild := testSymbol(t, owner.Scope, "itemChild")
	referenceChild := testSymbol(t, owner.Scope, "referenceChild")
	ownedItems := valueElements(t, testProperty(t, evaluator, ElementOf(owner), "Definition", "ownedItem"))
	if len(ownedItems) != 1 || ownedItems[0].Symbol != itemChild {
		t.Fatalf("Definition::ownedItem = %v, want itemChild and not ReferenceUsage", ownedItems)
	}
	if containsElementSymbol(ownedItems, referenceChild) {
		t.Fatalf("Definition::ownedItem includes ReferenceUsage %s", referenceChild.Name)
	}
}

func TestTypedManyDefinitionReturnsASequence(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "typed-many-definition.sysml", `package Parts {
		part def Part;
		part parts : Part;
	}
	package P {
		part def Wheel;
		part def Vehicle {
			part wheel : Wheel;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	vehicle := testSymbol(t, pkg.Scope, "Vehicle")
	wheel := testSymbol(t, vehicle.Scope, "wheel")
	wheelDefinition := testSymbol(t, pkg.Scope, "Wheel")
	value := testProperty(t, evaluator, ElementOf(wheel), "PartUsage", "partDefinition")
	got := valueElements(t, value)
	if len(got) != 1 || got[0].Symbol != wheelDefinition {
		t.Fatalf("PartUsage::partDefinition = %v, want [Wheel]", got)
	}
}

func TestStepBehaviorReturnsASequence(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "step-behavior.sysml", `package Actions {
		action def Action;
		action actions : Action;
	}
	package P {
		action def Behavior specializes Actions::Action;
		action run : Behavior;
	}`)
	pkg := testSymbol(t, root, "P")
	run := testSymbol(t, pkg.Scope, "run")
	behavior := testSymbol(t, pkg.Scope, "Behavior")
	got := valueElements(t, testProperty(t, evaluator, ElementOf(run), "Step", "behavior"))
	if len(got) != 1 || got[0].Symbol != behavior {
		t.Fatalf("Step::behavior = %v, want [Behavior]", got)
	}
}

func TestTransitionGuardExpressionReturnsASequence(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "transition-guard.sysml", `package P {
		state def Machine {
			state off;
			state on;
			transition first off if true then on;
		}
	}`)
	machine := testSymbol(t, testSymbol(t, root, "P").Scope, "Machine")
	var transition *symbols.Symbol
	for _, member := range machine.Scope.AllMembers() {
		if _, ok := member.Decl.(*ast.TransitionMember); ok {
			transition = member
			break
		}
	}
	if transition == nil {
		t.Fatal("resolver omitted transition member")
	}
	transitionNode := transition.Decl.(*ast.TransitionMember)
	guard := Element{Node: transitionNode.Guard, Container: transition}
	structure := evaluator.options.Structure.(testStructure)
	structure.metaclasses[guard.Key()] = "Expression"
	structure.relatedElements[testRelatedKey{element: ElementOf(transition).Key(), property: "guardExpression"}] =
		[]Element{guard}
	evaluator.options.Structure = structure
	got := valueElements(t, testProperty(t, evaluator, ElementOf(transition), "TransitionUsage", "guardExpression"))
	if len(got) != 1 {
		t.Fatalf("TransitionUsage::guardExpression = %v, want one expression", got)
	}
	if _, ok := got[0].Node.(*ast.LiteralBool); !ok {
		t.Fatalf("guard expression node = %T, want *ast.LiteralBool", got[0].Node)
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
		if !containsSymbol(out, member) {
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

func TestStructureBackedOwnedRelationshipsAndTypeOperands(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "m.sysml", "package P {}")
	typeElement := Element{Aspect: "test-type"}
	differencing := Element{Aspect: "test-differencing"}
	base := Element{Aspect: "test-base"}
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			typeElement.Key():  "PartDefinition",
			differencing.Key(): "Differencing",
			base.Key():         "PartDefinition",
		},
		ownedRelationships: map[ElementKey][]Element{
			typeElement.Key(): {differencing},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: differencing.Key(), property: "typeDifferenced"}:  {typeElement},
			{element: differencing.Key(), property: "differencingType"}: {base},
		},
	}
	evaluator.options.Structure = structure

	owned, ok := evaluator.Property(typeElement, "Type", "ownedDifferencing")
	if !ok || owned.Kind != SequenceValue || len(owned.Values) != 1 ||
		owned.Values[0].Kind != ElementValue || owned.Values[0].Element.Key() != differencing.Key() {
		t.Fatalf("Type::ownedDifferencing = %#v, %t; want the graph-owned Differencing", owned, ok)
	}
	operand, ok := evaluator.Property(typeElement, "Type", "differencingType")
	if !ok || operand.Kind != SequenceValue || len(operand.Values) != 1 ||
		operand.Values[0].Kind != ElementValue || operand.Values[0].Element.Key() != base.Key() {
		t.Fatalf("Type::differencingType = %#v, %t; want the graph-stated operand", operand, ok)
	}
}

func TestStructureBackedRelationshipEndpointsRespectMetaclassRedefinitions(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "m.sysml", "package P {}")
	namespace := Element{Aspect: "test-namespace"}
	member := Element{Aspect: "test-member"}
	membership := MembershipElement(Membership{Aspect: "test-membership"})
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			namespace.Key():  "Package",
			member.Key():     "PartUsage",
			membership.Key(): "Membership",
		},
		ownedRelationships: map[ElementKey][]Element{
			namespace.Key():  {},
			member.Key():     {},
			membership.Key(): {},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: membership.Key(), property: "memberElement"}: {member},
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			membership.Key(): {element: namespace, present: true},
		},
	}
	evaluator.options.Structure = structure

	for _, property := range []struct {
		class string
		name  string
		want  Element
	}{
		{class: "Relationship", name: "source", want: namespace},
		{class: "Relationship", name: "target", want: member},
		{class: "Membership", name: "memberElement", want: member},
	} {
		got, ok := evaluator.Property(membership, property.class, property.name)
		if property.class == "Relationship" {
			gotElements := valueElements(t, got)
			if !ok || len(gotElements) != 1 || gotElements[0].Key() != property.want.Key() {
				t.Errorf("%s::%s = %#v, %t; want sequence containing %v", property.class, property.name, got, ok, property.want)
			}
			continue
		}
		if !ok || got.Kind != ElementValue || got.Element.Key() != property.want.Key() {
			t.Errorf("%s::%s = %#v, %t; want %v", property.class, property.name, got, ok, property.want)
		}
	}
}

func TestStructureBackedAnnotationEndpointsUseTheirRedefinitions(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "m.sysml", "package P {}")
	annotation := Element{Aspect: "test-annotation"}
	annotatingElement := Element{Aspect: "test-annotating-element"}
	annotatedElement := Element{Aspect: "test-annotated-element"}
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			annotation.Key():        "Annotation",
			annotatingElement.Key(): "MetadataUsage",
			annotatedElement.Key():  "PartDefinition",
		},
		ownedRelationships: map[ElementKey][]Element{
			annotation.Key():        {},
			annotatingElement.Key(): {annotation},
			annotatedElement.Key():  {},
		},
		owningRelationships: map[ElementKey]testOptionalElement{},
		specializes: func(sub, super string) bool {
			if sub == "MetadataUsage" && super == "AnnotatingElement" {
				return true
			}
			return testSpecializes(sub, super)
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			annotation.Key(): {element: annotatingElement, present: true},
		},
		ownedRelatedElements: map[ElementKey][]Element{
			annotation.Key(): {annotatingElement},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: annotation.Key(), property: "annotatingElement"}: {annotatingElement},
			{element: annotation.Key(), property: "annotatedElement"}:  {annotatedElement},
		},
	}
	evaluator.options.Structure = structure

	for _, property := range []struct {
		element Element
		class   string
		name    string
		want    Element
	}{
		{element: annotation, class: "Relationship", name: "source", want: annotatingElement},
		{element: annotation, class: "Annotation", name: "annotatingElement", want: annotatingElement},
		{element: annotation, class: "Relationship", name: "target", want: annotatedElement},
		{element: annotation, class: "Annotation", name: "annotatedElement", want: annotatedElement},
		{element: annotation, class: "Annotation", name: "ownedAnnotatingElement", want: annotatingElement},
		{element: annotation, class: "Annotation", name: "owningAnnotatingElement", want: annotatingElement},
		{element: annotation, class: "Annotation", name: "owningAnnotatedElement", want: annotatedElement},
		{element: annotatingElement, class: "AnnotatingElement", name: "annotation", want: annotation},
		{element: annotatingElement, class: "AnnotatingElement", name: "ownedAnnotatingRelationship", want: annotation},
		{element: annotatingElement, class: "AnnotatingElement", name: "annotatedElement", want: annotatedElement},
	} {
		got, ok := evaluator.Property(property.element, property.class, property.name)
		matches := got.Kind == ElementValue && got.Element.Key() == property.want.Key()
		if got.Kind == SequenceValue {
			for _, element := range valueElements(t, got) {
				matches = matches || element.Key() == property.want.Key()
			}
		}
		if !ok || !matches {
			t.Errorf("%s::%s = %#v, %t; want %v", property.class, property.name, got, ok, property.want)
		}
	}

	owningAnnotation := Element{Aspect: "test-owning-annotation"}
	structure.metaclasses[owningAnnotation.Key()] = "Annotation"
	structure.ownedRelationships[owningAnnotation.Key()] = []Element{}
	structure.owningRelationships[annotatingElement.Key()] =
		testOptionalElement{element: owningAnnotation, present: true}
	structure.relatedElements[testRelatedKey{element: owningAnnotation.Key(), property: "annotatedElement"}] =
		[]Element{annotatedElement}
	evaluator = New(evaluator.resolver, evaluator.model, Options{Structure: structure})
	annotations, ok := evaluator.Property(annotatingElement, "AnnotatingElement", "annotation")
	if !ok || annotations.Kind != SequenceValue || len(annotations.Values) != 2 ||
		annotations.Values[0].Element.Key() != owningAnnotation.Key() ||
		annotations.Values[1].Element.Key() != annotation.Key() {
		t.Fatalf("AnnotatingElement::annotation = %#v, %t; want owning Annotation prepended to owned Annotation", annotations, ok)
	}
}

func TestStructureBackedRelationshipSourceAndTargetUseRedefinitions(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "relationship-endpoints.sysml", "package P {}")
	type endpointCase struct {
		class          string
		sourceProperty string
		targetProperty string
	}
	cases := []endpointCase{
		{class: "Specialization", sourceProperty: "specific", targetProperty: "general"},
		{class: "FeatureTyping", sourceProperty: "typedFeature", targetProperty: "type"},
		{class: "Subsetting", sourceProperty: "subsettingFeature", targetProperty: "subsettedFeature"},
		{class: "ReferenceSubsetting", sourceProperty: "referencingFeature", targetProperty: "referencedFeature"},
		{class: "Redefinition", sourceProperty: "redefiningFeature", targetProperty: "redefinedFeature"},
		{class: "Import", sourceProperty: "importOwningNamespace", targetProperty: "importedElement"},
		{class: "Annotation", sourceProperty: "annotatingElement", targetProperty: "annotatedElement"},
		{class: "FeatureValue", sourceProperty: "featureWithValue", targetProperty: "value"},
		{class: "Dependency", sourceProperty: "client", targetProperty: "supplier"},
	}
	structure := testStructure{
		metaclasses:           make(map[ElementKey]string),
		ownedRelationships:    make(map[ElementKey][]Element),
		owningRelatedElements: make(map[ElementKey]testOptionalElement),
		relatedElements:       make(map[testRelatedKey][]Element),
	}
	type endpoints struct {
		relationship Element
		source       Element
		target       Element
	}
	var graph []endpoints
	for _, test := range cases {
		relationship := Element{Aspect: "relationship-" + test.class}
		source := Element{Aspect: "source-" + test.class}
		target := Element{Aspect: "target-" + test.class}
		structure.metaclasses[relationship.Key()] = test.class
		structure.metaclasses[source.Key()] = "Element"
		structure.metaclasses[target.Key()] = "Element"
		structure.ownedRelationships[relationship.Key()] = []Element{}
		switch test.sourceProperty {
		case "importOwningNamespace", "annotatingElement", "featureWithValue":
			structure.owningRelatedElements[relationship.Key()] =
				testOptionalElement{element: source, present: true}
		default:
			structure.relatedElements[testRelatedKey{element: relationship.Key(), property: test.sourceProperty}] =
				[]Element{source}
		}
		structure.relatedElements[testRelatedKey{element: relationship.Key(), property: test.targetProperty}] = []Element{target}
		graph = append(graph, endpoints{relationship: relationship, source: source, target: target})
	}
	evaluator.options.Structure = structure
	for _, test := range graph {
		for _, endpoint := range []struct {
			property string
			want     Element
		}{
			{property: "source", want: test.source},
			{property: "target", want: test.target},
		} {
			got, ok := evaluator.Property(test.relationship, "Relationship", endpoint.property)
			gotElements := valueElements(t, got)
			if !ok || len(gotElements) != 1 || gotElements[0].Key() != endpoint.want.Key() {
				t.Errorf("%s::%s = %#v, %t; want %v", structure.metaclasses[test.relationship.Key()], endpoint.property, got, ok, endpoint.want)
			}
		}
	}
}

func TestStructureBackedOwningRelationshipOpposites(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "relationship-opposites.sysml", "package P {}")
	type ownerCase struct {
		class      string
		property   string
		ownerClass string
	}
	cases := []ownerCase{
		{class: "Conjugation", property: "owningType", ownerClass: "PartDefinition"},
		{class: "Disjoining", property: "owningType", ownerClass: "PartDefinition"},
		{class: "Differencing", property: "owningType", ownerClass: "PartDefinition"},
		{class: "Intersecting", property: "owningType", ownerClass: "PartDefinition"},
		{class: "Unioning", property: "owningType", ownerClass: "PartDefinition"},
		{class: "Specialization", property: "owningType", ownerClass: "PartDefinition"},
		{class: "FeatureTyping", property: "owningFeature", ownerClass: "PartUsage"},
		{class: "FeatureInverting", property: "owningFeature", ownerClass: "PartUsage"},
		{class: "CrossSubsetting", property: "owningFeature", ownerClass: "PartUsage"},
		{class: "FeatureChaining", property: "owningFeature", ownerClass: "PartUsage"},
		{class: "FeatureMembership", property: "owningType", ownerClass: "PartDefinition"},
		{class: "Import", property: "importOwningNamespace", ownerClass: "Package"},
		{class: "Annotation", property: "owningAnnotatedElement", ownerClass: "PartDefinition"},
		{class: "Documentation", property: "documentedElement", ownerClass: "PartDefinition"},
	}
	structure := testStructure{
		metaclasses:           make(map[ElementKey]string),
		ownedRelationships:    make(map[ElementKey][]Element),
		owningRelatedElements: make(map[ElementKey]testOptionalElement),
		relatedElements:       make(map[testRelatedKey][]Element),
	}
	type ownerEntry struct {
		relationship Element
		owner        Element
		test         ownerCase
	}
	var owners []ownerEntry
	for _, test := range cases {
		relationship := Element{Aspect: test.class + "-relationship"}
		owner := Element{Aspect: test.class + "-owner"}
		structure.metaclasses[relationship.Key()] = test.class
		structure.metaclasses[owner.Key()] = test.ownerClass
		structure.ownedRelationships[relationship.Key()] = []Element{}
		structure.owningRelatedElements[relationship.Key()] = testOptionalElement{element: owner, present: true}
		if test.property == "owningAnnotatedElement" {
			structure.relatedElements[testRelatedKey{
				element: relationship.Key(), property: "annotatedElement",
			}] = []Element{owner}
		}
		owners = append(owners, ownerEntry{relationship: relationship, owner: owner, test: test})
	}
	evaluator.options.Structure = structure
	for _, owner := range owners {
		got, ok := evaluator.Property(owner.relationship, owner.test.class, owner.test.property)
		if !ok || got.Kind != ElementValue || got.Element.Key() != owner.owner.Key() {
			t.Errorf("%s::%s = %#v, %t; want owning %s", owner.test.class, owner.test.property, got, ok, owner.test.ownerClass)
		}
	}
}

func TestStructureBackedFeatureOwningTypeRequiresTypeOwner(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "m.sysml", "package P {}")
	feature := Element{Aspect: "test-feature"}
	membership := MembershipElement(Membership{Aspect: "test-feature-membership"})
	typeElement := Element{Aspect: "test-owner-type"}
	packageFeature := Element{Aspect: "package-owned-feature"}
	packageMembership := MembershipElement(Membership{Aspect: "package-feature-membership"})
	packageElement := Element{Aspect: "package-owner"}
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			feature.Key():           "PartUsage",
			membership.Key():        "FeatureMembership",
			typeElement.Key():       "PartDefinition",
			packageFeature.Key():    "PartUsage",
			packageMembership.Key(): "FeatureMembership",
			packageElement.Key():    "Package",
		},
		ownedRelationships: map[ElementKey][]Element{
			feature.Key():           {},
			membership.Key():        {},
			typeElement.Key():       {},
			packageFeature.Key():    {},
			packageMembership.Key(): {},
			packageElement.Key():    {},
		},
		owningRelationships: map[ElementKey]testOptionalElement{
			feature.Key():        {element: membership, present: true},
			packageFeature.Key(): {element: packageMembership, present: true},
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			membership.Key():        {element: typeElement, present: true},
			packageMembership.Key(): {element: packageElement, present: true},
		},
	}
	evaluator.options.Structure = structure

	got, ok := evaluator.Property(feature, "Feature", "owningType")
	if !ok || got.Kind != ElementValue || got.Element.Key() != typeElement.Key() {
		t.Fatalf("Feature::owningType = %#v, %t; want graph-owned type", got, ok)
	}
	got, ok = evaluator.Property(packageFeature, "Feature", "owningType")
	if !ok || got.Kind != NullValue {
		t.Fatalf("package-owned Feature::owningType = %#v, %t; want null", got, ok)
	}
}

func TestStructureBackedOwnedFiltersUseMetaclassClosure(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "m.sysml", "package P {}")
	typeElement := Element{Aspect: "filter-type"}
	feature := Element{Aspect: "filter-feature"}
	featureMembership := MembershipElement(Membership{Aspect: "filter-feature-membership"})
	variantMembership := MembershipElement(Membership{Aspect: "filter-variant-membership"})
	owningMembership := MembershipElement(Membership{Aspect: "filter-owning-membership"})
	conjugation := Element{Aspect: "filter-conjugation"}
	differencing := Element{Aspect: "filter-differencing"}
	annotation := Element{Aspect: "filter-annotation"}
	metadataUsage := Element{Aspect: "filter-metadata-usage"}
	usage := Element{Aspect: "nested-usage"}
	reference := Element{Aspect: "nested-reference"}
	item := Element{Aspect: "nested-item"}
	referenceMembership := MembershipElement(Membership{Aspect: "nested-reference-membership"})
	itemMembership := MembershipElement(Membership{Aspect: "nested-item-membership"})

	structure := testStructure{
		metaclasses: map[ElementKey]string{
			typeElement.Key():         "PartDefinition",
			feature.Key():             "PartUsage",
			featureMembership.Key():   "FeatureMembership",
			variantMembership.Key():   "VariantMembership",
			owningMembership.Key():    "OwningMembership",
			conjugation.Key():         "Conjugation",
			differencing.Key():        "Differencing",
			annotation.Key():          "Annotation",
			metadataUsage.Key():       "MetadataUsage",
			usage.Key():               "Usage",
			reference.Key():           "ReferenceUsage",
			item.Key():                "ItemUsage",
			referenceMembership.Key(): "FeatureMembership",
			itemMembership.Key():      "FeatureMembership",
		},
		ownedRelationships: map[ElementKey][]Element{
			typeElement.Key(): {featureMembership, variantMembership, owningMembership, conjugation, differencing, annotation},
			usage.Key():       {referenceMembership, itemMembership},
		},
		owningRelationships: map[ElementKey]testOptionalElement{
			feature.Key(): {element: featureMembership, present: true},
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			featureMembership.Key(): {element: typeElement, present: true},
			annotation.Key():        {element: typeElement, present: true},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: featureMembership.Key(), property: "memberElement"}:     {feature},
			{element: referenceMembership.Key(), property: "memberElement"}:   {reference},
			{element: itemMembership.Key(), property: "memberElement"}:        {item},
			{element: owningMembership.Key(), property: "ownedMemberElement"}: {metadataUsage},
			{element: annotation.Key(), property: "annotatedElement"}:         {typeElement},
		},
	}
	evaluator.options.Structure = structure

	for _, test := range []struct {
		element Element
		class   string
		name    string
		want    Element
	}{
		{typeElement, "Type", "ownedDifferencing", differencing},
		{typeElement, "Type", "ownedConjugator", conjugation},
		{feature, "Feature", "owningType", typeElement},
		{featureMembership, "FeatureMembership", "owningType", typeElement},
		{featureMembership, "FeatureMembership", "ownedMemberFeature", feature},
		{annotation, "Annotation", "owningAnnotatedElement", typeElement},
	} {
		got, ok := evaluator.Property(test.element, test.class, test.name)
		matches := got.Kind == ElementValue && got.Element.Key() == test.want.Key()
		if test.name == "ownedDifferencing" && got.Kind == SequenceValue {
			matches = len(got.Values) == 1 && got.Values[0].Kind == ElementValue &&
				got.Values[0].Element.Key() == test.want.Key()
		}
		if !ok || !matches {
			t.Errorf("%s::%s = %#v, %t; want %v", test.class, test.name, got, ok, test.want)
		}
	}

	ownedMemberships, ok := evaluator.Property(typeElement, "Type", "ownedFeatureMembership")
	if !ok || len(ownedMemberships.Values) != 1 ||
		ownedMemberships.Values[0].Membership.Key() != featureMembership.Membership.Key() {
		t.Fatalf("Type::ownedFeatureMembership = %#v, %t; want only the FeatureMembership", ownedMemberships, ok)
	}
	ownedAnnotations, ok := evaluator.Property(typeElement, "Element", "ownedAnnotation")
	if !ok || len(ownedAnnotations.Values) != 1 || ownedAnnotations.Values[0].Element.Key() != annotation.Key() {
		t.Fatalf("Element::ownedAnnotation = %#v, %t; want the Annotation relationship only", ownedAnnotations, ok)
	}
	nestedItems, ok := evaluator.Property(usage, "Usage", "nestedItem")
	if !ok || len(nestedItems.Values) != 1 || nestedItems.Values[0].Element.Key() != item.Key() {
		t.Fatalf("Usage::nestedItem = %#v, %t; want only the actual ItemUsage", nestedItems, ok)
	}
}

func TestStructureBackedExpressionConnectorAndMultiplicityProperties(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "m.sysml", "package P { part bounds [1..2]; }")
	pkg := testSymbol(t, root, "P")
	boundsSymbol := testSymbol(t, pkg.Scope, "bounds")
	expression := Element{Aspect: "test-expression"}
	instantiation := Element{Aspect: "test-instantiation"}
	function := Element{Aspect: "test-function"}
	connector := Element{Aspect: "test-connector"}
	multiplicity := Element{
		Node: boundsSymbol.Decl, Container: boundsSymbol, Aspect: "multiplicity",
	}
	bounds, ok := boundsSymbol.Decl.(*ast.Usage)
	if !ok || bounds.Multiplicity == nil || !bounds.Multiplicity.IsRange {
		t.Fatalf("part bounds has no parsed range multiplicity: %#v", boundsSymbol.Decl)
	}
	lowerBound := Element{Node: bounds.Multiplicity.Lower, Container: boundsSymbol, Aspect: "lowerBound"}
	upperBound := Element{Node: bounds.Multiplicity.Upper, Container: boundsSymbol, Aspect: "upperBound"}
	lowerMembership := MembershipElement(Membership{Aspect: "lower-bound-membership", kind: "OwningMembership"})
	upperMembership := MembershipElement(Membership{Aspect: "upper-bound-membership", kind: "OwningMembership"})
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			expression.Key():      "Expression",
			instantiation.Key():   "InstantiationExpression",
			function.Key():        "Function",
			connector.Key():       "Connector",
			multiplicity.Key():    "MultiplicityRange",
			lowerMembership.Key(): "OwningMembership",
			upperMembership.Key(): "OwningMembership",
			lowerBound.Key():      "Expression",
			upperBound.Key():      "Expression",
		},
		ownedRelationships: map[ElementKey][]Element{
			expression.Key():      {},
			instantiation.Key():   {},
			function.Key():        {},
			connector.Key():       {},
			multiplicity.Key():    {lowerMembership, upperMembership},
			lowerMembership.Key(): {},
			upperMembership.Key(): {},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: instantiation.Key(), property: "instantiatedType"}: {expression},
			{element: function.Key(), property: "expression"}:            {expression},
			{element: function.Key(), property: "result"}:                {expression},
			{element: expression.Key(), property: "result"}:              {expression},
			{element: connector.Key(), property: "sourceFeature"}:        {expression},
			{element: connector.Key(), property: "targetFeature"}:        {expression},
			{element: multiplicity.Key(), property: "bound"}:             {expression},
			{element: multiplicity.Key(), property: "lowerBound"}:        {expression},
			{element: multiplicity.Key(), property: "upperBound"}:        {expression},
			{element: lowerMembership.Key(), property: "memberElement"}:  {lowerBound},
			{element: upperMembership.Key(), property: "memberElement"}:  {upperBound},
		},
	}
	evaluator.options.Structure = structure

	if instantiatedType, ok := evaluator.Property(instantiation, "InstantiationExpression", "instantiatedType"); ok {
		t.Errorf("InstantiationExpression::instantiatedType = %#v; want unsupported without a Type member", instantiatedType)
	}
	for _, test := range []struct {
		element Element
		class   string
		name    string
	}{
		{function, "Function", "result"},
		{expression, "Expression", "result"},
	} {
		if _, ok := evaluator.Property(test.element, test.class, test.name); ok {
			t.Errorf("%s::%s was served from a derived graph predicate without semantic inputs", test.class, test.name)
		}
	}
	functionExpressions, ok := evaluator.Property(function, "Function", "expression")
	if !ok || functionExpressions.Kind != SequenceValue || len(functionExpressions.Values) != 0 {
		t.Errorf("Function::expression = %#v, %t; want an empty sequence from known empty ownedFeature structure", functionExpressions, ok)
	}
	for _, property := range []string{"relatedFeature", "sourceFeature", "targetFeature"} {
		assertConnectorProjectionOmitted(t, evaluator, connector, property)
	}

	bound, ok := evaluator.Property(multiplicity, "MultiplicityRange", "bound")
	boundElements := valueElements(t, bound)
	if !ok || len(boundElements) != 2 || boundElements[0].Key() != lowerBound.Key() || boundElements[1].Key() != upperBound.Key() {
		t.Errorf("MultiplicityRange::bound = %#v, %t; want the two AST-declared bounds", bound, ok)
	}
	for property, want := range map[string]Element{"lowerBound": lowerBound, "upperBound": upperBound} {
		got, ok := evaluator.Property(multiplicity, "MultiplicityRange", property)
		if !ok || got.Kind != ElementValue || got.Element.Key() != want.Key() {
			t.Errorf("MultiplicityRange::%s = %#v, %t; want AST-derived %v", property, got, ok, want)
		}
	}
}

func TestStructureBackedCaseAndRequirementFilters(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "case-requirement.sysml", `package P { package R {} }`)
	pkg := testSymbol(t, root, "P")
	requirementScope := testSymbol(t, pkg.Scope, "R")

	caseElement := ElementOf(pkg)
	requirementElement := ElementOf(requirementScope)
	actorMembership := MembershipElement(Membership{Aspect: "case-actor"})
	subjectMembership := MembershipElement(Membership{Aspect: "case-subject"})
	objectiveMembership := MembershipElement(Membership{Aspect: "case-objective"})
	stakeholderMembership := MembershipElement(Membership{Aspect: "requirement-stakeholder"})
	concernMembership := MembershipElement(Membership{Aspect: "requirement-concern"})
	assumptionMembership := MembershipElement(Membership{Aspect: "requirement-assumption"})
	requiredMembership := MembershipElement(Membership{Aspect: "requirement-required"})
	actor := Element{Aspect: "case-actor-usage"}
	subject := Element{Aspect: "case-subject-usage"}
	objective := Element{Aspect: "case-objective-usage"}
	stakeholder := Element{Aspect: "requirement-stakeholder-usage"}
	concern := Element{Aspect: "requirement-concern-usage"}
	assumption := Element{Aspect: "requirement-assumption-usage"}
	required := Element{Aspect: "requirement-required-usage"}

	structure := testStructure{
		metaclasses: map[ElementKey]string{
			caseElement.Key():           "CaseDefinition",
			requirementElement.Key():    "RequirementDefinition",
			actorMembership.Key():       "ActorMembership",
			subjectMembership.Key():     "SubjectMembership",
			objectiveMembership.Key():   "ObjectiveMembership",
			stakeholderMembership.Key(): "StakeholderMembership",
			concernMembership.Key():     "FramedConcernMembership",
			assumptionMembership.Key():  "RequirementConstraintMembership",
			requiredMembership.Key():    "RequirementConstraintMembership",
			actor.Key():                 "PartUsage",
			subject.Key():               "ReferenceUsage",
			objective.Key():             "RequirementUsage",
			stakeholder.Key():           "PartUsage",
			concern.Key():               "ConcernUsage",
			assumption.Key():            "ConstraintUsage",
			required.Key():              "ConstraintUsage",
		},
		ownedRelationships: map[ElementKey][]Element{
			caseElement.Key():        {actorMembership, subjectMembership, objectiveMembership},
			requirementElement.Key(): {stakeholderMembership, concernMembership, assumptionMembership, requiredMembership},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: actorMembership.Key(), property: "memberElement"}:       {actor},
			{element: subjectMembership.Key(), property: "memberElement"}:     {subject},
			{element: objectiveMembership.Key(), property: "memberElement"}:   {objective},
			{element: stakeholderMembership.Key(), property: "memberElement"}: {stakeholder},
			{element: concernMembership.Key(), property: "memberElement"}:     {concern},
			{element: assumptionMembership.Key(), property: "memberElement"}:  {assumption},
			{element: requiredMembership.Key(), property: "memberElement"}:    {required},
		},
		attributes: map[testRelatedKey]Value{
			{element: concernMembership.Key(), property: "kind"}:    stringValue("concern"),
			{element: assumptionMembership.Key(), property: "kind"}: stringValue("assumption"),
			{element: requiredMembership.Key(), property: "kind"}:   stringValue("requirement"),
		},
	}
	evaluator.options.Structure = structure

	for _, test := range []struct {
		element Element
		class   string
		name    string
		want    Element
	}{
		{caseElement, "CaseDefinition", "actorParameter", actor},
		{caseElement, "CaseDefinition", "subjectParameter", subject},
		{caseElement, "CaseDefinition", "objectiveRequirement", objective},
		{requirementElement, "RequirementDefinition", "stakeholderParameter", stakeholder},
		{requirementElement, "RequirementDefinition", "framedConcern", concern},
		{requirementElement, "RequirementDefinition", "assumedConstraint", assumption},
		{requirementElement, "RequirementDefinition", "requiredConstraint", required},
	} {
		got := testProperty(t, evaluator, test.element, test.class, test.name)
		if !containsElement(got, test.want) {
			t.Errorf("%s::%s = %#v; want %v", test.class, test.name, got, test.want)
		}
	}
}

func TestStructureBackedStateAndTransitionFilters(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "state-transition.sysml", `package P {
		state def S {
			state a;
			state b;
			transition first a then b;
		}
	}`)
	pkg := testSymbol(t, root, "P")
	stateSymbol := testSymbol(t, pkg.Scope, "S")
	var transitionNode ast.Node
	for _, succession := range evaluator.model.ActionSuccessions(stateSymbol) {
		if _, ok := succession.Decl.(*ast.TransitionMember); ok {
			transitionNode = succession.Decl
			break
		}
	}
	if transitionNode == nil {
		t.Fatal("state definition has no parsed transition")
	}
	state := ElementOf(stateSymbol)
	subaction := MembershipElement(Membership{Aspect: "state-entry"})
	entry := Element{Aspect: "entry-action"}
	transition := NodeOf(transitionNode)
	trigger := Element{Aspect: "transition-trigger"}
	payload := Element{Aspect: "transition-payload"}
	effect := Element{Aspect: "transition-effect"}
	guard := Element{Aspect: "transition-guard"}
	triggerMembership := MembershipElement(Membership{Aspect: "transition-trigger-membership"})
	triggerPayloadMembership := MembershipElement(Membership{Aspect: "transition-payload-membership"})
	effectMembership := MembershipElement(Membership{Aspect: "transition-effect-membership"})
	successionElement := Element{Node: transitionNode, Container: stateSymbol, Aspect: "transition-succession"}
	successionMembership := MembershipElement(Membership{Aspect: "transition-succession-membership"})
	transitionOwner := MembershipElement(Membership{Aspect: "transition-owning-membership"})
	successionOwner := MembershipElement(Membership{Aspect: "succession-owning-membership"})
	successionFirstEnd := Element{Aspect: "transition-succession-first-end"}
	successionSecondEnd := Element{Aspect: "transition-succession-second-end"}
	successionFirstMembership := MembershipElement(Membership{Aspect: "transition-succession-first-membership", kind: "EndFeatureMembership"})
	successionSecondMembership := MembershipElement(Membership{Aspect: "transition-succession-second-membership", kind: "EndFeatureMembership"})
	flow := Element{Aspect: "flow"}
	payloadMembership := MembershipElement(Membership{Aspect: "flow-payload-membership"})
	payloadFeature := Element{Aspect: "flow-payload-feature"}
	flowEndMembership := MembershipElement(Membership{Aspect: "flow-end-membership"})
	flowEnd := Element{Aspect: "flow-end"}
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			state.Key():                      "StateUsage",
			subaction.Key():                  "StateSubactionMembership",
			entry.Key():                      "PerformActionUsage",
			transition.Key():                 "TransitionUsage",
			trigger.Key():                    "AcceptActionUsage",
			payload.Key():                    "Feature",
			effect.Key():                     "PerformActionUsage",
			guard.Key():                      "Expression",
			triggerMembership.Key():          "TransitionFeatureMembership",
			triggerPayloadMembership.Key():   "ParameterMembership",
			effectMembership.Key():           "TransitionFeatureMembership",
			successionElement.Key():          "SuccessionAsUsage",
			successionMembership.Key():       "OwningMembership",
			transitionOwner.Key():            "OwningMembership",
			successionOwner.Key():            "OwningMembership",
			successionFirstEnd.Key():         "FlowEnd",
			successionSecondEnd.Key():        "FlowEnd",
			successionFirstMembership.Key():  "EndFeatureMembership",
			successionSecondMembership.Key(): "EndFeatureMembership",
			flow.Key():                       "Flow",
			payloadMembership.Key():          "FeatureMembership",
			flowEndMembership.Key():          "FeatureMembership",
			payloadFeature.Key():             "PayloadFeature",
			flowEnd.Key():                    "FlowEnd",
		},
		ownedRelationships: map[ElementKey][]Element{
			state.Key():             {subaction},
			transition.Key():        {triggerMembership, effectMembership, successionMembership},
			successionElement.Key(): {successionFirstMembership, successionSecondMembership},
			trigger.Key():           {triggerPayloadMembership},
			flow.Key():              {payloadMembership, flowEndMembership},
		},
		owningRelationships: map[ElementKey]testOptionalElement{
			transition.Key():        {element: transitionOwner, present: true},
			successionElement.Key(): {element: successionOwner, present: true},
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			transitionOwner.Key(): {element: state, present: true},
			successionOwner.Key(): {element: state, present: true},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: subaction.Key(), property: "memberElement"}:                  {entry},
			{element: triggerMembership.Key(), property: "memberElement"}:          {trigger},
			{element: triggerPayloadMembership.Key(), property: "memberElement"}:   {payload},
			{element: effectMembership.Key(), property: "memberElement"}:           {effect},
			{element: successionMembership.Key(), property: "memberElement"}:       {successionElement},
			{element: successionFirstMembership.Key(), property: "memberElement"}:  {successionFirstEnd},
			{element: successionSecondMembership.Key(), property: "memberElement"}: {successionSecondEnd},
			{element: transition.Key(), property: "guardExpression"}:               {guard},
			{element: payloadMembership.Key(), property: "memberElement"}:          {payloadFeature},
			{element: flowEndMembership.Key(), property: "memberElement"}:          {flowEnd},
			{element: flow.Key(), property: "connectorEnd"}:                        {flowEnd},
		},
		attributes: map[testRelatedKey]Value{
			{element: subaction.Key(), property: "kind"}:         stringValue("entry"),
			{element: triggerMembership.Key(), property: "kind"}: stringValue("trigger"),
			{element: effectMembership.Key(), property: "kind"}:  stringValue("effect"),
		},
	}
	source := testSymbol(t, stateSymbol.Scope, "a")
	target := testSymbol(t, stateSymbol.Scope, "b")
	evaluator.options.Structure = structure

	gotEntry := testProperty(t, evaluator, state, "StateUsage", "entryAction")
	if gotEntry.Kind != ElementValue || gotEntry.Element.Key() != entry.Key() {
		t.Fatalf("StateUsage::entryAction = %#v; want %v", gotEntry, entry)
	}
	for _, test := range []struct {
		name string
		want Element
	}{
		{name: "source", want: ElementOf(source)},
		{name: "target", want: ElementOf(target)},
		{name: "succession", want: successionElement},
		{name: "triggerAction", want: trigger},
		{name: "effectAction", want: effect},
		{name: "guardExpression", want: guard},
	} {
		got := testProperty(t, evaluator, transition, "TransitionUsage", test.name)
		if !containsElement(got, test.want) {
			t.Errorf("TransitionUsage::%s = %#v; want %v", test.name, got, test.want)
		}
	}
	connectorEnds := valueElements(t, testProperty(t, evaluator, successionElement, "Connector", "connectorEnd"))
	if len(connectorEnds) != 2 || connectorEnds[0].Key() != successionFirstEnd.Key() ||
		connectorEnds[1].Key() != successionSecondEnd.Key() {
		t.Fatalf("TransitionMember::connectorEnd = %#v; want its two graph-owned ends", connectorEnds)
	}
	sourceFeature := testProperty(t, evaluator, successionElement, "Connector", "sourceFeature")
	if sourceFeature.Kind != NullValue {
		t.Errorf("TransitionMember::sourceFeature = %#v; want null with no owned ReferenceSubsetting", sourceFeature)
	}
	targetFeatures := valueElements(t, testProperty(t, evaluator, successionElement, "Connector", "targetFeature"))
	if len(targetFeatures) != 0 {
		t.Errorf("TransitionMember::targetFeature = %#v; want empty with no owned ReferenceSubsetting", targetFeatures)
	}
	gotPayloadParameter := testProperty(t, evaluator, trigger, "AcceptActionUsage", "payloadParameter")
	if gotPayloadParameter.Kind != ElementValue || gotPayloadParameter.Element.Key() != payload.Key() {
		t.Fatalf("AcceptActionUsage::payloadParameter = %#v; want %v", gotPayloadParameter, payload)
	}
	gotPayload := testProperty(t, evaluator, flow, "Flow", "payloadFeature")
	if gotPayload.Kind != SequenceValue || !containsElement(gotPayload, payloadFeature) {
		t.Fatalf("Flow::payloadFeature = %#v; want %v", gotPayload, payloadFeature)
	}
	for _, test := range []struct {
		name string
		want Element
	}{
		{name: "flowEnd", want: flowEnd},
	} {
		got := testProperty(t, evaluator, flow, "Flow", test.name)
		if !containsElement(got, test.want) {
			t.Errorf("Flow::%s = %#v; want %v", test.name, got, test.want)
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
