package model

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

func TestSendActionSentMessageAccess(t *testing.T) {
	ws := NewWorkspace()

	src := `package Test {
		private import Actions::*;
		item def SetSpeed;
		part driver_a {
			action driverBehavior {
				action sendSetSpeed send new SetSpeed() to vehicle_a;
			}
		}
		part vehicle_a;
		occurrence cruiseControlInteraction_a {
			message testMessage = driver_a.driverBehavior.sendSetSpeed.sentMessage;
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
