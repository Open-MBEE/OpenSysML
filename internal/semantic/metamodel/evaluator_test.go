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
)

var testMetaclassParents = map[string]string{
	"Package":                         "Namespace",
	"LibraryPackage":                  "Package",
	"Namespace":                       "Element",
	"Definition":                      "Classifier",
	"Classifier":                      "Type",
	"Type":                            "Namespace",
	"Feature":                         "Type",
	"PayloadFeature":                  "Feature",
	"FlowEnd":                         "Feature",
	"Usage":                           "Feature",
	"PartDefinition":                  "Definition",
	"AttributeDefinition":             "Definition",
	"ItemDefinition":                  "Definition",
	"OccurrenceDefinition":            "Definition",
	"ActionDefinition":                "Behavior",
	"PartUsage":                       "Usage",
	"AttributeUsage":                  "Usage",
	"ReferenceUsage":                  "Usage",
	"ItemUsage":                       "Usage",
	"ActionUsage":                     "Usage",
	"PerformActionUsage":              "ActionUsage",
	"AcceptActionUsage":               "ActionUsage",
	"StateUsage":                      "ActionUsage",
	"SuccessionAsUsage":               "Succession",
	"FeatureMembership":               "Membership",
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
	"EndFeatureMembership":            "FeatureMembership",
	"Membership":                      "Relationship",
	"OwningMembership":                "Membership",
	"VariantMembership":               "Membership",
	"Relationship":                    "Element",
	"Annotation":                      "Relationship",
	"Documentation":                   "Relationship",
	"Import":                          "Relationship",
	"FeatureValue":                    "Relationship",
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
	t.Helper()
	p := parser.New(source.New(name, []byte(text)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	index := symbols.NewIndex()
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
				membership := ElementOf(member).Membership
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
				ownedMemberElement := ElementOf(member)
				if alias, ok := member.Decl.(*ast.Alias); ok {
					if owner := member.Owner(); owner != nil && owner.Scope != nil {
						if target, resolved := evaluator.resolver.ResolveTarget(owner.Scope, alias.For); resolved && target != nil {
							ownedMemberElement = ElementOf(target)
						}
					}
				}
				structure.relatedElements[testRelatedKey{element: handle.Key(), property: "memberElement"}] =
					[]Element{ownedMemberElement}
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
	if transition, ok := element.Node.(*ast.TransitionMember); ok && transition != nil {
		return "TransitionUsage", true
	}
	if usage, ok := element.Node.(*ast.Usage); ok && usage != nil {
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

func TestCanonicalElementAndMembershipKeysIgnoreContext(t *testing.T) {
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
	if membership.Key() != otherMembership.Key() {
		t.Fatalf("membership keys differ with contextual handles: %v != %v", membership.Key(), otherMembership.Key())
	}
	if got, want := membership.Key(), (Membership{Node: member.Decl}).Key(); got != want {
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
	sysmlEvaluator.options.Structure = testStructure{
		metaclass:   testMetaclass,
		specializes: testSpecializes,
		attributes: map[testRelatedKey]Value{
			{element: ElementOf(member).Key(), property: "isVariable"}: booleanValue(true),
		},
	}
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
		{ElementOf(pkg), "Element", "owningNamespace"},
		{ElementOf(pkg), "Namespace", "ownedMembership"},
		{ElementOf(value), "Feature", "type"},
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

	localMembers := resolveMembershipMembers(t, evaluator, local)
	allMembers := resolveMembershipMembers(t, evaluator, wantMemberships)
	assertElementSymbolOrder(t, valueElements(t, testProperty(t, evaluator, ElementOf(derived), "Namespace", "ownedMember")), localMembers)
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
		resolveMembershipMembers(t, evaluator, local))
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

func TestStructureQualifiedNameUsesOwnedNamespacesAndRejectsDuplicates(t *testing.T) {
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
	evaluator.options.Structure = testDocumentStructure(evaluator, root, base, owner)

	got := testProperty(t, evaluator, ElementOf(redefined), "Element", "qualifiedName")
	if got.Kind != StringValue || got.String != "B::original" {
		t.Fatalf("Element::qualifiedName = %#v; want %q", got, "B::original")
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
	features := map[string]string{
		"input":    "ParameterMembership",
		"output":   "ParameterMembership",
		"both":     "ParameterMembership",
		"result":   "ReturnParameterMembership",
		"ordinary": "FeatureMembership",
	}
	structure := testStructure{
		metaclasses:        map[ElementKey]string{ElementOf(compute).Key(): "CalculationDefinition"},
		ownedRelationships: map[ElementKey][]Element{ElementOf(compute).Key(): {}},
		relatedElements:    make(map[testRelatedKey][]Element),
		attributes:         make(map[testRelatedKey]Value),
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
			variant part selected;
			part ordinary;
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
	ownedUsages := valueElements(t, testProperty(t, evaluator, ElementOf(choice), "Definition", "ownedUsage"))
	if len(ownedUsages) != 2 || !containsElementSymbol(ownedUsages, variant) || !containsElementSymbol(ownedUsages, ordinary) {
		t.Fatalf("Definition::ownedUsage = %#v; want both owned Usage members", ownedUsages)
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

func TestStructureFeaturingTypesOnlyImplyFeatureMembershipOwner(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "featuring-type-ownership.sysml", "package P {}")
	owner := Element{Aspect: "feature-owner"}
	feature := Element{Aspect: "feature"}
	membership := MembershipElement(Membership{Aspect: "feature-membership", kind: "FeatureMembership"})
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			owner.Key():      "PartDefinition",
			feature.Key():    "PartUsage",
			membership.Key(): "FeatureMembership",
		},
		ownedRelationships: map[ElementKey][]Element{
			owner.Key():   {membership},
			feature.Key(): {},
		},
		owningRelationships: map[ElementKey]testOptionalElement{
			feature.Key(): {element: membership, present: true},
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			membership.Key(): {element: owner, present: true},
		},
		attributes: map[testRelatedKey]Value{
			{element: feature.Key(), property: "isVariable"}: booleanValue(false),
		},
	}
	evaluator.options.Structure = structure
	got := valueElements(t, testProperty(t, evaluator, feature, "Feature", "featuringType"))
	if len(got) != 1 || got[0].Key() != owner.Key() {
		t.Fatalf("FeatureMembership-owned featuringType = %v; want owner %v", got, owner)
	}

	plainOwner := Element{Aspect: "plain-owner"}
	plainFeature := Element{Aspect: "plain-owned-feature"}
	plainMembership := MembershipElement(Membership{Aspect: "plain-membership", kind: "OwningMembership"})
	structure.metaclasses[plainOwner.Key()] = "PartDefinition"
	structure.metaclasses[plainFeature.Key()] = "PartUsage"
	structure.metaclasses[plainMembership.Key()] = "OwningMembership"
	structure.ownedRelationships[plainOwner.Key()] = []Element{plainMembership}
	structure.ownedRelationships[plainFeature.Key()] = []Element{}
	structure.owningRelationships[plainFeature.Key()] = testOptionalElement{element: plainMembership, present: true}
	structure.owningRelatedElements[plainMembership.Key()] = testOptionalElement{element: plainOwner, present: true}
	structure.attributes[testRelatedKey{element: plainFeature.Key(), property: "isVariable"}] = booleanValue(false)
	evaluator.options.Structure = structure
	got = valueElements(t, testProperty(t, evaluator, plainFeature, "Feature", "featuringType"))
	if len(got) != 0 {
		t.Fatalf("OwningMembership-owned featuringType = %v; want no implied owner", got)
	}
}

func TestStructureFeaturingTypesUsesExplicitTypeFeaturingAndRejectsVariableImpliedType(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "explicit-featuring-type.sysml", "package P {}")
	feature := Element{Aspect: "explicitly-featured"}
	explicitType := Element{Aspect: "explicit-featuring-type"}
	typeFeaturing := Element{Aspect: "type-featuring"}
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			feature.Key():       "PartUsage",
			explicitType.Key():  "PartDefinition",
			typeFeaturing.Key(): "TypeFeaturing",
		},
		ownedRelationships: map[ElementKey][]Element{
			feature.Key(): {typeFeaturing},
		},
		owningRelationships: map[ElementKey]testOptionalElement{
			feature.Key(): {present: false},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: typeFeaturing.Key(), property: "featuringType"}: {explicitType},
		},
	}
	evaluator.options.Structure = structure
	got := valueElements(t, testProperty(t, evaluator, feature, "Feature", "featuringType"))
	if len(got) != 1 || got[0].Key() != explicitType.Key() {
		t.Fatalf("explicit featuringType = %v; want %v", got, explicitType)
	}

	owner := Element{Aspect: "variable-feature-owner"}
	variable := Element{Aspect: "variable-feature"}
	membership := MembershipElement(Membership{Aspect: "variable-feature-membership", kind: "FeatureMembership"})
	structure.metaclasses[owner.Key()] = "PartDefinition"
	structure.metaclasses[variable.Key()] = "PartUsage"
	structure.metaclasses[membership.Key()] = "FeatureMembership"
	structure.ownedRelationships[owner.Key()] = []Element{membership}
	structure.ownedRelationships[variable.Key()] = []Element{}
	structure.owningRelationships = map[ElementKey]testOptionalElement{
		variable.Key(): {element: membership, present: true},
	}
	structure.owningRelatedElements = map[ElementKey]testOptionalElement{
		membership.Key(): {element: owner, present: true},
	}
	structure.attributes = map[testRelatedKey]Value{
		{element: variable.Key(), property: "isVariable"}: booleanValue(true),
	}
	evaluator.options.Structure = structure
	if _, ok := evaluator.Property(variable, "Feature", "featuringType"); ok {
		t.Fatal("variable FeatureMembership feature should have unsupported implied featuringType")
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

func TestStructureMembershipNamesUseDeclaredAttributes(t *testing.T) {
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
			{element: named.Key(), property: "declaredName"}:      stringValue("membershipName"),
			{element: named.Key(), property: "declaredShortName"}: stringValue("mn"),
		},
	}
	evaluator.options.Structure = structure

	for _, test := range []struct {
		element  Element
		property string
		want     string
	}{
		{named, "name", "membershipName"},
		{named, "shortName", "mn"},
	} {
		got := testProperty(t, evaluator, test.element, "Element", test.property)
		if got.Kind != StringValue || got.String != test.want {
			t.Errorf("Membership::%s = %#v; want %q", test.property, got, test.want)
		}
	}
	qualifiedName := testProperty(t, evaluator, named, "Element", "qualifiedName")
	if qualifiedName.Kind != StringValue || qualifiedName.String != "membershipName" {
		t.Fatalf("Membership::qualifiedName = %#v; want membershipName", qualifiedName)
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
		specializes: func(sub, super string) bool {
			if sub == "MetadataUsage" && super == "AnnotatingElement" {
				return true
			}
			return testSpecializes(sub, super)
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			annotation.Key(): {element: annotatingElement, present: true},
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
		{element: annotation, class: "Annotation", name: "owningAnnotatedElement", want: annotatedElement},
		{element: annotatingElement, class: "AnnotatingElement", name: "annotation", want: annotation},
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
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			expression.Key():    "Expression",
			instantiation.Key(): "InstantiationExpression",
			function.Key():      "Function",
			connector.Key():     "Connector",
			multiplicity.Key():  "MultiplicityRange",
		},
		ownedRelationships: map[ElementKey][]Element{
			expression.Key():    {},
			instantiation.Key(): {},
			function.Key():      {},
			connector.Key():     {},
			multiplicity.Key():  {},
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
		},
	}
	evaluator.options.Structure = structure

	for _, test := range []struct {
		element Element
		class   string
		name    string
	}{
		{instantiation, "InstantiationExpression", "instantiatedType"},
		{function, "Function", "expression"},
		{function, "Function", "result"},
		{expression, "Expression", "result"},
		{connector, "Connector", "sourceFeature"},
		{connector, "Connector", "targetFeature"},
	} {
		if _, ok := evaluator.Property(test.element, test.class, test.name); ok {
			t.Errorf("%s::%s was served from a derived graph predicate without semantic inputs", test.class, test.name)
		}
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
	effect := Element{Aspect: "transition-effect"}
	guard := Element{Aspect: "transition-guard"}
	triggerMembership := MembershipElement(Membership{Aspect: "transition-trigger-membership"})
	effectMembership := MembershipElement(Membership{Aspect: "transition-effect-membership"})
	successionElement := Element{Aspect: "transition-succession"}
	successionMembership := MembershipElement(Membership{Aspect: "transition-succession-membership"})
	transitionOwner := MembershipElement(Membership{Aspect: "transition-owning-membership"})
	flow := Element{Aspect: "flow"}
	payloadMembership := MembershipElement(Membership{Aspect: "flow-payload-membership"})
	payloadFeature := Element{Aspect: "flow-payload-feature"}
	flowEndMembership := MembershipElement(Membership{Aspect: "flow-end-membership"})
	flowEnd := Element{Aspect: "flow-end"}
	structure := testStructure{
		metaclasses: map[ElementKey]string{
			state.Key():                "StateUsage",
			subaction.Key():            "StateSubactionMembership",
			entry.Key():                "PerformActionUsage",
			transition.Key():           "TransitionUsage",
			trigger.Key():              "AcceptActionUsage",
			effect.Key():               "PerformActionUsage",
			guard.Key():                "Expression",
			triggerMembership.Key():    "TransitionFeatureMembership",
			effectMembership.Key():     "TransitionFeatureMembership",
			successionElement.Key():    "SuccessionAsUsage",
			successionMembership.Key(): "OwningMembership",
			transitionOwner.Key():      "OwningMembership",
			flow.Key():                 "Flow",
			payloadMembership.Key():    "FeatureMembership",
			flowEndMembership.Key():    "FeatureMembership",
			payloadFeature.Key():       "PayloadFeature",
			flowEnd.Key():              "FlowEnd",
		},
		ownedRelationships: map[ElementKey][]Element{
			state.Key():      {subaction},
			transition.Key(): {triggerMembership, effectMembership, successionMembership},
			flow.Key():       {payloadMembership, flowEndMembership},
		},
		owningRelationships: map[ElementKey]testOptionalElement{
			transition.Key(): {element: transitionOwner, present: true},
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			transitionOwner.Key(): {element: state, present: true},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: subaction.Key(), property: "memberElement"}:            {entry},
			{element: triggerMembership.Key(), property: "memberElement"}:    {trigger},
			{element: effectMembership.Key(), property: "memberElement"}:     {effect},
			{element: successionMembership.Key(), property: "memberElement"}: {successionElement},
			{element: transition.Key(), property: "guardExpression"}:         {guard},
			{element: payloadMembership.Key(), property: "memberElement"}:    {payloadFeature},
			{element: flowEndMembership.Key(), property: "memberElement"}:    {flowEnd},
			{element: flow.Key(), property: "connectorEnd"}:                  {flowEnd},
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
