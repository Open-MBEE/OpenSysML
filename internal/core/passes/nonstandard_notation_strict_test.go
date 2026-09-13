package passes

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/conformance"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// extensionInventory is every construct the audit classifies as OpenSysML
// notation: the pass must report each one in either mode.
var extensionInventory = []string{
	"state def S { choice c; }",
	"state def S { junction j; }",
	"state def S { history h; }",
	"state def S { shallow history h; }",
	"state def S { deep history h; }",
	"state def S { state a { defer e; } }",
	"part def P { part a; first a; }",
	"package P { view def V { expose P::*; } }",
	"action def A { action x; action y; transition first x then y; }",
	"part def P { require constraint { true } }",
}

// notationDiags runs the pass over a document in the named mode.
func notationDiags(t *testing.T, name, src string, mode conformance.Mode) []Diagnostic {
	t.Helper()
	root, pd, idx := analyzeInputs(t, name, src)
	if hasParseError(pd) {
		t.Fatalf("%s: the notation must stay parsed in either mode, got %+v", src, pd)
	}
	ctx := NewContextWithOptions(name, source.KindOf(name), idx, pd, Options{Conformance: mode})
	return NonstandardNotationPass{}.Run(ctx, name, root)
}

// Strict mode changes what a finding weighs, never whether there is one: the
// same constructs are reported, as errors.
func TestExtensionInventoryIsAnErrorUnderStrictMode(t *testing.T) {
	for _, src := range extensionInventory {
		strict := notationDiags(t, "a.sysml", src, conformance.ModeStrict)
		def := notationDiags(t, "a.sysml", src, conformance.ModeDefault)
		if len(strict) == 0 || len(strict) != len(def) {
			t.Fatalf("%s: strict gave %d finding(s), default %d; want the same, non-zero count", src, len(strict), len(def))
		}
		for i, d := range strict {
			if d.Severity != SeverityError {
				t.Errorf("%s: strict severity = %v, want error", src, d.Severity)
			}
			if def[i].Severity != SeverityWarning {
				t.Errorf("%s: default severity = %v, want warning", src, def[i].Severity)
			}
			if d.Code != CodeNonstandardNotation || d.Code != def[i].Code {
				t.Errorf("%s: strict code = %q, default %q, want %q", src, d.Code, def[i].Code, CodeNonstandardNotation)
			}
			if d.Message != def[i].Message || d.Span != def[i].Span {
				t.Errorf("%s: strict mode must change only the severity, got %+v vs %+v", src, d, def[i])
			}
		}
	}
}

// KerML notation in a SysML file is non-conforming SysML too, so strict mode
// errors on it and a KerML file stays silent in either mode.
func TestKerMLNotationFollowsTheMode(t *testing.T) {
	const src = "package P { namespace N; }"
	for _, d := range notationDiags(t, "a.sysml", src, conformance.ModeStrict) {
		if d.Severity != SeverityError || d.Code != CodeKerMLNotation {
			t.Errorf("strict: got %+v, want a kerml-notation error", d)
		}
	}
	if got := notationDiags(t, "a.kerml", src, conformance.ModeStrict); len(got) != 0 {
		t.Errorf("a KerML file uses KerML notation: got %+v, want silence", got)
	}
}

// Standard notation must stay silent under strict mode: a false rejection of
// conforming notation would be the worse defect.
func TestStandardNotationIsSilentUnderStrictMode(t *testing.T) {
	for _, src := range []string{
		"package P { part def Widget; }",
		"state def S { entry; then a; state a; }",
		"state s parallel { state a; state b; }",
		"state def S { state a; state b; transition first a then b; }",
		"action def A { first start; then done; }",
		"action def A { fork f; join j; merge m; decide d; }",
		"package P { attribute done : Boolean; attribute region : Boolean; }",
	} {
		if got := notationDiags(t, "a.sysml", src, conformance.ModeStrict); len(got) != 0 {
			t.Errorf("%s: got %+v, want silence under strict mode", src, got)
		}
	}
}

// The strict severity is chosen by the mode alone, so an unnamed mode is the
// default one.
func TestNotationSeverity(t *testing.T) {
	if got := notationSeverity(conformance.ModeStrict); got != SeverityError {
		t.Errorf("strict severity = %v, want error", got)
	}
	if got := notationSeverity(conformance.ModeDefault); got != SeverityWarning {
		t.Errorf("default severity = %v, want warning", got)
	}
}

func TestExtensionFindingsFollowTheMode(t *testing.T) {
	for _, tc := range []struct {
		name, file, src, code string
	}{
		{"one_ended_first", "a.sysml", "part def P { part a; first a; }", CodeNonstandardNotation},
		{"requirement_constraint", "a.sysml", "analysis def An { attribute size; require constraint { size >= 1 } }", CodeNonstandardNotation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			def := notationDiags(t, tc.file, tc.src, conformance.ModeDefault)
			strict := notationDiags(t, tc.file, tc.src, conformance.ModeStrict)
			if len(def) != 1 || len(strict) != 1 {
				t.Fatalf("got %d default and %d strict findings, want 1 each: %+v / %+v", len(def), len(strict), def, strict)
			}
			if def[0].Code != tc.code || strict[0].Code != tc.code {
				t.Errorf("codes = %q / %q, want %q", def[0].Code, strict[0].Code, tc.code)
			}
			if def[0].Severity != SeverityWarning || strict[0].Severity != SeverityError {
				t.Errorf("severities = %v / %v, want warning then error", def[0].Severity, strict[0].Severity)
			}
			if def[0].Message != strict[0].Message || def[0].Span != strict[0].Span {
				t.Errorf("strict mode must move only the severity: %+v vs %+v", def[0], strict[0])
			}
		})
	}
}

func TestRecoveredGrammarViolationsAreErrorsInEitherMode(t *testing.T) {
	for _, tc := range []struct {
		name, file, src, code string
	}{
		{"definition_keyword_name", "a.sysml", "package P { part def part; }", CodeReservedKeywordName},
		{"alias_keyword_name", "a.sysml", "package P { part def B; alias part for P::B; }", CodeReservedKeywordName},
		{"sysml_declaration_in_kerml", "a.kerml", "package P { part def Wheel; }", CodeSysMLNotation},
	} {
		t.Run(tc.name, func(t *testing.T) {
			def := notationDiags(t, tc.file, tc.src, conformance.ModeDefault)
			strict := notationDiags(t, tc.file, tc.src, conformance.ModeStrict)
			if len(def) != 1 || len(strict) != 1 {
				t.Fatalf("got %d default and %d strict findings, want 1 each: %+v / %+v", len(def), len(strict), def, strict)
			}
			if def[0].Code != tc.code || strict[0].Code != tc.code {
				t.Errorf("codes = %q / %q, want %q", def[0].Code, strict[0].Code, tc.code)
			}
			if def[0].Severity != SeverityError || strict[0].Severity != SeverityError {
				t.Errorf("severities = %v / %v, want errors", def[0].Severity, strict[0].Severity)
			}
			if def[0].Message != strict[0].Message || def[0].Span != strict[0].Span {
				t.Errorf("modes changed the finding: %+v vs %+v", def[0], strict[0])
			}
		})
	}
}

// A keyword-as-name is reported once per span after the parser warning is
// replaced by the notation error for the same recovered declaration.
func TestKeywordAsNameIsReportedOnceInEitherMode(t *testing.T) {
	for _, tc := range []struct{ name, file, src string }{
		{"definition", "a.sysml", "package P { part def part; }"},
		{"usage", "a.sysml", "package P { part def B; part part : B; }"},
		{"alias", "a.sysml", "package P { part def B; alias part for P::B; }"},
		{"kerml_alias", "a.kerml", "package P { class B; alias class for P::B; }"},
		{"multiplicity", "a.sysml", "package P { multiplicity comment [1..1]; }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, mode := range []conformance.Mode{conformance.ModeDefault, conformance.ModeStrict} {
				root, pd, idx := analyzeInputs(t, tc.file, tc.src)
				got := []Diagnostic{}
				for _, d := range AnalyzeWithOptions(tc.file, source.KindOf(tc.file), root, pd, idx,
					Options{Conformance: mode}) {
					if d.Code == CodeReservedKeywordName {
						got = append(got, d)
					}
				}
				if len(got) != 1 {
					t.Fatalf("%v: got %+v, want the keyword reported exactly once", mode, got)
				}
				if got[0].Severity != SeverityError {
					t.Errorf("%v: severity = %v, want error", mode, got[0].Severity)
				}
			}
		})
	}
}

// Only what the parser recovered as a keyword-as-name is escalated: a name the
// grammar admits is not, whatever the lexer of either language knows the word as.
func TestKeywordAsNameLeavesTheNamesTheGrammarAdmits(t *testing.T) {
	for _, tc := range []struct{ name, src string }{
		// An unnamed enum usage with a value: the member the parser names `enum`
		// is its reading of the text, not a name the author wrote.
		{"a.sysml", "package T { enum def D { enum = 60; enum = 80; } }"},
		// `type` is a keyword of the other language and a legal name here
		// (stdlib Metadata/ImageMetadata.kerml).
		{"a.sysml", "package U { attribute type : X; }"},
	} {
		if got := notationDiags(t, tc.name, tc.src, conformance.ModeStrict); len(got) != 0 {
			t.Errorf("%s: got %+v, want no finding", tc.src, got)
		}
	}
}
