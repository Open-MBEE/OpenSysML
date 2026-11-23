package metamodel

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

// A flow's sourceOutputFeature and targetInputFeature are the first ownedFeature of its first and
// second connectorEnd, not the features those redefine (KerML 8.3.4.9.2 deriveFlowSourceOutputFeature, deriveFlowTargetInputFeature).
func TestSymbolStructureDerivesAFlowsEndFeaturesFromItsEnds(t *testing.T) {
	for _, tc := range []struct{ name, file, text string }{
		{"sysml", "flows.sysml", `package P {
	item def Fuel;
	part def Tank { out item fuelOut : Fuel; part inner { out item o : Fuel; } }
	part def Engine { in item fuelIn : Fuel; part inner { in item i : Fuel; } }
	part def Vehicle {
		part tank : Tank;
		part engine : Engine;
		flow f of Fuel from tank.fuelOut to engine.fuelIn;
		succession flow s of Fuel from tank.fuelOut to engine.fuelIn;
		flow chained from tank.inner.o to engine.inner.i;
		message unattached of Fuel from tank to engine;
	}
}`},
		{"kerml", "flows.kerml", `package P {
	class Fuel;
	class Tank { out feature fuelOut : Fuel; feature inner { out feature o : Fuel; } }
	class Engine { in feature fuelIn : Fuel; feature inner { in feature i : Fuel; } }
	class Vehicle {
		feature tank : Tank;
		feature engine : Engine;
		flow f of Fuel from tank.fuelOut to engine.fuelIn;
		succession flow s of Fuel from tank.fuelOut to engine.fuelIn;
		flow chained from tank.inner.o to engine.inner.i;
	}
}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			evaluator, root := newTestEvaluatorWithIndex(t, libs.NewModelIndex(), tc.file, tc.text)
			structure := NewSymbolStructure(evaluator.resolver, evaluator.model)
			e := New(evaluator.resolver, evaluator.model, Options{Structure: structure})
			vehicle := testSymbol(t, testSymbol(t, root, "P").Scope, "Vehicle")
			for _, name := range []string{"f", "s", "chained"} {
				flow := structure.ElementFor(testSymbol(t, vehicle.Scope, name))
				ends := testProperty(t, e, flow, "Connector", "connectorEnd")
				if len(ends.Values) != 2 {
					t.Fatalf("%s has connector ends %v, want two", name, ends.Values)
				}
				for index, property := range []string{"sourceOutputFeature", "targetInputFeature"} {
					owned := testProperty(t, e, ends.Values[index].Element, "Type", "ownedFeature")
					if len(owned.Values) != 1 {
						t.Fatalf("%s end %d owns features %v, want one", name, index, owned.Values)
					}
					got := testProperty(t, e, flow, "Flow", property)
					if got.Kind != ElementValue || !sameStructureElement(got.Element, owned.Values[0].Element, structure) {
						t.Errorf("%s.%s = %v, want the feature its end owns %v", name, property, got, owned.Values[0])
					}
				}
			}
			if tc.name != "sysml" {
				return
			}
			// A message's ends are the ones it inherits, which own no feature.
			message := structure.ElementFor(testSymbol(t, vehicle.Scope, "unattached"))
			for _, property := range []string{"sourceOutputFeature", "targetInputFeature"} {
				if got := testProperty(t, e, message, "Flow", property); got.Kind != NullValue {
					t.Errorf("unattached.%s = %v, want none", property, got)
				}
			}
		})
	}
}
