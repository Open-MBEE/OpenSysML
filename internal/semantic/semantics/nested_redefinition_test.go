package semantics

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// nestedRedefs resolves src as one document and reports the nested
// redefinitions declared on the member name of the root package.
func nestedRedefsOf(t *testing.T, src, name string) []NestedRedefinition {
	t.Helper()
	idx := stdlibIndex(t)
	m, r, replace := trackedModel(t, idx, "n.sysml")
	scope := replace(src)
	owner := sym(t, sym(t, scope, "P").Scope, name)
	var out []NestedRedefinition
	r.InDocument("n.sysml", func() { out = m.NestedRedefinitionsOf(owner) })
	return out
}

func TestNestedRedefinitionsOf(t *testing.T) {
	src := `package P {
		private import ScalarValues::Real;
		part def Leaf { attribute value : Real; }
		part def Mid { part leaf : Leaf; attribute scale : Real; }
		part def Top { part mid : Mid; }
		part shorthand : Top {
			attribute :>> mid.scale = 4.0;
			attribute :>> mid.leaf.value = 9.0;
			attribute :>> mid.unknown.value = 1.0;
			part :>> mid { attribute :>> scale = 3.0; }
		}
	}`
	got := nestedRedefsOf(t, src, "shorthand")
	if len(got) != 2 {
		t.Fatalf("NestedRedefinitionsOf(shorthand) = %d entries, want 2: %+v", len(got), got)
	}
	for i, want := range [][]string{{"mid", "scale"}, {"mid", "leaf", "value"}} {
		if len(got[i].Path) != len(want) {
			t.Fatalf("entry %d path = %v, want %v", i, got[i].Path, want)
		}
		for j, seg := range want {
			if got[i].Path[j] != seg {
				t.Fatalf("entry %d path = %v, want %v", i, got[i].Path, want)
			}
		}
	}
	if got[0].Target == nil || got[0].Target.Name != "scale" {
		t.Fatalf("entry 0 target = %v, want scale", got[0].Target)
	}
	if got[1].Target == nil || got[1].Target.Name != "value" {
		t.Fatalf("entry 1 target = %v, want value", got[1].Target)
	}
	if got[1].Feature == nil {
		t.Fatal("entry 1 feature is nil")
	}
}

// A type with no chain redefinition lists none: a one-level redefinition and a
// nested-body redefinition are standard features.
func TestNestedRedefinitionsOfStandardOnly(t *testing.T) {
	src := `package P {
		part def Top { part mid : P::Mid; }
		part def Mid { attribute scale : ScalarValues::Real; }
		part standard : Top {
			part :>> mid { attribute :>> scale = 3.0; }
		}
	}`
	if got := nestedRedefsOf(t, src, "standard"); len(got) != 0 {
		t.Fatalf("NestedRedefinitionsOf(standard) = %+v, want none", got)
	}
}

// NestedRedefinitionsOf is memoized per owner symbol.
func TestNestedRedefinitionsOfMemoized(t *testing.T) {
	src := `package P {
		part def Top { part mid : P::Mid; }
		part def Mid { attribute scale : ScalarValues::Real; }
		part shorthand : Top { attribute :>> mid.scale = 4.0; }
	}`
	idx := stdlibIndex(t)
	m, r, replace := trackedModel(t, idx, "n.sysml")
	scope := replace(src)
	owner := sym(t, sym(t, scope, "P").Scope, "shorthand")
	var first, second []NestedRedefinition
	r.InDocument("n.sysml", func() {
		first = m.NestedRedefinitionsOf(owner)
		second = m.NestedRedefinitionsOf(owner)
	})
	if len(first) != 1 || len(second) != 1 || first[0].Feature != second[0].Feature {
		t.Fatalf("memoized read = %+v then %+v", first, second)
	}
	var _ *symbols.Symbol = first[0].Feature
}
