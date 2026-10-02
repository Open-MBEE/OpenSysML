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

// kermlDeclarationsSrc declares members with keywords only KerML.xtext spells,
// written so that the text is legal KerML and parses as SysML too.
const kermlDeclarationsSrc = "package K { class C; feature f; step s; inv { true } connector c from f to s; }\n"

// A snippet loaded from a .kerml file declares with KerML keywords legally, so
// the buffer must not report them there.
func TestLoadedKerMLDeclarationsAreSilent(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "decl.kerml"), kermlDeclarationsSrc)

	s := NewSession()
	if _, err := s.LoadPaths([]string{dir}); err != nil {
		t.Fatalf("LoadPaths: %v", err)
	}
	if diags := s.Diagnostics(); hasCode(diags, passes.CodeKerMLNotation) {
		t.Fatalf("KerML declarations are legal in .kerml, got %v", codesOf(diags))
	}
}

// The same text in a .sysml file, or typed at the prompt that reads as SysML,
// draws one kerml-notation warning per declaration and no error.
func TestKerMLDeclarationsWarnInSysMLAndAtThePrompt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "decl.sysml"), kermlDeclarationsSrc)

	s := NewSession()
	if _, err := s.LoadPaths([]string{dir}); err != nil {
		t.Fatalf("LoadPaths: %v", err)
	}
	if got := countCode(s.Diagnostics(), passes.CodeKerMLNotation); got != 5 {
		t.Fatalf("loaded .sysml: want 5 %s findings, got %d in %v", passes.CodeKerMLNotation, got, codesOf(s.Diagnostics()))
	}
	if s.HasErrors() {
		t.Fatalf("the notation stays parsed, so it must not error: %v", s.DiagnosticLines())
	}

	prompt := NewSession()
	res := prompt.Submit(kermlDeclarationsSrc)
	if got := countCode(res.Diagnostics, passes.CodeKerMLNotation); got != 5 {
		t.Fatalf("prompt: want 5 %s findings, got %d in %v", passes.CodeKerMLNotation, got, codesOf(res.Diagnostics))
	}
	for _, d := range res.Diagnostics {
		if d.Severity == diag.SeverityError {
			t.Fatalf("the notation stays parsed, so it must not error: %v", d)
		}
	}
}

func countCode(diags []diag.Diagnostic, code string) int {
	n := 0
	for _, d := range diags {
		if d.Code == code {
			n++
		}
	}
	return n
}
