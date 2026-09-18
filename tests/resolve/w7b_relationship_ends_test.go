package resolve_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const relationshipMembers = `package P {
	classifier A { feature next : A; }
	classifier B { feature prev : B; }
	feature f : A;
	feature g : B;
	specialization Gen subtype A specializes B;
	specialization Sub subset f subsets g;
	featuring F of f by A;
	inverting i inverse f of g;
	disjoining D disjoint A from B;
	disjoint f.next from g.prev;
}
`

// relationshipWorkspace resolves a KerML document of keyword-first relationships.
func relationshipWorkspace(t *testing.T) (*resolve.Resolver, *symbols.Index) {
	t.Helper()
	f := source.New("w7b-relationships.kerml", []byte(relationshipMembers))
	p := parser.New(f)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndex()
	idx.AddDocument(f.Name(), root)
	idx.ExpandWildcardImports()
	r := resolve.New(idx)
	r.SetModel(semantics.NewModel(r))
	r.ResolveDocument(f.Name(), root)
	return r, idx
}

// TestW7BRelationshipEndsResolve checks both ends of every keyword-first
// relationship name the element they were written for. When the ends were two
// relationships of an anonymous usage, the second read as a second general of
// the first (F86–F91).
func TestW7BRelationshipEndsResolve(t *testing.T) {
	r, _ := relationshipWorkspace(t)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("expected every relationship end to resolve, got %v", r.Diagnostics)
	}
}

// TestW7BRelationshipMemberIsNamed checks the relationship a name is written for
// is the element that name denotes.
func TestW7BRelationshipMemberIsNamed(t *testing.T) {
	_, idx := relationshipWorkspace(t)
	for _, name := range []string{"P::Gen", "P::Sub", "P::F", "P::i", "P::D"} {
		syms := idx.LookupQualified(name)
		if len(syms) != 1 {
			t.Fatalf("%s names %d symbols, want 1", name, len(syms))
		}
		if syms[0].Kind != symbols.SymbolRelationship {
			t.Errorf("%s has kind %v, want %v", name, syms[0].Kind, symbols.SymbolRelationship)
		}
	}
}
