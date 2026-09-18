package model

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

func TestMessageTargetEventResolution(t *testing.T) {
	ws := NewWorkspace()

	src := `package Test {
		private import Flows::*;
		item def SetSpeed;
		occurrence def TestInteraction {
			part driver;
			part controller;
			message setSpeedMessage of SetSpeed from driver to controller;
			ref part test {
				event occurrence sent = setSpeedMessage.sourceEvent;
			}
		}
	}`

	ws.Open("test.sysml", []byte(src), 1)
	diags := ws.Diagnostics("test.sysml")

	var errs []string
	for _, d := range diags {
		if d.Severity == diag.SeverityError {
			errs = append(errs, d.Message)
			t.Logf("ERROR: %s", d.Message)
		}
	}

	if len(errs) > 0 {
		t.Fatalf("Expected no errors, got %d errors", len(errs))
	}
}
