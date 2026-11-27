package metamodel

import (
	"slices"
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

// The memoized supertype lists are handed out as copies: a reader that appends to or
// overwrites its list must not change what the next reader sees.
func TestEditingASupertypeListLeavesTheMemoizedListIntact(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "supertypes.sysml", `package P {
		part def Base { attribute baseValue; }
		part def Leaf specializes Base { attribute leafValue; }
	}`)
	pkg := testSymbol(t, root, "P")
	base := ElementOf(testSymbol(t, pkg.Scope, "Base"))
	leaf := ElementOf(testSymbol(t, pkg.Scope, "Leaf"))
	lists := map[string]func(Element) ([]Element, bool){
		"owned supertypes":          evaluator.structureOwnedSupertypes,
		"specialized general types": evaluator.structureSpecializedGeneralTypes,
		"general types":             evaluator.structureGeneralTypes,
	}
	for name, list := range lists {
		first, ok := list(leaf)
		if !ok || len(first) == 0 || first[0].Key() != base.Key() {
			t.Fatalf("%s of Leaf = %v, %v; want Base first", name, first, ok)
		}
		want := len(first)
		first[0] = Element{Aspect: "intruder"}
		_ = append(first, Element{Aspect: "intruder"})
		again, ok := list(leaf)
		if !ok || len(again) != want || again[0].Key() != base.Key() {
			t.Errorf("%s of Leaf after an edit = %v, %v; want the original list", name, again, ok)
		}
	}
	owned, handled, ok := evaluator.structureOwnedFeatures(leaf)
	again, _, _ := evaluator.structureOwnedFeatures(leaf)
	if !handled || !ok || len(owned.Values) != 1 || len(again.Values) != 1 ||
		!sameTestValue(owned.Values[0], again.Values[0]) {
		t.Errorf("owned features of Leaf = %v then %v; want leafValue both times", owned.Values, again.Values)
	}
}

// Type::input, Type::output and Type::directedFeature read one memoized list of
// directions: each keeps its own selection, in whichever order they are asked.
func TestDirectedFeaturePropertiesShareTheMemoizedDirections(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "directions.sysml", `package Base {
		part def Anything;
	}
	package Parts {
		part def Part;
	}
	package P {
		part def Base { in attribute source; out attribute result; }
		part def Leaf specializes Base { inout attribute both; attribute plain; }
	}`)
	leaf := ElementOf(testSymbol(t, testSymbol(t, root, "P").Scope, "Leaf"))
	want := map[string][]string{
		"output":          {"both", "result"},
		"input":           {"both", "source"},
		"directedFeature": {"both", "source", "result"},
	}
	for range 2 {
		for _, property := range []string{"output", "input", "directedFeature"} {
			value := testProperty(t, evaluator, leaf, "Type", property)
			var names []string
			for _, feature := range value.Values {
				names = append(names, feature.Element.Symbol.Name)
			}
			if !slices.Equal(names, want[property]) {
				t.Errorf("Type::%s of Leaf = %v, want %v", property, names, want[property])
			}
		}
	}
}
