package model

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

func TestSIWildcardImportIntegration(t *testing.T) {
	ws := NewWorkspace()

	src := `package Test {
		private import SI::*;
		attribute mass = 100[kg];
		attribute length = 50[mm];
		attribute time = 2[h];
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
		t.Fatalf("Expected no errors with SI wildcard imports, got:\n  %v", errs)
	}
	t.Log("✓ SI short names (kg, mm, h) resolved via wildcard import")
}

func TestSIMemberImportIntegration(t *testing.T) {
	ws := NewWorkspace()

	src := `package Test {
		private import SI::kg;
		attribute mass = 100[kg];
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
		t.Fatalf("Expected no errors with SI member import, got:\n  %v", errs)
	}
	t.Log("✓ SI::kg resolved via member import")
}
