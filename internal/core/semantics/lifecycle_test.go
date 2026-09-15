package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
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
