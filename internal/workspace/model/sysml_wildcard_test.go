package model

import "testing"

func TestSysMLWildcardExpansion(t *testing.T) {
	ws := NewWorkspace()

	// Check if Systems::Usage exists
	sysUsage := ws.index.LookupQualified("SysML::Systems::Usage")
	t.Logf("SysML::Systems::Usage: %d symbols", len(sysUsage))

	// Check if re-exported to SysML::Usage
	usage := ws.index.LookupQualified("SysML::Usage")
	t.Logf("SysML::Usage (re-exported): %d symbols", len(usage))

	if len(sysUsage) == 0 {
		t.Error("SysML::Systems::Usage not found in stdlib")
	}
	if len(usage) == 0 {
		t.Error("SysML::Usage not re-exported via 'public import Systems::*'")
	}
}
