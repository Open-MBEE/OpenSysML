package model

import (
	"testing"
)

func TestRequirementParameterBinding(t *testing.T) {
	ws := NewWorkspace()

	src := `package test {
		private import ScalarValues::*;
		
		part def Vehicle {
			attribute mass : Real;
		}
		
		requirement def MassLimit {
			subject vehicle : Vehicle;
			
			require constraint {
				vehicle.mass < 1000.0
			}
		}
		
		requirement checkVehicle : MassLimit {
			subject testVehicle = Vehicle();
		}
	}`

	ws.Open("test.sysml", []byte(src), 1)
	diags := ws.Diagnostics("test.sysml")

	t.Logf("Found %d diagnostics", len(diags))
	for _, d := range diags {
		t.Logf("  %v", d)
	}

	if len(diags) != 1 || diags[0].Code != "invocation-not-behavior" {
		t.Errorf("Expected one invocation-not-behavior diagnostic, got %v", diags)
	}
}
