package model

import (
	"strings"
	"testing"
)

func TestISQImportInWorkspace(t *testing.T) {
	ws := NewWorkspace()

	// Verify ISQ::MassValue exists
	syms := ws.LookupQualified("ISQ::MassValue")
	t.Logf("ISQ::MassValue: %d symbols", len(syms))
	if len(syms) == 0 {
		t.Fatal("ISQ::MassValue not in index")
	}

	// Check ISQ package itself
	isqSyms := ws.LookupQualified("ISQ")
	t.Logf("ISQ: %d symbols", len(isqSyms))
	if len(isqSyms) > 0 {
		isq := isqSyms[0]
		t.Logf("ISQ: name=%s kind=%v hasScope=%v", isq.Name, isq.Kind, isq.Scope != nil)
	}

	// Open a document with import
	src := `import ISQ::*;
attribute mass : MassValue;`
	ws.Open("test.sysml", []byte(src), 1)

	// Check diagnostics
	diags := ws.Diagnostics("test.sysml")
	t.Logf("Diagnostics: %d", len(diags))
	for _, d := range diags {
		t.Logf("  - %s: %s (code=%s source=%s)", d.Severity, d.Message, d.Code, d.Source)
		if strings.Contains(d.Message, "unresolved") && strings.Contains(d.Message, "MassValue") {
			t.Error("MassValue unresolved after import ISQ::*")
		}
	}
}
