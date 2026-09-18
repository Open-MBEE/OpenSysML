package resolve_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// A succession usage is a redefinition target like any other feature: the name
// resolves to the succession declared in the general action (F52).
const f52Model = `package SuccessionDeclarations {
	connection def Before {
		end source;
		end target;
	}
	action def Base {
		action paint;
		action dry;
		succession named : Before [1] first paint then dry;
	}
	action def Process :> Base {
		succession redefines named : Before [1] first paint then dry;
	}
}`

func TestF52SuccessionIsARedefinitionTarget(t *testing.T) {
	r, root, rootScope := resolvedDoc(t, f52Model)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("diagnostics: %v", r.Diagnostics)
	}
	var found int
	for _, ref := range resolve.References(root, rootScope) {
		if !ref.Redefines || nameText(ref.QN) != "named" {
			continue
		}
		found++
		sym, ok := r.ResolveReference(ref)
		if !ok {
			t.Fatal("`succession redefines named` does not resolve its target")
		}
		// A succession usage is classified as one, so the target carries the
		// succession kind rather than the unclassified one.
		if sym.Kind != symbols.SymbolSuccessionUsage {
			t.Errorf("target kind %v, want %v", sym.Kind, symbols.SymbolSuccessionUsage)
		}
	}
	if found != 1 {
		t.Errorf("%d redefinition references to `named`, want 1", found)
	}
}

// A redefinition of a succession that no general declares stays unresolved.
func TestF52SuccessionRedefinesUnknownTargetFails(t *testing.T) {
	r, _, _ := resolvedDoc(t, `package S {
	connection def Before {
		end source;
		end target;
	}
	action def Process {
		action paint;
		action dry;
		succession redefines nosuchsuccession : Before [1] first paint then dry;
	}
}`)
	if len(r.Diagnostics) == 0 {
		t.Error("expected an unresolved-reference diagnostic for a redefinition of nothing")
	}
}
