package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// One qualified name read in two scopes answers in each with what that scope
// sees, whichever was read first, and reports the segments of that reading.
func TestReadQualifiedAnswersPerScope(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": `
			package One { package A { attribute x; } }
			package Two { package A { attribute x; } }
		`,
	})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")
	one, two := scopeOf(t, root, "One"), scopeOf(t, root, "Two")
	name := qn(false, "A", "x")

	for round := 0; round < 2; round++ {
		for _, tc := range []struct {
			scope *symbols.Scope
			pkg   string
		}{{one, "One"}, {two, "Two"}} {
			rd := r.ReadQualified(tc.scope, name)
			sym, ok := rd.Symbol()
			if !ok || sym.Name != "x" || sym.OwnerScope.Parent() != tc.scope {
				t.Fatalf("round %d in %s: A::x = %v, %v; want that package's x",
					round, tc.pkg, sym, ok)
			}
			part, ok := rd.Part(0)
			if !ok || part.Scope != sym.OwnerScope {
				t.Fatalf("round %d in %s: segment A = %v, %v; want %s::A", round, tc.pkg, part, ok, tc.pkg)
			}
		}
	}
	if _, resolved := r.PartSymbol(name, 0); resolved {
		t.Fatal("a reading left a segment record on the written name")
	}
	if len(r.Diagnostics) != 0 {
		t.Fatalf("readings reported diagnostics: %v", r.Diagnostics)
	}
}

// A reading of a name that fails reports how it failed: the count it was
// ambiguous among, or the segments it did reach.
func TestReadQualifiedReportsFailure(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": `
			package P {
				package Twice { attribute t; attribute t; }
				package Q { attribute y; }
			}
		`,
	})
	r := New(idx)
	p := scopeOf(t, idx.DocumentRoot("a.sysml"), "P")

	rd := r.ReadQualified(p, qn(false, "Twice", "t"))
	if _, ok := rd.Symbol(); ok {
		t.Fatal("Twice::t resolved; want ambiguous")
	}
	if n, ok := rd.Ambiguity(); !ok || n != 2 {
		t.Fatalf("Twice::t ambiguity = %d, %v; want 2 candidates", n, ok)
	}

	rd = r.ReadQualified(p, qn(false, "Q", "missing"))
	if _, ok := rd.Symbol(); ok {
		t.Fatal("Q::missing resolved")
	}
	if _, ok := rd.Ambiguity(); ok {
		t.Fatal("Q::missing reported as ambiguous")
	}
	if part, ok := rd.Part(0); !ok || part.Name != "Q" {
		t.Fatalf("segment Q = %v, %v; want the package reached", part, ok)
	}
	if len(r.Diagnostics) != 0 {
		t.Fatalf("readings reported diagnostics: %v", r.Diagnostics)
	}
}
