package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// trackedModel holds idx in a tracking resolver, the way a workspace holds its
// persistent model, and returns a function replacing the document called name.
func trackedModel(t *testing.T, idx *symbols.Index, name string) (*Model, *resolve.Resolver, func(src string) *symbols.Scope) {
	t.Helper()
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)
	r.Track()
	replace := func(src string) *symbols.Scope {
		p := parser.New(source.New(name, []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("parse diagnostics: %v", p.Diagnostics)
		}
		idx.AddDocument(name, root)
		r.Invalidate(idx.TakeChanges())
		r.InDocument(name, func() { r.ResolveDocument(name, root) })
		return idx.DocumentRoot(name)
	}
	return m, r, replace
}

// Replacing a document drops the body-application index built from its previous
// tree, so neither its old root nor its old body expressions stay referenced.
func TestBodyApplicationIndexFollowsItsDocument(t *testing.T) {
	const src = `package P {
		part def C;
		part cs : C[*];
		attribute picked = cs.{ in x; x };
	}`
	m, r, replace := trackedModel(t, stdlibIndex(t), "bodies.sysml")
	query := func(scope *symbols.Scope) []*symbols.Symbol {
		p := sym(t, scope, "P")
		body := appliedBody(t, valueOf(t, p.Scope, "picked"))
		x := sym(t, symbols.BodyExprScope(p.Scope, body), "x")
		var types []*symbols.Symbol
		r.InDocument("bodies.sysml", func() { types = m.BodyParameterElementTypes(x) })
		return types
	}
	for i := 0; i < 3; i++ {
		scope := replace(src)
		if types := query(scope); len(types) != 1 || types[0].Name != "C" {
			t.Fatalf("edit %d: x typed %v, want C", i, types)
		}
		if len(m.bodyIndexed) != 1 || len(m.bodyApplications) != 1 {
			t.Fatalf("edit %d: %d roots and %d bodies indexed, want 1 and 1", i, len(m.bodyIndexed), len(m.bodyApplications))
		}
		for root := range m.bodyIndexed {
			if root != scope {
				t.Fatalf("edit %d: index keyed by a previous root", i)
			}
		}
	}
}

// A document declaring a scalar name is part of the scalar table's input:
// replacing it drops the table, and the next query maps the new symbol.
func TestScalarTableFollowsItsDeclaringDocument(t *testing.T) {
	const src = `package ScalarValues { datatype Integer; }
	package P { attribute n : ScalarValues::Integer; }`
	m, r, replace := trackedModel(t, symbols.NewIndex(), "scalars.sysml")
	var previous *symbols.Symbol
	for i := 0; i < 3; i++ {
		scope := replace(src)
		integer := sym(t, sym(t, scope, "ScalarValues").Scope, "Integer")
		var prim PrimType
		var ok bool
		r.InDocument("scalars.sysml", func() { prim, ok = m.ScalarLatticeElement(integer) })
		if !ok || prim != PrimInteger {
			t.Fatalf("edit %d: Integer classified %v %v, want PrimInteger", i, prim, ok)
		}
		if _, stale := m.scalars[previous]; stale || len(m.scalars) != 1 {
			t.Fatalf("edit %d: table holds %d symbols, want the current Integer only", i, len(m.scalars))
		}
		previous = integer
	}
}

// A memoized member-source closure is owned by the document declaring its
// symbol, and a reader served from the memo depends on that document: editing
// it reaches what the reader derived from the closure.
func TestMemberSourcesReadersDependOnTheDeclaringDocument(t *testing.T) {
	idx := symbols.NewIndex()
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)
	r.Track()
	add := func(name, src string) {
		p := parser.New(source.New(name, []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("parse diagnostics: %v", p.Diagnostics)
		}
		idx.AddDocument(name, root)
	}
	add("a.sysml", "package A { part def Base; part def Derived :> Base; }")
	add("b.sysml", "package B { part def Other; }")
	r.Invalidate(idx.TakeChanges())
	derived := sym(t, sym(t, idx.DocumentRoot("a.sysml"), "A").Scope, "Derived")
	sources := func(from string) []*symbols.Symbol {
		var out []*symbols.Symbol
		r.InDocument(from, func() { out = m.MemberSources(derived) })
		return out
	}
	if got := sources("a.sysml"); len(got) != 1 || got[0].Name != "Base" {
		t.Fatalf("MemberSources(Derived) = %v, want Base", got)
	}
	if got := sources("b.sysml"); len(got) != 1 || got[0].Name != "Base" {
		t.Fatalf("memoized MemberSources(Derived) = %v, want Base", got)
	}
	if deps := r.Dependents("a.sysml"); len(deps) != 1 || deps[0] != "b.sysml" {
		t.Fatalf("a.sysml's dependents = %v, want b.sysml, served from the memo", deps)
	}
	var lookup []lookupSource
	r.InDocument("b.sysml", func() { lookup = m.lookupSources(derived) })
	if len(lookup) != 1 || lookup[0].sym.Name != "Base" {
		t.Fatalf("lookupSources(Derived) = %v, want Base", lookup)
	}
	add("a.sysml", "package A { part def Base; part def Derived; }")
	ch := idx.TakeChanges()
	ch.Docs = map[string]bool{"a.sysml": true}
	dropped := r.Invalidate(ch)
	if len(dropped) != 2 || dropped[0] != "a.sysml" || dropped[1] != "b.sysml" {
		t.Fatalf("editing a.sysml dropped %v, want a.sysml and its reader b.sysml", dropped)
	}
	if len(m.memberSources) != 0 || len(m.lookupOrder) != 0 {
		t.Fatalf("%d member-source and %d lookup-order closures survive the edit, want none", len(m.memberSources), len(m.lookupOrder))
	}
	derived = sym(t, sym(t, idx.DocumentRoot("a.sysml"), "A").Scope, "Derived")
	if got := sources("b.sysml"); len(got) != 0 {
		t.Fatalf("MemberSources(Derived) after the edit = %v, want none", got)
	}
}
