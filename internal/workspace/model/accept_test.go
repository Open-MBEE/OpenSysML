package model

import (
	"testing"
)

func TestAcceptActionParsing(t *testing.T) {
	ws := NewWorkspace()

	src := `package test {
		item def Scene;
		
		action takePicture {
			action trigger accept scene : Scene;
		}
	}`

	ws.Open("test.sysml", []byte(src), 1)
	diags := ws.Diagnostics("test.sysml")

	t.Logf("Found %d diagnostics", len(diags))
	for _, d := range diags {
		t.Logf("  %v", d)
	}
}
