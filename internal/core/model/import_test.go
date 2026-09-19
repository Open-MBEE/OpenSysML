package model

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

func TestVerdictKindWithActions(t *testing.T) {
	ws := NewWorkspace()

	src := `package test {
		private import VerificationCases::*;
		
		part def Vehicle {
			attribute mass;
		}
		
		requirement vehicleMassRequirement {
			subject vehicle : Vehicle;
			in massActual;
			require constraint { 
				massActual == vehicle.mass
			}
		}
		
		verification def VehicleMassTest {
			subject testVehicle : Vehicle;
			
			action evaluateData {
				in massProcessed;
				out verdict : VerdictKind = 
					PassIf(vehicleMassRequirement(vehicle = testVehicle, massActual = massProcessed));
			}
			
			return verdict : VerdictKind = evaluateData.verdict;
		}
	}`

	ws.Open("test.sysml", []byte(src), 1)
	diags := ws.Diagnostics("test.sysml")

	t.Logf("Diagnostics: %d", len(diags))
	for _, d := range diags {
		if d.Severity == diag.SeverityError {
			t.Logf("  [ERROR] %v", d.Message)
		}
	}

	// Check if VerdictKind is resolved
	hasUnresolvedVerdictKind := false
	for _, d := range diags {
		if d.Severity == diag.SeverityError && d.Message == "unresolved reference: VerdictKind" {
			hasUnresolvedVerdictKind = true
		}
	}

	if hasUnresolvedVerdictKind {
		t.Errorf("VerdictKind not resolved despite import VerificationCases::*")
	}
}
