package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// reservedNameWarnings counts the reserved-keyword-name warnings of a parse.
func reservedNameWarnings(t *testing.T, name, input string) (warnings int, errs []string) {
	t.Helper()
	p := New(source.New(name, []byte(input)))
	p.ParseFile()
	for _, w := range p.Warnings {
		if w.Code == codeReservedKeywordName {
			warnings++
		}
	}
	for _, d := range p.Diagnostics {
		errs = append(errs, d.Message)
	}
	return warnings, errs
}

// Xtext reserves a literal within the grammar that declares it, and KerML.xtext
// and SysML.xtext share only KerMLExpressions.xtext. So each language's reserved
// set is its own, which the pinned validators confirm both ways: `part chains`
// is clean in SysML and `feature chains` is not in KerML, while `feature frame`
// is clean in KerML and `part frame` is not in SysML.
func TestW7CReservedWordsArePerLanguage(t *testing.T) {
	for _, tc := range []struct {
		file  string
		input string
		want  int
	}{
		// KerML-only literals name a SysML usage, with nothing to report.
		{"a.sysml", "part chains : T;", 0},
		{"a.sysml", "part type : T;", 0},
		{"a.sysml", "part multiplicity : T;", 0},
		{"a.sysml", "part step : T;", 0},
		{"a.sysml", "part namespace : T;", 0},
		// SysML's own literals still need the quotes of an unrestricted name.
		{"a.sysml", "part all : T;", 1},
		{"a.sysml", "part frame : T;", 1},
		{"a.sysml", "part state : T;", 1},
		// And the other way around, in a KerML source.
		{"a.kerml", "feature frame : T;", 0},
		{"a.kerml", "feature entry : T;", 0},
		{"a.kerml", "feature accept : T;", 0},
		{"a.kerml", "feature render : T;", 0},
		{"a.kerml", "feature type : T;", 1},
		{"a.kerml", "feature multiplicity : T;", 1},
		// `all` is KerML's isSufficient prefix here, not a name.
		{"a.kerml", "feature all : T;", 0},
	} {
		got, errs := reservedNameWarnings(t, tc.file, tc.input)
		if len(errs) > 0 {
			t.Errorf("%s %q: unexpected errors %v", tc.file, tc.input, errs)
		}
		if got != tc.want {
			t.Errorf("%s %q: %d reserved-keyword-name warning(s), want %d", tc.file, tc.input, got, tc.want)
		}
	}
}

// A kindless body member named by an unreserved word keeps that name rather
// than becoming an anonymous usage.
func TestW7CUnreservedWordNamesBodyMember(t *testing.T) {
	src := "part def T {\n  chains : T;\n  ref differences : T;\n}\n"
	p := New(source.New("body.sysml", []byte(src)))
	f := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("unexpected diagnostics: %v", p.Diagnostics)
	}
	dump := ast.Dump(f)
	for _, name := range []string{"chains", "differences"} {
		if want := `name="` + name + `"`; !strings.Contains(dump, want) {
			t.Errorf("no %s in\n%s", want, dump)
		}
	}
}

// A word the language does not reserve still states the relationship it always
// did where one belongs, so the KerML notation a SysML file may carry keeps
// parsing (`part x chains a.b`, not a usage named `chains`).
func TestW7CRelationshipWordsStillReadAsRelationships(t *testing.T) {
	for _, input := range []string{
		"part def T { part a; part b : T subsets a; }",
		"part def T { part a; part b : T disjoint from a; }",
		"part def T { part a; part b : T unions a; }",
		"part def T { part a; part b : T inverse of a; }",
		"part def T { part a; part b featured by T; }",
	} {
		p := New(source.New("rel.sysml", []byte(input)))
		p.ParseFile()
		if len(p.Diagnostics) > 0 {
			t.Errorf("%q: %v", input, p.Diagnostics)
		}
	}
}
