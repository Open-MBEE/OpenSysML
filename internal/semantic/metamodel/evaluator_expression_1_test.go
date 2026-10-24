package metamodel

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"testing"
)

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
	if name, ok := evaluator.Property(exhibit, "Element", "name"); !ok ||
		name.Kind != StringValue || name.String != "idling" {
		t.Fatalf("ExhibitStateUsage::name = %#v, %t; want declared name %q", name, ok, "idling")
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
