package parser

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/lexer"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// otherLanguageKeywords returns the keywords only the other language reserves:
// in a `.sysml` file the KerML-only literals, in `.kerml` the SysML-only ones.
func otherLanguageKeywords(kind source.Kind) []string {
	other := source.KindKerML
	if kind == source.KindKerML {
		other = source.KindSysML
	}
	var words []string
	for _, kw := range source.Keywords() {
		if source.IsKeywordIn(kw, other) && !source.IsKeywordIn(kw, kind) {
			words = append(words, kw)
		}
	}
	return words
}

// parseClean parses input as a file of the given name and fails on any diagnostic.
func parseDump(t *testing.T, name, input string) string {
	t.Helper()
	p := New(source.New(name, []byte(input)))
	f := p.ParseFile()
	for _, d := range p.Diagnostics {
		t.Errorf("%s: unexpected diagnostic: %s", name, d.Message)
	}
	return ast.Dump(f)
}

// Xtext reserves a literal only within the grammar that declares it, so a word
// only the other language spells is an ordinary name: it declares a feature, is
// read before an operator, closes a body as its result, and is referenced
// qualified or through a chain, with nothing to report.
func TestOtherLanguageKeywordsAreNames(t *testing.T) {
	for _, kw := range otherLanguageKeywords(source.KindSysML) {
		src := strings.ReplaceAll(`package P {
	attribute def T;
	part def H {
		attribute KW : T;
		attribute a : T = KW + 1;
		attribute b : T = KW <= 3 and KW < 3;
		assert constraint { KW <= (a - b) }
		assert constraint { KW < a }
		calc c : T { KW }
		part n { attribute KW : T; }
		attribute d : T = H::KW + n.KW;
		attribute e : T = (KW);
	}
}
`, "KW", kw)
		dump := parseDump(t, "sysml_"+kw+".sysml", src)
		for _, want := range []string{`name="` + kw + `"`, `(FeatureReference name="` + kw + `")`} {
			if !strings.Contains(dump, want) {
				t.Errorf("a.sysml %q: no %s in\n%s", kw, want, dump)
			}
		}
	}
	for _, kw := range otherLanguageKeywords(source.KindKerML) {
		src := strings.ReplaceAll(`package P {
	datatype T;
	struct H {
		feature KW : T;
		feature a : T = KW + 1;
		feature b : T = KW <= 3 and KW < 3;
		inv { KW <= (a - b) }
		inv { KW < a }
		expr c : T { KW }
		feature n { feature KW : T; }
		feature d : T = H::KW + n.KW;
		feature e : T = (KW);
	}
}
`, "KW", kw)
		dump := parseDump(t, "kerml_"+kw+".kerml", src)
		for _, want := range []string{`name="` + kw + `"`, `(FeatureReference name="` + kw + `")`} {
			if !strings.Contains(dump, want) {
				t.Errorf("a.kerml %q: no %s in\n%s", kw, want, dump)
			}
		}
	}
}

// A keyword of the other notation stays the keyword only where a named
// declaration of that notation follows it, which the notation pass then
// reports; before anything a name can be followed by — a body and a
// multiplicity included — it is the name.
func TestOtherLanguageKeywordReadsAsName(t *testing.T) {
	for _, tc := range []struct {
		file, input string
		keyword     bool // the word is read as a keyword
	}{
		{"a.sysml", "step s;", true},
		{"a.sysml", "step <s> step;", true},
		{"a.sysml", "step <s>;", true},
		{"a.sysml", "step <s> x := 1;", true},
		{"a.sysml", "step <s> := 1;", true},
		{"a.kerml", "action <a> x := 1;", true},
		{"a.sysml", "part def D { calc c { step < upperBound > lowerBound } }", false},
		{"a.sysml", "part def D { calc c { (step < upperBound) == (step > lowerBound) } }", false},
		{"a.sysml", "step #M s;", true},
		{"a.sysml", "feature redefines x;", true},
		{"a.sysml", "part def D { inv { x > 0 } }", false},
		{"a.sysml", "part def D { connector [1] a to b; }", false},
		{"a.sysml", "namespace { }", false},
		{"a.sysml", "part def D { step { } }", false},
		{"a.sysml", "part def D { step [1]; }", false},
		{"a.kerml", "struct S { action { } }", false},
		{"a.kerml", "struct S { part [1]; }", false},
		{"a.sysml", "part def D { step : T; }", false},
		{"a.sysml", "part def D { ref step : T; }", false},
		{"a.sysml", "part def D { expr; }", false},
		{"a.sysml", "part def D { expr = 1; }", false},
		{"a.sysml", "part def D { chains : T; }", false},
		{"a.sysml", "part def D { member; }", false},
		{"a.sysml", "part def D { composite { } }", false},
		{"a.sysml", "part def D { calc c { step <= 3 } }", false},
		{"a.sysml", "part def D { calc c { step < 3 } }", false},
		{"a.sysml", "part def D { calc c { step#(1) } }", false},
		{"a.sysml", "part def D { calc c { if ok ? step else other } }", false},
		{"a.sysml", "part def D { calc c { step and other } }", false},
		{"a.sysml", "part def D { calc c { step.x } }", false},
		{"a.sysml", "part def D { calc c { step(1) } }", false},
		{"a.sysml", "part def D { action a { assign step := 1; } }", false},
		{"a.kerml", "action a;", true},
		{"a.kerml", "struct S { connection c connect a to b; }", true},
		{"a.kerml", "struct S { constraint { x > 0 } }", false},
		{"a.kerml", "struct S { connect [1] a to b; }", false},
		{"a.kerml", "struct S { action : T; }", false},
		{"a.kerml", "struct S { part; }", false},
		{"a.kerml", "struct S { inv { action <= 3 } }", false},
		{"a.kerml", "struct S { expr while { in snapshot : T; } }", false},
	} {
		sc := NewScanner(source.New(tc.file, []byte(tc.input)))
		var tok lexer.Token
		for {
			tok = sc.Next()
			if tok.Kind == lexer.EOF {
				t.Fatalf("%s %q: no keyword of the other language in the input", tc.file, tc.input)
			}
			if tok.Kind == lexer.Keyword && !Reserves(source.New(tc.file, nil).Kind(), tok.KeywordID) {
				break
			}
			if tok.Kind == lexer.Identifier && source.IsKeyword(tc.input[tok.Span.Offset:tok.Span.End()]) {
				break
			}
		}
		if got := tok.Kind == lexer.Keyword; got != tc.keyword {
			t.Errorf("%s %q: %q read as keyword = %v, want %v", tc.file, tc.input, tc.input[tok.Span.Offset:tok.Span.End()], got, tc.keyword)
		}
	}
}

// The reclassification looks past trivia and comments to the token after the word.
func TestOtherLanguageKeywordLooksPastTrivia(t *testing.T) {
	dump := parseDump(t, "trivia.sysml", "part def D { attribute step : T; calc c { step /* bound */ // note\n <= 3 } }")
	if !strings.Contains(dump, `(FeatureReference name="step")`) {
		t.Errorf("step before a comment is not read as a name:\n%s", dump)
	}
}
