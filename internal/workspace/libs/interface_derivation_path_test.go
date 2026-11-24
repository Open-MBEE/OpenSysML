package libs

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// Parts held as an interface record declare no AST node, yet a derivation between the
// requirement two of them inherit still runs between two requirements.
func TestDerivationEdgesTellRecordedPartsApart(t *testing.T) {
	const partsName, linksName = "parts.sysml", "links.sysml"
	const partsText = `package Parts { part def D { requirement r; } part a : D; part b : D; }`
	const linksText = `package Links {
		private import RequirementDerivation::*;
		private import Parts::*;
		#derivation connection two { end #original ::> a.r; end #derive ::> b.r; }
		#derivation connection one { end #original ::> a.r; end #derive ::> Parts::a.r; }
	}`
	idx := NewModelIndex()
	partsRoot := parser.New(source.New(partsName, []byte(partsText))).ParseFile()
	idx.AddDocument(partsName, partsRoot)
	r := resolve.New(idx)
	model := semantics.NewModel(r)
	r.SetModel(model)
	r.ResolveDocument(partsName, partsRoot)
	rec, err := WriteInterface(partsName, source.KindSysML, idx, r, model, nil, nil)
	if err != nil {
		t.Fatalf("WriteInterface: %v", err)
	}
	recorded, err := symbols.BuildRecorded(rec.Scope, partsName)
	if err != nil {
		t.Fatalf("BuildRecorded: %v", err)
	}
	ridx := NewModelIndex()
	ridx.AddRecordedDocument(partsName, source.KindSysML, recorded, nil)
	linksRoot := parser.New(source.New(linksName, []byte(linksText))).ParseFile()
	ridx.AddDocument(linksName, linksRoot)
	ridx.ExpandWildcardImports()
	rr := resolve.New(ridx)
	rmodel := semantics.NewModel(rr)
	rr.SetModel(rmodel)
	rr.ResolveDocument(linksName, linksRoot)
	if a := lookupPath(t, ridx.DocumentRoot(partsName), "Parts", "a"); !a.Recorded() {
		t.Fatal("Parts::a is not a recorded symbol")
	}
	for name, same := range map[string]bool{"two": false, "one": true} {
		edges := rmodel.RelationshipEdgesOf(lookupPath(t, ridx.DocumentRoot(linksName), "Links", name), semantics.RelationshipDerivation)
		if len(edges) != 1 {
			t.Errorf("%s: %d derivation edges, want 1", name, len(edges))
			continue
		}
		if got := edges[0].SameEnds(); got != same {
			t.Errorf("%s: same ends = %v, want %v", name, got, same)
		}
	}
}
