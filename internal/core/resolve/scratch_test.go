package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A transient name resolved inside Scratch answers as usual, and once Scratch
// ends the resolver holds nothing keyed by its nodes; what was memoized before,
// and what the run memoized about the model's own nodes, stays.
func TestScratchDropsOnlyTransientNodes(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": `package P { attribute x; attribute y; attribute z; }`,
	})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")
	kept := qn(false, "P", "x")
	if _, ok := r.ResolveQualified(root, kept); !ok {
		t.Fatal("P::x did not resolve")
	}
	before := r.MemoSize()

	owned := qn(false, "P", "z")
	for i := 0; i < 3; i++ {
		transient := qn(false, "P", "y")
		called := qn(false, "P", "y")
		r.Scratch(map[ast.Node]bool{transient: true, called: true}, func() {
			sym, ok := r.ResolveQualified(root, transient)
			if !ok || sym.Name != "y" {
				t.Fatalf("P::y inside Scratch = %v, %v", sym, ok)
			}
			if _, ok := r.ResolveInvocationName(root, called); !ok {
				t.Fatal("invocation name P::y did not resolve")
			}
			if _, ok := r.ResolveQualified(root, owned); !ok {
				t.Fatal("P::z did not resolve")
			}
			if r.MemoSize() <= before {
				t.Fatal("Scratch did not memoize while running")
			}
		})
		if _, resolved := r.PartSymbol(transient, 1); resolved {
			t.Fatal("Scratch kept a transient node's segments")
		}
	}
	if _, resolved := r.PartSymbol(kept, 1); !resolved {
		t.Fatal("Scratch dropped a resolution made before it")
	}
	if _, resolved := r.PartSymbol(owned, 1); !resolved {
		t.Fatal("Scratch dropped a model-owned resolution made under it")
	}
	// P::z's entries were added once; the transient ones three times, and gone.
	afterOwned := r.MemoSize()
	r.Scratch(map[ast.Node]bool{qn(false, "P", "y"): true}, func() {})
	if got := r.MemoSize(); got != afterOwned {
		t.Fatalf("memo size %d after an empty Scratch, want %d", got, afterOwned)
	}
	extra := qn(false, "P", "y")
	r.Scratch(map[ast.Node]bool{extra: true}, func() { r.ResolveQualified(root, extra) })
	if got := r.MemoSize(); got != afterOwned {
		t.Fatalf("memo size %d after a transient Scratch, want %d", got, afterOwned)
	}
}

// A nested Scratch drops its own transient nodes and leaves the rest to the
// enclosing one, which judges them by its own set.
func TestScratchNests(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": `package P { attribute x; attribute y; }`,
	})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")
	r.ResolveQualified(root, qn(false, "P", "x"))
	before := r.MemoSize()
	outer, inner := qn(false, "P", "x"), qn(false, "P", "y")
	r.Scratch(map[ast.Node]bool{outer: true}, func() {
		r.Scratch(map[ast.Node]bool{inner: true}, func() {
			r.ResolveQualified(root, inner)
			r.ResolveQualified(root, outer)
		})
		if _, resolved := r.PartSymbol(inner, 1); resolved {
			t.Fatal("inner Scratch kept its transient node")
		}
		if _, resolved := r.PartSymbol(outer, 1); !resolved {
			t.Fatal("inner Scratch dropped the outer call's node")
		}
	})
	if got := r.MemoSize(); got != before {
		t.Fatalf("memo size %d after nested Scratch, want %d", got, before)
	}
}

// The spellings suggested for a transient node's unresolved name are keyed by
// the name, not the node; they are the node's all the same and go with it.
func TestScratchDropsTransientSuggestions(t *testing.T) {
	idx := indexOf(t, map[string]string{
		"a.sysml": `package P { attribute mass; attribute speed; }`,
	})
	r := New(idx)
	root := idx.DocumentRoot("a.sysml")
	owned := qn(false, "maas")
	r.UnresolvedName(root, "maas", owned)
	before := r.MemoSize()
	transient := qn(false, "sped")
	r.Scratch(map[ast.Node]bool{transient: true}, func() {
		if got := r.UnresolvedName(root, "sped", transient); got == "sped" {
			t.Fatalf("no spelling suggested for sped: %q", got)
		}
		if r.MemoSize() <= before {
			t.Fatal("Scratch did not memoize the suggestion while running")
		}
	})
	if got := r.MemoSize(); got != before {
		t.Fatalf("memo size %d after Scratch, want %d: a transient name's suggestions stayed", got, before)
	}
}
