package metamodel

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// Property hands out independent values: editing one must not change what the
// per-element memos serve to the properties projecting from them.
func TestEditingAPropertyResultLeavesTheMemoizedProjectionsIntact(t *testing.T) {
	evaluator, _ := newTestEvaluatorWithIndex(t, libs.NewModelIndex(), "memo.sysml", "package P {}")
	owner := Element{Aspect: "owner"}
	evaluator = New(evaluator.resolver, evaluator.model, Options{Structure: testStructure{
		metaclasses:        map[ElementKey]string{owner.Key(): "PartDefinition"},
		ownedRelationships: map[ElementKey][]Element{owner.Key(): {}},
	}})
	type qualified struct{ class, name string }
	properties := []qualified{{"Type", "feature"}, {"Type", "inheritedMembership"}, {"Namespace", "membership"}}
	dependents := []qualified{{"Type", "directedFeature"}, {"Namespace", "member"}, {"Type", "inheritedFeature"}}
	for _, property := range properties {
		before, ok := evaluator.Property(owner, property.class, property.name)
		if !ok || !before.Success || len(before.Values) == 0 {
			t.Fatalf("%s::%s is not computable", property.class, property.name)
		}
		want := before.Values[0]
		before.Values[0] = Value{Kind: ElementValue, Element: Element{Aspect: "intruder"}, Success: true}
		for _, dependent := range append([]qualified{property}, dependents...) {
			after, ok := evaluator.Property(owner, dependent.class, dependent.name)
			if !ok || !after.Success {
				t.Fatalf("%s::%s is not computable after the edit", dependent.class, dependent.name)
			}
			for _, value := range after.Values {
				if value.Kind == ElementValue && value.Element.Aspect == "intruder" {
					t.Errorf("%s::%s serves the edited value after editing %s::%s",
						dependent.class, dependent.name, property.class, property.name)
				}
			}
			if dependent == property && (len(after.Values) != len(before.Values) || !sameTestValue(after.Values[0], want)) {
				t.Errorf("%s::%s changed after its result was edited", property.class, property.name)
			}
		}
	}
}

func sameTestValue(left, right Value) bool {
	if left.Kind != right.Kind {
		return false
	}
	switch left.Kind {
	case ElementValue:
		return left.Element.Key() == right.Element.Key()
	case MembershipValue:
		return left.Membership.Key() == right.Membership.Key()
	}
	return false
}
