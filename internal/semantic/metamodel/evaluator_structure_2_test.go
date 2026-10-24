package metamodel

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/translate/rdf/ontology"
	"testing"
)

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

// KerML 1.1 TypeFeaturing::owningFeatureOfType is the featureOfType that owns the TypeFeaturing;
// every name of the owning end answers that Feature, and the owned end its featuringType.
func TestStructureTypeFeaturingEndpointsNameTheOwningFeature(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "type-featuring-endpoints.sysml", `package P {
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
	structure.owningRelatedElements[typeFeaturing.Key()] = testOptionalElement{element: feature, present: true}
	structure.relatedElements[testRelatedKey{element: typeFeaturing.Key(), property: "featuringType"}] = []Element{explicitType}
	evaluator.options.Structure = structure
	for _, property := range []string{"source", "featureOfType", "owningFeatureOfType"} {
		got, ok := evaluator.relationshipEndpoints(typeFeaturing, property)
		if !ok || len(got) != 1 || got[0].Kind != ElementValue || got[0].Element.Key() != feature.Key() {
			t.Fatalf("TypeFeaturing %s endpoints = %#v, %t; want the owning Feature", property, got, ok)
		}
	}
	for _, property := range []string{"target", "featuringType"} {
		got, ok := evaluator.relationshipEndpoints(typeFeaturing, property)
		if !ok || len(got) != 1 || got[0].Kind != ElementValue || got[0].Element.Key() != explicitType.Key() {
			t.Fatalf("TypeFeaturing %s endpoints = %#v, %t; want the featuring Type", property, got, ok)
		}
	}
	if got := testProperty(t, evaluator, typeFeaturing, "TypeFeaturing", "owningFeatureOfType"); got.Kind != ElementValue ||
		got.Element.Key() != feature.Key() {
		t.Fatalf("TypeFeaturing::owningFeatureOfType = %#v; want the owning Feature", got)
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
