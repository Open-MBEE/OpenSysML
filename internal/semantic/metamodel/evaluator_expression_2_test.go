package metamodel

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"testing"
)

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

func TestInstantiationExpressionResolvesBracketAndSequenceOperators(t *testing.T) {
	evaluator, root := newTestEvaluatorWithIndex(t, libs.NewModelIndex(), "sequence-operator-types.sysml", "package P {}")
	pkg := testSymbol(t, root, "P")
	bracket := Element{
		Node:      &ast.IndexExpr{Bracket: true, Operand: &ast.LiteralInteger{}, Index: &ast.LiteralInteger{}},
		Container: pkg,
		Aspect:    "bracket-operator",
	}
	sequence := Element{
		Node:      &ast.SequenceExpr{Elements: []ast.Node{&ast.LiteralInteger{}, &ast.LiteralInteger{}}},
		Container: pkg,
		Aspect:    "sequence-operator",
	}

	for _, test := range []struct {
		name     string
		element  Element
		function *symbols.Symbol
	}{
		{name: "[", element: bracket, function: evaluator.model.OperatorFunction(ast.OpIndex)},
		{name: ",", element: sequence, function: evaluator.model.OperatorFunctionNamed(",")},
	} {
		if test.function == nil {
			t.Fatalf("BaseFunctions operator %q was not resolved", test.name)
		}
		instantiatedType, ok := evaluator.instantiationExpressionProperty(test.element, "instantiatedType")
		if !ok || instantiatedType.Kind != ElementValue || instantiatedType.Element.Symbol != test.function {
			t.Errorf("operator %q instantiatedType = %#v, %t; want %s",
				test.name, instantiatedType, ok, symbols.FQNOf(test.function))
		}
		evaluator.options.Structure = testStructure{
			metaclasses: map[ElementKey]string{
				bracket.Key():  "OperatorExpression",
				sequence.Key(): "SequenceExpression",
			},
		}
		function := testProperty(t, evaluator, test.element, "Expression", "function")
		if function.Kind != ElementValue || function.Element.Symbol != test.function {
			t.Errorf("operator %q function = %#v; want %s", test.name, function, symbols.FQNOf(test.function))
		}
	}
}
