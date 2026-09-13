package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// analyzeSrc returns every diagnostic src produces, the parser's included.
func analyzeSrc(t *testing.T, src string) []Diagnostic {
	t.Helper()
	root, parseDiags, idx := analyzeInputs(t, "<t>", src)
	return Analyze("<t>", root, parseDiags, idx)
}

// importVisibilityDiags returns the import-visibility findings of src.
func importVisibilityDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	var out []Diagnostic
	for _, d := range analyzeSrc(t, src) {
		if d.Code == "import-visibility" {
			out = append(out, d)
		}
	}
	return out
}

// The indicator is mandatory on an import at any nesting depth; an explicit one
// conforms, and an expose carries an implicit protected visibility instead.
func TestImportVisibilityIndicatorRequired(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		count int
	}{
		{"bare at the root", "import P::*;", 1},
		{"bare in a package", "package Q { import P::*; }", 1},
		{"bare in a definition body", "package Q { part def D { import P::*; } }", 1},
		{"bare in an import body", "package Q { import P::* { import R::*; } }", 2},
		{"public", "public import P::*;", 0},
		{"private", "private import P::*;", 0},
		{"protected", "protected import P::*;", 0},
		{"recursive membership", "import P::**;", 1},
		{"expose", "package P { part p; view v { expose P::**; } }", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if diags := importVisibilityDiags(t, tc.src); len(diags) != tc.count {
				t.Fatalf("got %d import-visibility diagnostics, want %d: %v", len(diags), tc.count, diags)
			}
		})
	}
}

// The parser itself carries the finding, as a warning on the `import` keyword,
// so a direct parser consumer sees it before any pass runs.
func TestImportVisibilityIsAParserWarning(t *testing.T) {
	src := "package Q {\n\timport P::*;\n}"
	p := parser.New(source.New("<t>", []byte(src)))
	p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse errors = %v, want none: the bare form still reads", p.Diagnostics)
	}
	if len(p.Warnings) != 1 || p.Warnings[0].Code != CodeImportVisibility {
		t.Fatalf("parse warnings = %v, want one %s", p.Warnings, CodeImportVisibility)
	}
	if got := src[p.Warnings[0].Span.Offset:p.Warnings[0].Span.End()]; got != "import" {
		t.Errorf("span covers %q, want \"import\"", got)
	}
}

// The finding is an error spanning the `import` keyword: ImportPrefix makes the
// indicator mandatory, so the reference rejects the bare form too (D2).
func TestImportVisibilityErrorsOnTheKeyword(t *testing.T) {
	src := "package Q {\n\timport P::*;\n}"
	diags := importVisibilityDiags(t, src)
	if len(diags) != 1 {
		t.Fatalf("got %d diagnostics, want 1: %v", len(diags), diags)
	}
	d := diags[0]
	if d.Severity != SeverityError {
		t.Errorf("severity = %v, want error", d.Severity)
	}
	if d.Source != "syntax" {
		t.Errorf("source = %q, want syntax", d.Source)
	}
	if got := src[d.Span.Offset:d.Span.End()]; got != "import" {
		t.Errorf("span covers %q, want \"import\"", got)
	}
}

// D2 is a severity change only: a bare import still brings its names in, so
// name resolution run over the document sees what it imported and still reports
// an unresolved target.
func TestImportVisibilityChangesSeverityOnly(t *testing.T) {
	src := "package Lib { part def Widget; }\npackage App { import Lib::*; part w : Widget; import Nowhere::*; }"
	root, parseDiags, idx := analyzeInputs(t, "<t>", src)

	var errored bool
	for _, d := range (GrammarViolationPass{}).Run(NewContext("<t>", idx, parseDiags), "<t>", root) {
		if d.Code == "import-visibility" && d.Severity == SeverityError {
			errored = true
		}
	}
	if !errored {
		t.Error("the bare imports produced no import-visibility error")
	}

	var unresolved bool
	for _, d := range (NameResolutionPass{}).Run(NewContext("<t>", idx, nil), "<t>", root) {
		if d.Code == "unresolved" {
			unresolved = true
		} else if d.Severity == SeverityError {
			t.Errorf("unexpected name-resolution error, so the bare import changed model reading: %+v", d)
		}
	}
	if !unresolved {
		t.Error("the unresolved import target was not reported")
	}
}

// The finding is notation, so a whole run still reaches the tiers above syntax:
// a bare import must not delete the unresolved-reference diagnostic beside it.
func TestImportVisibilityDoesNotGateHigherTiers(t *testing.T) {
	src := "package Q { part def A; }\npackage R { import Q::*; part x : NoSuchType; }"
	var importError, unresolved bool
	for _, d := range analyzeSrc(t, src) {
		switch d.Code {
		case "import-visibility":
			importError = d.Severity == SeverityError
		case "unresolved":
			unresolved = true
		}
	}
	if !importError {
		t.Error("the bare import produced no import-visibility error")
	}
	if !unresolved {
		t.Error("the unresolved reference was suppressed, so D2 is not severity-only")
	}
}
