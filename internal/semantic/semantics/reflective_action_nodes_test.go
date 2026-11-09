package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// An action written as a node reflects that node's metaclass (SysML v2 §8.2.2.17), a
// `succession flow` a SuccessionFlowUsage (§8.2.2.14), in a document and an index record alike.
func TestReflectiveMetaclassOfActionNodes(t *testing.T) {
	const src = `package P {
		private import ScalarValues::*;
		attribute def Sig;
		item def Fuel;
		port def SigPort;
		part def Tank { out item fuelOut : Fuel; }
		part def Engine { in item fuelIn : Fuel; }
		action def Notify;
		action def A {
			in sig : Sig;
			in p : SigPort;
			attribute count : Integer;
			action acceptIt accept s : Sig via p;
			action assignIt assign count := 1;
			perform action performIt : Notify;
			action ifIt if count > 0 { action body1; }
			action whileIt while count < 3 { action body2; }
			action forIt for i : Integer in (1, 2, 3) { action body3; }
			action stop terminate;
		}
		part def Vehicle {
			part tank : Tank;
			part engine : Engine;
			flow f of Fuel from tank.fuelOut to engine.fuelIn;
			succession flow sf of Fuel from tank.fuelOut to engine.fuelIn;
		}
	}`
	for _, fixture := range reflectiveFixtures(t, "action-nodes.sysml", source.KindSysML, src) {
		for path, want := range map[string]string{
			"P::A::acceptIt":  "AcceptActionUsage",
			"P::A::assignIt":  "AssignmentActionUsage",
			"P::A::performIt": "PerformActionUsage",
			"P::A::ifIt":      "IfActionUsage",
			"P::A::whileIt":   "WhileLoopActionUsage",
			"P::A::forIt":     "ForLoopActionUsage",
			"P::A::stop":      "TerminateActionUsage",
			"P::Vehicle::f":   "FlowUsage",
			"P::Vehicle::sf":  "SuccessionFlowUsage",
		} {
			sym := nestedSym(t, fixture.root, path)
			mc := fixture.model.MetaclassOf(sym)
			if mc == nil || mc.Name != want {
				t.Errorf("%s metaclass = %v, want %s", path, mc, want)
				continue
			}
			if want != "FlowUsage" && want != "SuccessionFlowUsage" && !fixture.model.reflectiveMetaclassConforms(sym, "ActionUsage") {
				t.Errorf("%s (%s) does not conform to ActionUsage", path, want)
			}
			if want == "SuccessionFlowUsage" && !fixture.model.reflectiveMetaclassConforms(sym, "FlowUsage") {
				t.Errorf("%s does not conform to FlowUsage", path)
			}
		}
	}
}
