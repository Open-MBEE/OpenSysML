package model

import "testing"

func TestWorkspaceDefUsageResolvesClean(t *testing.T) {
	ws := NewWorkspace()
	src := "part def Engine; part def Car specializes Engine { part e : Engine; }"
	ws.Open("m.sysml", []byte(src), 1)
	diags := ws.Diagnostics("m.sysml")
	if len(diags) != 0 {
		t.Fatalf("expected no diagnostics, got %v", diags)
	}
}

func TestWorkspaceDefUsageCrossKindTypeError(t *testing.T) {
	ws := NewWorkspace()
	src := "attribute def Mass; part def Car specializes Mass;"
	ws.Open("m.sysml", []byte(src), 1)
	diags := ws.Diagnostics("m.sysml")
	found := false
	for _, d := range diags {
		if d.Source == "type" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a type diagnostic, got %v", diags)
	}
}
