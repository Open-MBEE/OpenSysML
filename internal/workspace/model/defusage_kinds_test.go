package model

import "testing"

func TestWorkspaceMixedNewKindsResolveClean(t *testing.T) {
	ws := NewWorkspace()
	src := `
package Sys {
	item def Fuel;
	port def FuelPort;
	part def Tank { port supply : FuelPort; out item fuelOut : Fuel; }
	part def Engine { port intake : FuelPort; in item fuelIn : Fuel; }
	part def Vehicle {
		part tank : Tank;
		part engine : Engine;
		connection c connect tank to engine;
		flow f of Fuel from tank.fuelOut to engine.fuelIn;
		action def Start { part p; }
		use case def Drive;
	}
}`
	ws.Open("m.sysml", []byte(src), 1)
	diags := ws.Diagnostics("m.sysml")
	if len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
}
