package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A workspace holding two copies of the standard library declares every
// metaclass twice; a filter naming one still classifies the candidates by it.
func TestExposedElementsFilterByAMetaclassDeclaredTwice(t *testing.T) {
	idx := stdlibIndex(t)
	add := func(name, src string) *ast.RootNamespace {
		p := parser.New(source.New(name, []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("%s: parse diagnostics: %v", name, p.Diagnostics)
		}
		idx.AddDocument(name, root)
		return root
	}
	const metaclassCopy = `package SysML { package Systems { metadata def PartUsage; } }`
	add("lib_one.sysml", metaclassCopy)
	add("lib_two.sysml", metaclassCopy)
	const name = "t.sysml"
	root := add(name, `
		package Model {
			part abcd {
				part a { port a; }
				part b { port b; }
				action d;
				interface i connect (a.a, b.b);
				part c;
				port p;
			}
			private import StandardViewDefinitions::*;
			view filtered : InterconnectionView { expose abcd; filter @PartUsage; }
			view unfiltered : InterconnectionView { expose abcd; }
		}
	`)
	r := resolve.New(idx)
	m := NewModel(r)
	r.ResolveDocument(name, root)
	scope := idx.DocumentRoot(name)
	wantNames(t, "unfiltered", exposedNames(t, m, nestedSym(t, scope, "Model::unfiltered")), []string{"abcd"})
	wantNames(t, "filtered by @PartUsage", exposedNames(t, m, nestedSym(t, scope, "Model::filtered")), []string{"abcd"})
	if !m.MetaclassConforms(nestedSym(t, scope, "Model::abcd"), "SysML::Systems::Usage") {
		t.Fatalf("MetaclassConforms(abcd, Usage) = false, want true")
	}
	if m.MetaclassConforms(nestedSym(t, scope, "Model::abcd::d"), "SysML::Systems::PartUsage") {
		t.Fatalf("MetaclassConforms(action d, PartUsage) = true, want false")
	}
}
