package model

import (
	"testing"
)

func TestActionThenDone(t *testing.T) {
	ws := NewWorkspace()

	src := `package test {
	action def ChargeBattery {
		action endCharging;
		then done;
	}
}`

	ws.Open("test.sysml", []byte(src), 1)
	diags := ws.Diagnostics("test.sysml")

	t.Logf("Found %d diagnostics", len(diags))
	for _, d := range diags {
		t.Logf("  %v", d)
	}

	// Should have 0 diagnostics if "done" is implicit or properly resolved
	if len(diags) > 0 {
		t.Errorf("Expected 0 diagnostics, got %d", len(diags))
	}
}
