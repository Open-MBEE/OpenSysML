package passes

import (
	"reflect"
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// preparedBatch is a batch whose annotation types are reached every way member
// lookup can go: declared, inherited, through an inherited import, a filtered
// import, a typed usage, and across documents.
var preparedBatch = map[string]string{
	"meta.sysml": `package Meta {
	metadata def Tag { attribute n; }
	metadata def Other { attribute m; }
}`,
	"model.sysml": `package P {
	part def Base { public import Meta::*; }
	part def Sub :> Base;
	part def Filtered { public import Meta::*[@Meta::Tag]; }
	part b : Base;
	part def Owned { metadata def Own { attribute n; } }
	part def Derived :> Owned;
	part def C {
		@Meta::Tag { n = 1; }
		@Sub::Tag { n = 2; }
		@Filtered::Tag { n = 3; }
		@b::Tag { n = 4; }
		@Derived::Own { n = 5; }
		@Missing { n = 6; }
	}
}`,
}

func indexedBatch(t *testing.T, docs map[string]string) (*symbols.Index, map[string]*ast.RootNamespace) {
	t.Helper()
	idx := symbols.NewIndex()
	roots := make(map[string]*ast.RootNamespace, len(docs))
	for name, src := range docs {
		p := parser.New(source.New(name, []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("%s: parse diagnostics %v", name, p.Diagnostics)
		}
		idx.AddDocument(name, root)
		roots[name] = root
	}
	idx.ExpandWildcardImports()
	return idx, roots
}

// ownersOf renders the owner of every scope of the tree in tree order, "" for none.
func ownersOf(idx *symbols.Index, root *symbols.Scope) []string {
	var out []string
	var visit func(*symbols.Scope)
	visit = func(s *symbols.Scope) {
		owner := ""
		if s.Owner() != nil {
			owner = idx.GetFQN(s.Owner())
		}
		out = append(out, owner)
		for _, c := range s.Children() {
			visit(c)
		}
	}
	visit(root)
	return out
}

// Preparing a batch links exactly the annotation bodies analyzing its documents
// links, wherever the annotation type is found, so analysis writes nothing.
func TestPrepareBatchLinksWhatAnalysisLinks(t *testing.T) {
	const name = "model.sysml"
	analyzed, roots := indexedBatch(t, preparedBatch)
	AnalyzeWithOptions(name, source.KindSysML, roots[name], nil, analyzed, Options{})
	want := ownersOf(analyzed, analyzed.DocumentRoot(name))
	if !slices.ContainsFunc(want, func(o string) bool { return o != "" }) {
		t.Fatal("analysis linked no annotation body, so the comparison proves nothing")
	}

	prepared, _ := indexedBatch(t, preparedBatch)
	PrepareBatch(prepared, &Batch{Documents: []string{"meta.sysml", name}})
	if got := ownersOf(prepared, prepared.DocumentRoot(name)); !reflect.DeepEqual(got, want) {
		t.Errorf("preparing links owners\n%q\nwant those analysis links\n%q", got, want)
	}

	// The gathers read every workspace document, so a batch of one document
	// prepares the others' bodies too.
	others, _ := indexedBatch(t, preparedBatch)
	PrepareBatch(others, &Batch{Documents: []string{"meta.sysml"}})
	if got := ownersOf(others, others.DocumentRoot(name)); !reflect.DeepEqual(got, want) {
		t.Errorf("preparing a batch without %s links its owners\n%q\nwant those analysis links\n%q", name, got, want)
	}
}
