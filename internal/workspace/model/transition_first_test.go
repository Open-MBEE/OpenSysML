package model

import (
	"testing"
)

func TestTransitionFirstStart(t *testing.T) {
	ws := NewWorkspace()

	src := `package test {
	attribute def StartSignal;
	
	state def States {
		first start then off;
		
		state off;
		
		transition t1
			first start
			accept StartSignal
			then off;
	}
}`

	ws.Open("test.sysml", []byte(src), 1)
	diags := ws.Diagnostics("test.sysml")

	t.Logf("Found %d diagnostics", len(diags))
	for _, d := range diags {
		t.Logf("  %v", d)
	}

	// Should have 0 diagnostics if "start" is properly registered
	if len(diags) > 0 {
		t.Errorf("Expected 0 diagnostics, got %d", len(diags))
	}
}
