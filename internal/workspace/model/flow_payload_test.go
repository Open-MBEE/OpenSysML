package model

import (
	"testing"
)

func TestFlowPayloadRedefinition(t *testing.T) {
	ws := NewWorkspace()

	src := `package test {
		private import Flows::*;
		
		part def Fuel;
		
		flow def FuelFlow {
			end a; end b;
			ref :>> payload : Fuel;
		}
	}`

	ws.Open("test.sysml", []byte(src), 1)
	diags := ws.Diagnostics("test.sysml")

	t.Logf("Found %d diagnostics", len(diags))
	for _, d := range diags {
		t.Logf("  %v", d)
	}

	// Should have 0 diagnostics - payload should resolve via inheritance
	if len(diags) > 0 {
		t.Errorf("Expected 0 diagnostics, got %d", len(diags))
	}
}
