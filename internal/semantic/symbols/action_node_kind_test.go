package symbols

import (
	"strings"
	"testing"
)

// actionNodeModel declares one action of each node kind (SysML v2 §8.2.2.17
// ActionNode) and a flow beside a succession flow (§8.2.2.14).
const actionNodeModel = `package P {
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
        action sendIt send sig via p;
        action acceptIt accept s : Sig via p;
        action assignIt assign count := 1;
        perform action performIt : Notify;
        action ifIt if count > 0 { action body1; }
        action whileIt while count < 3 { action body2; }
        action forIt for i : Integer in (1, 2, 3) { action body3; }
    }
    part def Vehicle {
        part tank : Tank;
        part engine : Engine;
        flow f of Fuel from tank.fuelOut to engine.fuelIn;
        succession flow sf of Fuel from tank.fuelOut to engine.fuelIn;
    }
}
`

// An action written as a node is that node's usage kind and a `succession flow` a
// succession flow usage, not the plain action or flow usage of the keyword.
func TestBuildClassifiesActionNodesByTheirStatement(t *testing.T) {
	root := build(t, actionNodeModel)
	for path, want := range map[string]SymbolKind{
		"P::A::acceptIt":  SymbolAcceptActionUsage,
		"P::A::assignIt":  SymbolAssignmentActionUsage,
		"P::A::performIt": SymbolPerformActionUsage,
		"P::A::ifIt":      SymbolIfActionUsage,
		"P::A::whileIt":   SymbolWhileLoopActionUsage,
		"P::A::forIt":     SymbolForLoopActionUsage,
		"P::A::body1":     SymbolUnknown,
		"P::Vehicle::f":   SymbolFlowUsage,
		"P::Vehicle::sf":  SymbolSuccessionFlowUsage,
	} {
		sym := lookupPath(t, root, path)
		if want == SymbolUnknown {
			if sym != nil {
				t.Errorf("%s is declared as %s; a node's body declares into the node", path, sym.Kind)
			}
			continue
		}
		if sym == nil {
			t.Errorf("%s is not declared", path)
			continue
		}
		if sym.Kind != want {
			t.Errorf("%s kind = %s, want %s", path, sym.Kind, want)
		}
		if !sym.Kind.IsFeature() {
			t.Errorf("%s (%s) is not a feature", path, sym.Kind)
		}
		if sym.Kind.IsActionUsage() == (want == SymbolFlowUsage || want == SymbolSuccessionFlowUsage) {
			t.Errorf("%s (%s) IsActionUsage = %v", path, sym.Kind, sym.Kind.IsActionUsage())
		}
	}
}

// A terminate node declared with a name is a terminate action usage.
func TestBuildClassifiesTerminateNode(t *testing.T) {
	root := build(t, `package P {
	action def A {
		action stop terminate;
		action quit terminate stop;
	}
}`)
	for _, path := range []string{"P::A::stop", "P::A::quit"} {
		if sym := lookupPath(t, root, path); sym == nil || sym.Kind != SymbolTerminateActionUsage {
			t.Errorf("%s is not a terminate action usage", path)
		}
	}
}

// A subject is a reference usage (SysML v2 §8.2.2.19) in a requirement body
// (`parseSubjectMember`) as in a case body (the generic usage path).
func TestBuildClassifiesSubjectsAsReferenceUsages(t *testing.T) {
	root := build(t, `package P {
	item def Thing;
	requirement def Req { subject s : Thing; }
	use case def UC { subject s : Thing; }
	requirement req : Req { subject t : Thing; }
	use case uc : UC { subject t : Thing; }
}`)
	for _, path := range []string{"P::Req::s", "P::UC::s", "P::req::t", "P::uc::t"} {
		sym := lookupPath(t, root, path)
		if sym == nil {
			t.Errorf("%s is not declared", path)
			continue
		}
		if sym.Kind != SymbolReferenceUsage {
			t.Errorf("%s kind = %s, want referenceUsage", path, sym.Kind)
		}
	}
}

// lookupPath follows a `::` path from root, nil where a step is missing.
func lookupPath(t *testing.T, root *Scope, path string) *Symbol {
	t.Helper()
	scope := root
	var sym *Symbol
	for _, step := range splitPath(path) {
		if scope == nil {
			return nil
		}
		var ok bool
		sym, ok = scope.LookupLocal(step)
		if !ok {
			return nil
		}
		scope = sym.Scope
	}
	return sym
}

func splitPath(path string) []string {
	var steps []string
	for _, s := range strings.Split(path, "::") {
		steps = append(steps, s)
	}
	return steps
}
