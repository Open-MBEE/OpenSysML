package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// A feature's memoized composite-ness is dropped with the document declaring it,
// so an edit that makes the feature referential is answered anew.
func TestEffectiveIsCompositeMemoFollowsTheDeclaringDocument(t *testing.T) {
	idx := symbols.NewIndex()
	r := resolve.New(idx)
	m := NewModel(r)
	r.SetModel(m)
	r.Track()
	add := func(src string) *symbols.Symbol {
		p := parser.New(source.New("a.sysml", []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("parse diagnostics: %v", p.Diagnostics)
		}
		idx.AddDocument("a.sysml", root)
		ch := idx.TakeChanges()
		ch.Docs = map[string]bool{"a.sysml": true}
		r.Invalidate(ch)
		return nested(t, sym(t, idx.DocumentRoot("a.sysml"), "A").Scope, "Whole", "p")
	}
	composite := func(p *symbols.Symbol) bool {
		var out bool
		r.InDocument("a.sysml", func() { out = m.EffectiveIsComposite(p) })
		return out
	}
	p := add("package A { part def Whole { part p; } }")
	for _, ask := range []string{"first", "memoized"} {
		if !composite(p) {
			t.Fatalf("part p must be composite on the %s read", ask)
		}
	}
	if _, ok := m.effectiveComposite[p]; !ok {
		t.Fatalf("the settled composite-ness of p was not memoized")
	}
	p = add("package A { part def Whole { ref part p; } }")
	if len(m.effectiveComposite) != 0 {
		t.Fatalf("%d memoized answers survive the edit, want none", len(m.effectiveComposite))
	}
	if composite(p) {
		t.Fatalf("ref part p must not be composite")
	}
}
