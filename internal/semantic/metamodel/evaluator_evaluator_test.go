package metamodel

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"testing"
)

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

	allMembers := resolveMembershipMembers(t, evaluator, wantMemberships)
	assertElementSymbolOrder(t, valueElements(t, testProperty(t, evaluator, ElementOf(derived), "Namespace", "ownedMember")),
		resolveMembershipMembers(t, evaluator, owningMemberships(local)))
	assertElementSymbolOrder(t, valueElements(t, testProperty(t, evaluator, ElementOf(derived), "Namespace", "member")), allMembers)

	alias := testSymbol(t, derived.Scope, "inheritedAlias")
	if ElementOf(alias).Membership.Feature {
		t.Fatal("alias membership is incorrectly marked as a FeatureMembership")
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
