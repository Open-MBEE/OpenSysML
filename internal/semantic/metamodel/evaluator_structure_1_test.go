package metamodel

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
	"strings"
	"testing"
)

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

func TestStructureQualifiedNameDependsOnlyOnEarlierMembers(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "qualified-name-earlier-members.sysml", `package P {
		part def A;
	}`)
	pkg := testSymbol(t, root, "P")
	namespace := ElementOf(pkg)
	named := Element{Aspect: "named-member"}
	unavailable := Element{Aspect: "unavailable-member"}

	makeStructure := func(members ...Element) testStructure {
		structure := testDocumentStructure(evaluator, root, pkg)
		structure.unknownOwnedRelations = make(map[ElementKey]bool)
		for _, member := range members {
			addTestOwnedMembership(&structure, namespace, member, "Feature")
		}
		structure.attributes[testRelatedKey{element: named.Key(), property: "declaredName"}] =
			stringValue("named")
		structure.unknownOwnedRelations[unavailable.Key()] = true
		return structure
	}

	evaluator.options.Structure = makeStructure(named, unavailable)
	if got := testProperty(t, evaluator, named, "Element", "qualifiedName"); got.Kind != StringValue ||
		got.String != "P::named" {
		t.Fatalf("qualifiedName with a later unavailable sibling = %#v; want %q", got, "P::named")
	}

	evaluator = New(evaluator.resolver, evaluator.model)
	evaluator.options.Structure = makeStructure(unavailable, named)
	value, ok := evaluator.Property(named, "Element", "qualifiedName")
	if ok {
		t.Fatalf("qualifiedName with an earlier unavailable sibling = %#v, %t; want unsupported", value, ok)
	}
	reason, explained := evaluator.Explain(named, "Element", "qualifiedName")
	if !explained || reason.Category != ReasonDependsOnUnsupported ||
		reason.Dependency != "Element::name" {
		t.Fatalf("qualifiedName reason = %#v, %t; want unsupported Element::name dependency", reason, explained)
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
	shortName, ok := evaluator.Property(element, "Element", "shortName")
	if !ok || shortName.Kind != NullValue {
		t.Fatalf("Element::shortName = %#v, %t; want a computable null short name", shortName, ok)
	}
	if got, ok := evaluator.Explain(element, "Element", "shortName"); ok {
		t.Fatalf("Explain for declared shortName = %#v, %t; want no unsupported reason", got, ok)
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
	if len(semanticEnds) != 2 || semanticEnds[0] == nil || semanticEnds[0].Name != "source" ||
		semanticEnds[1] == nil || semanticEnds[1].Name != "target" {
		t.Fatalf("semantic flow ends = %#v, want the inherited source and target ends", semanticEnds)
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
		want := []string{"source", "target"}[index]
		name, handled, ok := evaluator.structureName(end, "name")
		if !handled || !ok || name.Kind != StringValue || name.String != want {
			t.Errorf("FlowEnd[%d] name = %#v, handled=%t, ok=%t; want library end name %q",
				index, name, handled, ok, want)
		}
		shortName, handled, ok := evaluator.structureName(end, "shortName")
		if !handled || !ok || shortName.Kind != NullValue {
			t.Errorf("FlowEnd[%d] shortName = %#v, handled=%t, ok=%t; want null",
				index, shortName, handled, ok)
		}
	}
	for index, membership := range []Element{firstMembership, secondMembership} {
		shortName, handled, ok := evaluator.structureMemberName(membership, "", "shortName")
		if !handled || !ok || shortName.Kind != NullValue {
			t.Errorf("%s::ownedMemberShortName = %#v, handled=%t, ok=%t; want null for end %d",
				membership.Membership.Metaclass(), shortName, handled, ok, index)
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
