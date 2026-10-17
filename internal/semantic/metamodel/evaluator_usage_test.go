package metamodel

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"testing"
)

func TestUsageMayTimeVaryForReferentialActionsAndStates(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "usage-may-time-vary.sysml", `package Occurrences {
		occurrence def Occurrence;
	}
	package Actions {
		action def Action;
	}
	package States {
		state def State;
	}
	package P {
		part def Vehicle :> Occurrences::Occurrence {
			perform action performed : Actions::Action;
			exhibit state exhibited : States::State;
			composite action compositeAction : Actions::Action;
		}
		perform action packageAction : Actions::Action;
	}`)
	pkg := testSymbol(t, root, "P")
	vehicle := testSymbol(t, pkg.Scope, "Vehicle")
	cases := []struct {
		symbol          *symbols.Symbol
		wantComposite   bool
		wantMayTimeVary bool
	}{
		{testSymbol(t, vehicle.Scope, "performed"), false, true},
		{testSymbol(t, vehicle.Scope, "exhibited"), false, true},
		{testSymbol(t, vehicle.Scope, "compositeAction"), true, false},
		{testSymbol(t, pkg.Scope, "packageAction"), false, false},
	}
	for _, tc := range cases {
		if got := evaluator.model.EffectiveIsComposite(tc.symbol); got != tc.wantComposite {
			t.Errorf("%s.isComposite = %t, want %t",
				symbols.FQNOf(tc.symbol), got, tc.wantComposite)
		}
		got := testProperty(t, evaluator, ElementOf(tc.symbol), "Usage", "mayTimeVary")
		if got.Kind != BooleanValue || got.Boolean != tc.wantMayTimeVary {
			t.Errorf("%s.mayTimeVary = %v, want %t",
				symbols.FQNOf(tc.symbol), got, tc.wantMayTimeVary)
		}
	}
}

func TestUsageReferenceAndCompositeConstraints(t *testing.T) {
	evaluator, root := newTestEvaluator(t, "usage-composition.sysml", `package P {
		part def Vehicle {
			port ignitionCmdPort;
			attribute mass;
			part child;
			ref part spare;
			event occurrence arrived;
			port outer {
				port inner;
			}
			state machine {
				state operatingStates {
					state off;
				}
			}
		}
		port def PortShape {
			port nested;
		}
		action def Flow {
			fork f;
		}
	}`)
	vehicle := testSymbol(t, testSymbol(t, root, "P").Scope, "Vehicle")
	portShape := testSymbol(t, testSymbol(t, root, "P").Scope, "PortShape")
	flow := testSymbol(t, testSymbol(t, root, "P").Scope, "Flow")
	machine := testSymbol(t, vehicle.Scope, "machine")
	states := testSymbol(t, machine.Scope, "operatingStates")
	cases := []struct {
		symbol      *symbols.Symbol
		composite   bool
		referential bool
	}{
		{testSymbol(t, vehicle.Scope, "ignitionCmdPort"), false, true},
		{testSymbol(t, vehicle.Scope, "mass"), false, true},
		{testSymbol(t, vehicle.Scope, "child"), true, false},
		{testSymbol(t, vehicle.Scope, "spare"), false, true},
		{testSymbol(t, vehicle.Scope, "arrived"), false, true},
		{testSymbol(t, vehicle.Scope, "outer"), false, true},
		{testSymbol(t, testSymbol(t, vehicle.Scope, "outer").Scope, "inner"), true, false},
		{testSymbol(t, portShape.Scope, "nested"), true, false},
		{testSymbol(t, flow.Scope, "f"), true, false},
		{testSymbol(t, states.Scope, "off"), true, false},
	}
	evaluator.options.Structure = nil
	for _, tc := range cases {
		element := ElementOf(tc.symbol)
		composite := testProperty(t, evaluator, element, "Feature", "isComposite")
		if composite.Kind != BooleanValue || composite.Boolean != tc.composite {
			t.Errorf("%s.isComposite = %v, want %t", symbols.FQNOf(tc.symbol), composite, tc.composite)
		}
		reference := testProperty(t, evaluator, element, "Usage", "isReference")
		if reference.Kind != BooleanValue || reference.Boolean != tc.referential {
			t.Errorf("%s.isReference = %v, want %t", symbols.FQNOf(tc.symbol), reference, tc.referential)
		}
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
