package metamodel

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

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
	return New(resolver, model), index.DocumentRoot(name)
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
		"definition", "usage", "ownedUsage", "nestedUsage", "directedUsage", "variant",
		"isReference", "owningUsage", "owningDefinition", "nestedPart", "nestedAttribute",
	} {
		testProperty(t, evaluator, ElementOf(parent), "Usage", property)
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
	if ownerDefinition.Kind != ElementValue || ownerDefinition.Element.Symbol != leaf {
		t.Fatalf("Usage::owningDefinition = %#v, want Leaf", ownerDefinition)
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
	bodyDirection := testProperty(t, evaluator, NodeOf(body), "Feature", "direction")
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
	chainingFeatures := valueElements(t, testProperty(t, evaluator, ElementOf(chain), "Feature", "chainingFeature"))
	if len(chainingFeatures) != 1 || chainingFeatures[0].Symbol != testSymbol(t, source.Scope, "target") {
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
