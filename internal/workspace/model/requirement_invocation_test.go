package model

import (
	"testing"
)

func TestRequirementInvocationParameters(t *testing.T) {
	ws := NewWorkspace()

	src := `package test {
		private import ScalarValues::*;
		private import VerificationCases::*;
		
		part def Vehicle {
			attribute mass : Real;
		}
		
		requirement vehicleMassRequirement {
			subject vehicle : Vehicle;
			in massActual : Real;
			
			require constraint { 
			    massActual == vehicle.mass
			}
		}
		
		// Invoke requirement with named arguments inside PassIf inside action
		action evaluateData {
			in testVehicle : Vehicle;
			in massValue : Real;
			out verdict : VerdictKind = PassIf(vehicleMassRequirement(vehicle = testVehicle, massActual = massValue));
		}
	}`

	ws.Open("test.sysml", []byte(src), 1)
	diags := ws.Diagnostics("test.sysml")

	t.Logf("Found %d diagnostics", len(diags))
	for _, d := range diags {
		t.Logf("  %v", d)
	}

	// Should have 0 diagnostics - vehicle and massActual should resolve as named args
	if len(diags) > 0 {
		t.Errorf("Expected 0 diagnostics, got %d", len(diags))
	}
}
