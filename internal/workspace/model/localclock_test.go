package model

import (
	"testing"
)

func TestLocalClockRedefinition(t *testing.T) {
	ws := NewWorkspace()

	src := `package test {
	part def Server {
		part :>> localClock;
	}
}`

	ws.Open("test.sysml", []byte(src), 1)
	diags := ws.Diagnostics("test.sysml")

	t.Logf("Found %d diagnostics", len(diags))
	for _, d := range diags {
		t.Logf("  %v", d)
	}

	// Should resolve localClock from Occurrence via Part → Item → Occurrence chain
	if len(diags) > 0 {
		t.Errorf("Expected 0 diagnostics, got %d", len(diags))
	}
}
