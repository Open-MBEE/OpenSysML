package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// kermlOnlyKindWords are the declaration keywords of KerML.xtext that SysML v2
// 8.2.2.1.2 does not reserve, so each is a legal name in a SysML file.
var kermlOnlyKindWords = []string{
	"assoc", "behavior", "bool", "class", "classifier", "connector", "datatype", "expr",
	"feature", "function", "interaction", "inv", "metaclass", "predicate", "step", "struct",
	"subclassifier", "type",
}

// keywordNamedUsages declare a usage named W after the SysML keywords that open it,
// in each tail namesDeclaration does not already settle.
var keywordNamedUsages = []string{
	"part def T { attribute a; action W subsets a; }",
	"part def T { attribute a; action W subsets a { } }",
	"part def T { attribute a; part W :> a; }",
	"part def T { attribute a; ref item W redefines a; }",
	"part def T { attribute a; attribute W = 1; }",
	"part def T { part W [2]; }",
	"part def T { attribute a; in item W :>> a; }",
	"part def T { attribute a; individual part W subsets a; }",
	"part def T { attribute a; perform action W subsets a; }",
	"part def T { attribute a; exhibit state W subsets a; }",
	"part def T { attribute a; event occurrence W subsets a; }",
	"part def T { attribute a; snapshot W subsets a; }",
	"part def T { attribute a; timeslice W subsets a; }",
	"part def V { variation part p { variant part W subsets p; } }",
}

func TestKerMLOnlyWordsAreNotSysMLReserved(t *testing.T) {
	for _, w := range kermlOnlyKindWords {
		if !source.IsKeywordIn(w, source.KindKerML) || source.IsKeywordIn(w, source.KindSysML) {
			t.Errorf("%s: want a KerML keyword SysML does not reserve", w)
		}
	}
}

// A word SysML does not reserve names the usage whose SysML keyword precedes it
// (spec example 8.4.13.6/8, `action behavior subsets Parts::Part::ownedActions`).
func TestKerMLOnlyWordNamingAUsageIsNotKerMLNotation(t *testing.T) {
	for _, w := range kermlOnlyKindWords {
		for _, form := range keywordNamedUsages {
			src := "package P { " + strings.ReplaceAll(form, "W", w) + " }"
			root, pd, _ := analyzeInputs(t, "a.sysml", src)
			if hasParseError(pd) {
				t.Errorf("%s: got parse errors %+v, want none", src, pd)
				continue
			}
			if got := notationDiags(t, "a.sysml", src, diag.ConformanceStrict); len(got) != 0 {
				t.Errorf("%s: got %+v, want no finding", src, got)
			}
			var named bool
			ast.Inspect(root, func(n ast.Node) bool {
				if u, ok := n.(*ast.Usage); ok && u.Ident.Name == w && u.Keyword != w {
					named = true
				}
				return true
			})
			if !named {
				t.Errorf("%s: want a usage named %s", src, w)
			}
		}
	}
}

// The same word in declaration position is still KerML notation, as is a word
// SysML reserves wherever it stands.
func TestKerMLOnlyWordInDeclarationPositionIsStillReported(t *testing.T) {
	for _, src := range []string{
		"package P { part def T { behavior B; } }",
		"package P { part def T { feature f subsets g; attribute g; } }",
		"package P { part def T { struct S specializes T; } }",
		"package P { part def T { abstract class C; } }",
		"package P { part def T { ref feature f; } }",
	} {
		wantNotationError(t, "a.sysml", src, CodeKerMLNotation, "is KerML notation")
	}
}

// KerML reserves no SysML kind keyword, so there the SysML word is the name.
func TestSysMLOnlyWordNamingAKerMLFeatureIsItsName(t *testing.T) {
	const src = "package P { class C { feature a; feature part subsets a; step action :> a; } }"
	root, pd, _ := analyzeInputs(t, "a.kerml", src)
	if hasParseError(pd) {
		t.Fatalf("got parse errors %+v, want none", pd)
	}
	names := map[string]bool{}
	ast.Inspect(root, func(n ast.Node) bool {
		if u, ok := n.(*ast.Usage); ok {
			names[u.Ident.Name] = true
		}
		return true
	})
	if !names["part"] || !names["action"] {
		t.Errorf("got usages %v, want ones named part and action", names)
	}
}
