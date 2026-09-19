package model

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

func TestAssignmentExampleTyping(t *testing.T) {
	ws := NewWorkspace()

	// Simplified from Assignment Example.sysml
	src := `package Test {
		private import ISQ::power;
		private import ISQ::mass;
		
		action def ComputeMotion {
			in attribute powerProfile :> power[*];
			in attribute vehicleMass :> mass;
		}
	}`

	ws.Open("test.sysml", []byte(src), 1)
	diags := ws.Diagnostics("test.sysml")

	var errs []string
	for _, d := range diags {
		if d.Severity == diag.SeverityError {
			errs = append(errs, d.Message)
		}
	}

	if len(errs) > 0 {
		t.Fatalf("Expected no errors, got:\n  %v", errs)
	}
}
