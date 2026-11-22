package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// escapeFindings analyzes a document in the named mode and returns its
// nonstandard-notation findings, failing on a parse error.
func escapeFindings(t *testing.T, name, src string, mode diag.ConformanceMode) []diag.Diagnostic {
	t.Helper()
	root, pd, idx := analyzeInputs(t, name, src)
	if hasParseError(pd) {
		t.Fatalf("%s: the escape must stay parsed in either mode, got %+v", src, pd)
	}
	var got []diag.Diagnostic
	for _, d := range AnalyzeWithOptions(name, source.KindOf(name), root, pd, idx, Options{Conformance: mode}) {
		if d.Code == CodeNonstandardNotation {
			got = append(got, d)
		}
	}
	return got
}

// `\r` in a name or a string is a nonstandard-notation warning in the default
// mode and the same finding, as an error, under strict conformance.
func TestNonstandardEscapeEscalatesUnderStrictMode(t *testing.T) {
	for _, tc := range []struct {
		name, file, src string
		want            int
	}{
		{"name_sysml", "a.sysml", `package P { part def 'a\rb'; }`, 1},
		{"name_kerml", "a.kerml", `package P { class 'a\rb'; }`, 1},
		{"string_sysml", "a.sysml", `package P { attribute s = "a\rb\r"; }`, 2},
		{"string_kerml", "a.kerml", `package P { feature s = "a\rb\r"; }`, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			def := escapeFindings(t, tc.file, tc.src, diag.ConformanceDefault)
			strict := escapeFindings(t, tc.file, tc.src, diag.ConformanceStrict)
			if len(def) != tc.want || len(strict) != tc.want {
				t.Fatalf("default gave %d finding(s) %+v, strict %d %+v; want %d each", len(def), def, len(strict), strict, tc.want)
			}
			for i := range def {
				if def[i].Severity != diag.SeverityWarning || strict[i].Severity != diag.SeverityError {
					t.Errorf("severities = %v / %v, want warning / error", def[i].Severity, strict[i].Severity)
				}
				if def[i].Span != strict[i].Span || def[i].Message != strict[i].Message {
					t.Errorf("strict moved the finding: %+v became %+v", def[i], strict[i])
				}
				if !strings.Contains(def[i].Message, "`\\r`") {
					t.Errorf("message = %q, want it to name the escape", def[i].Message)
				}
			}
		})
	}
}

// Table 4's escape sequences are standard in either mode.
func TestTable4EscapesAreSilentInEitherMode(t *testing.T) {
	for _, tc := range []struct{ file, src string }{
		{"a.sysml", `package P { part def 'a\'\"\b\f\t\n\\b'; attribute s = "a\'\"\b\f\t\n\\b"; }`},
		{"a.kerml", `package P { class 'a\'\"\b\f\t\n\\b'; feature s = "a\'\"\b\f\t\n\\b"; }`},
	} {
		for _, mode := range []diag.ConformanceMode{diag.ConformanceDefault, diag.ConformanceStrict} {
			if got := escapeFindings(t, tc.file, tc.src, mode); len(got) != 0 {
				t.Errorf("%s: got %+v, want no finding", tc.src, got)
			}
		}
	}
}
