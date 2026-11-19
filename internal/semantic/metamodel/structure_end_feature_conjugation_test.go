package metamodel

import "testing"

// A conjugated type's only supertype is the original type of its Conjugation (KerML 8.3.3.1.10
// supertypes), so its endFeature and feature are the original type's ends.
func TestStructureEndFeatureOfConjugatedTypeIsTheOriginalTypesEnds(t *testing.T) {
	evaluator, _ := newTestEvaluator(t, "m.sysml", "package P {}")
	original := Element{Aspect: "original"}
	conjugated := Element{Aspect: "conjugated"}
	conjugation := Element{Aspect: "conjugation"}
	end := Element{Aspect: "end"}
	endMembership := MembershipElement(Membership{Aspect: "end-membership"})

	evaluator.options.Structure = testStructure{
		metaclasses: map[ElementKey]string{
			original.Key():      "Classifier",
			conjugated.Key():    "Classifier",
			conjugation.Key():   "Conjugation",
			end.Key():           "Feature",
			endMembership.Key(): "EndFeatureMembership",
		},
		ownedRelationships: map[ElementKey][]Element{
			original.Key():   {endMembership},
			conjugated.Key(): {conjugation},
			end.Key():        {},
		},
		owningRelationships: map[ElementKey]testOptionalElement{
			end.Key(): {element: endMembership, present: true},
		},
		owningRelatedElements: map[ElementKey]testOptionalElement{
			endMembership.Key(): {element: original, present: true},
			conjugation.Key():   {element: conjugated, present: true},
		},
		relatedElements: map[testRelatedKey][]Element{
			{element: endMembership.Key(), property: "memberElement"}:      {end},
			{element: endMembership.Key(), property: "ownedMemberElement"}: {end},
			{element: conjugation.Key(), property: "originalType"}:         {original},
			{element: conjugation.Key(), property: "conjugatedType"}:       {conjugated},
		},
		attributes: map[testRelatedKey]Value{
			{element: end.Key(), property: "isEnd"}:                {Kind: BooleanValue, Boolean: true},
			{element: endMembership.Key(), property: "visibility"}: {Kind: StringValue, String: "public"},
		},
	}
	for _, property := range []string{"feature", "endFeature"} {
		got, ok := evaluator.Property(conjugated, "Type", property)
		if !ok || got.Kind != SequenceValue || len(got.Values) != 1 || got.Values[0].Element.Key() != end.Key() {
			t.Errorf("Type::%s of the conjugated type = %#v, %t; want the original type's end", property, got, ok)
		}
	}
}
