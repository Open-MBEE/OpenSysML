package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// The unary `~` is the one operator KerML 1.0 §8.2.5.8.1 leaves undefined —
// abstract `DataFunctions::'~'`, concretely defined by no library — so a tool
// warns where it is used: in a feature value, a filter condition or a
// multiplicity bound, in KerML and SysML alike, once at each use. `not` and
// `-`, which the library does define, draw nothing. The pinned pilot evaluates
// none of it, so the warning is one-sided.
func TestUndefinedOperatorWarns(t *testing.T) {
	kerml := `package P {
	private import ScalarValues::*;
	feature x : Integer;
	feature b : Boolean;
	feature a = ~5;
	feature c = ~x;
	feature d = ~b;
	feature e = 1 + ~2;
	feature f = ~1.5;
	feature ok = not b;
	feature ok2 = -x;
}`
	wantOperatorDiags(t, "a.kerml", codeUndefinedOperator, kerml,
		"5:14 operator '~' invokes DataFunctions::'~'",
		"6:14 operator '~' invokes DataFunctions::'~'",
		"7:14 operator '~' invokes DataFunctions::'~'",
		"8:18 operator '~' invokes DataFunctions::'~'",
		"9:14 operator '~' invokes DataFunctions::'~'")
	// Strict conformance judges the notation, not this finding: §8.2.5.8.1
	// asks for a warning, so strict mode keeps it one.
	root := parser.New(source.New("a.kerml", []byte(kerml))).ParseFile()
	idx := newTestIndex()
	idx.AddDocument("a.kerml", root)
	idx.ExpandWildcardImports()
	strict := 0
	for _, d := range AnalyzeWithOptions("a.kerml", source.KindKerML, root, nil, idx, Options{Conformance: diag.ConformanceStrict}) {
		if d.Code == codeUndefinedOperator {
			strict++
			if d.Severity != diag.SeverityWarning {
				t.Errorf("strict severity = %v, want warning", d.Severity)
			}
		}
	}
	if strict != 5 {
		t.Errorf("strict mode gave %d undefined-operator diagnostics, want 5", strict)
	}

	wantOperatorDiags(t, "a.sysml", codeUndefinedOperator, `package P {
	private import ScalarValues::*;
	attribute v = ~5;
	package Q { filter ~1 == 2; }
}`,
		"3:16 operator '~' invokes DataFunctions::'~'",
		"4:21 operator '~' invokes DataFunctions::'~'")
}

// The pass reads only the written operator at the syntax tier, so a name it
// cannot resolve elsewhere in the same document never hides the warning: both
// the unresolved-reference error and the `~` warning are reported together.
func TestUndefinedOperatorSurvivesUnresolvedReference(t *testing.T) {
	for _, tc := range []struct {
		name, src string
	}{
		{"a.kerml", `package P {
	feature bad = missing;
	feature value = ~5;
}`},
		{"a.sysml", `package P {
	attribute bad = missing;
	attribute value = ~5;
}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := parser.New(source.New(tc.name, []byte(tc.src))).ParseFile()
			idx := newTestIndex()
			idx.AddDocument(tc.name, root)
			idx.ExpandWildcardImports()
			unresolved, warnings := false, 0
			for _, d := range Analyze(tc.name, root, nil, idx) {
				if d.Code == codeUndefinedOperator {
					warnings++
				}
				if d.Severity == diag.SeverityError && strings.Contains(d.Message, "unresolved reference: missing") {
					unresolved = true
				}
			}
			if !unresolved {
				t.Errorf("no unresolved-reference error for `missing` in %s", tc.name)
			}
			if warnings != 1 {
				t.Errorf("%d undefined-operator warnings in %s, want 1", warnings, tc.name)
			}
		})
	}
}
