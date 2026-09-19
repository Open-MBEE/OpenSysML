package repl

import (
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

// codesOf reports the diagnostic codes of a result, so a finding is asserted by
// the code it is stable under rather than its wording.
func codesOf(diags []diag.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, d.Code)
	}
	return out
}

func hasCode(diags []diag.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

// The buffer carries no file extension, so `namespace` typed at the prompt is
// read as the SysML the prompt takes and draws the KerML-notation warning.
func TestSubmittedNamespaceWarnsAsKerMLNotation(t *testing.T) {
	s := NewSession()
	res := s.Submit("namespace N;\n")
	if !hasCode(res.Diagnostics, passes.CodeKerMLNotation) {
		t.Fatalf("want a %s finding, got %v", passes.CodeKerMLNotation, codesOf(res.Diagnostics))
	}
	for _, d := range res.Diagnostics {
		if d.Severity == diag.SeverityError {
			t.Fatalf("the notation stays parsed, so it must not error: %v", d)
		}
	}
}

// A snippet loaded from a .kerml file is KerML, where `namespace` is legal, so
// the same buffer must not report it there.
func TestLoadedKerMLNamespaceIsSilent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ns.kerml"), "namespace N { class C; }\n")

	s := NewSession()
	if _, err := s.LoadPaths([]string{dir}); err != nil {
		t.Fatalf("LoadPaths: %v", err)
	}
	if diags := s.Diagnostics(); hasCode(diags, passes.CodeKerMLNotation) {
		t.Fatalf("`namespace` is legal in .kerml, got %v", codesOf(diags))
	}
}

// A .sysml file loaded the same way still draws the warning, which is what the
// CLI's -validate reports.
func TestLoadedSysMLNamespaceWarns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ns.sysml"), "namespace N { part def P; }\n")

	s := NewSession()
	if _, err := s.LoadPaths([]string{dir}); err != nil {
		t.Fatalf("LoadPaths: %v", err)
	}
	if diags := s.Diagnostics(); !hasCode(diags, passes.CodeKerMLNotation) {
		t.Fatalf("want a %s finding, got %v", passes.CodeKerMLNotation, codesOf(diags))
	}
	if s.HasErrors() {
		t.Fatalf("the notation stays parsed, so it must not error: %v", s.DiagnosticLines())
	}
}
