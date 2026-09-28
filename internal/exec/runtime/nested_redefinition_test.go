package runtime

import (
	"errors"
	"testing"
)

// A chain naming a feature read under a redefined alias applies however the
// member is named on the read: a redefinition shares one feature value under
// both names, so the chain matches the object either name materializes.
func TestNestedRedefinitionUnderAnAliasedName(t *testing.T) {
	model := `package test {
		private import ScalarValues::Real;
		part def Leaf { attribute value : Real default = 1.0; }
		part def Mid { part leaf : Leaf; }
		part def Base { part mid : Mid; }
		part def Top :> Base {
			part renamed :>> mid;
			attribute :>> mid.leaf.value = 99.0;
		}
		part top : Top;
	}`
	for _, first := range []string{"renamed", "mid"} {
		t.Run(first+"_read_first", func(t *testing.T) {
			ctx, idx := libraryShapeContext(t, model)
			top := instantiateQualified(t, ctx, idx, "test::top")
			second := "renamed"
			if first == "renamed" {
				second = "mid"
			}
			for _, name := range []string{first, second} {
				leaf := readInstance(t, ctx, readInstance(t, ctx, top, name), "leaf")
				fv, err := leaf.GetFeatureValue(ctx, "value")
				if err != nil {
					t.Fatalf("GetFeatureValue(%s.leaf.value): %v", name, err)
				}
				if got := realValue(t, fv.HeldValue()); got != 99.0 {
					t.Fatalf("%s.leaf.value = %v, want the chain's 99.0", name, got)
				}
			}
		})
	}
}

// A chain stating only a type leaves the target's declared default and
// multiplicity to the feature it redefines: the chain inherits them, exactly
// as a redefining body stating only a type does.
func TestNestedRedefinitionTypeOnlyChainKeepsInheritedValue(t *testing.T) {
	model := `package test {
		private import ScalarValues::Real;
		part def Leaf { attribute value : Real[2] default = (1.0, 2.0); }
		part def Mid { part leaf : Leaf; }
		part def Top { part mid : Mid; }
		part typed : Top { attribute :>> mid.leaf.value : Real; }
		part ranged : Top { attribute :>> mid.leaf.value : Real[3]; }
		part valued : Top { attribute :>> mid.leaf.value = (9.0, 8.0); }
	}`
	pair := func(t *testing.T, ctx *Context, top *Instance) []float64 {
		t.Helper()
		leaf := readInstance(t, ctx, top, "mid")
		leaf = readInstance(t, ctx, leaf, "leaf")
		fv, err := leaf.GetFeatureValue(ctx, "value")
		if err != nil {
			t.Fatalf("GetFeatureValue(value): %v", err)
		}
		var out []float64
		for _, el := range elementsOf(fv.HeldValue()) {
			out = append(out, realValue(t, el))
		}
		return out
	}
	upper := func(t *testing.T, ctx *Context, top *Instance) int64 {
		t.Helper()
		leaf := readInstance(t, ctx, top, "mid")
		leaf = readInstance(t, ctx, leaf, "leaf")
		return leaf.FeatureValues["value"].Feature.Multiplicity.Upper.Value
	}
	t.Run("type_only", func(t *testing.T) {
		ctx, idx := libraryShapeContext(t, model)
		top := instantiateQualified(t, ctx, idx, "test::typed")
		got := pair(t, ctx, top)
		if len(got) != 2 || got[0] != 1.0 || got[1] != 2.0 {
			t.Fatalf("typed.mid.leaf.value = %v, want the redefined (1.0, 2.0)", got)
		}
		if got := upper(t, ctx, top); got != 2 {
			t.Fatalf("typed.mid.leaf.value multiplicity = %v, want the redefined [2]", got)
		}
	})
	t.Run("stated_multiplicity", func(t *testing.T) {
		ctx, idx := libraryShapeContext(t, model)
		top := instantiateQualified(t, ctx, idx, "test::ranged")
		if got := upper(t, ctx, top); got != 3 {
			t.Fatalf("ranged.mid.leaf.value multiplicity = %v, want the chain's [3]", got)
		}
	})
	t.Run("stated_value", func(t *testing.T) {
		ctx, idx := libraryShapeContext(t, model)
		top := instantiateQualified(t, ctx, idx, "test::valued")
		got := pair(t, ctx, top)
		if len(got) != 2 || got[0] != 9.0 || got[1] != 8.0 {
			t.Fatalf("valued.mid.leaf.value = %v, want the chain's (9.0, 8.0)", got)
		}
	})
}

// A chain's override of a leaf the type redefines under a second name ranks
// over the other name's own declaration: both names read the chain's value,
// as the nested-body form reads it.
func TestNestedRedefinitionOverridesAnAliasedLeaf(t *testing.T) {
	model := `package test {
		private import ScalarValues::Real;
		part def Leaf {
			attribute value : Real;
			attribute renamed :>> value default = 2.0;
		}
		part def Mid { part leaf : Leaf; }
		part def Top { part mid : Mid; }
		part top : Top { attribute :>> mid.leaf.value = 99.0; }
		part top2 : Top {
			part :>> mid {
				part :>> leaf { attribute :>> value = 99.0; }
			}
		}
	}`
	read := func(t *testing.T, ctx *Context, top *Instance, name string) float64 {
		t.Helper()
		leaf := readInstance(t, ctx, readInstance(t, ctx, top, "mid"), "leaf")
		fv, err := leaf.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("GetFeatureValue(%s): %v", name, err)
		}
		return realValue(t, fv.HeldValue())
	}
	for _, root := range []string{"top", "top2"} {
		t.Run(root, func(t *testing.T) {
			ctx, idx := libraryShapeContext(t, model)
			top := instantiateQualified(t, ctx, idx, "test::"+root)
			for _, name := range []string{"value", "renamed"} {
				if got := read(t, ctx, top, name); got != 99.0 {
					t.Fatalf("%s.mid.leaf.%s = %v, want 99.0", root, name, got)
				}
			}
		})
	}

	// Two names one declaration values stay a modeling error, with or without
	// a chain: both names state a value for one feature.
	t.Run("two_valued_names_still_conflict", func(t *testing.T) {
		ctx, idx := libraryShapeContext(t, `package test {
			private import ScalarValues::Real;
			part def Leaf {
				attribute value : Real default = 1.0;
				attribute renamed :>> value default = 2.0;
			}
			part def Mid { part leaf : Leaf; }
			part def Top { part mid : Mid; }
			part top : Top { attribute :>> mid.leaf.value = 99.0; }
		}`)
		top := instantiateQualified(t, ctx, idx, "test::top")
		mid := readInstance(t, ctx, top, "mid")
		if _, err := mid.GetFeatureValue(ctx, "leaf"); !errors.Is(err, ErrConflictingRedefinition) {
			t.Fatalf("GetFeatureValue(leaf) = %v, want ErrConflictingRedefinition", err)
		}
	})
}

// Two chains in one body do what two same-named redefining members of a
// redefining body do: the later declaration wins. The nested-body form reads
// the second value; the chain form reads the same.
func TestNestedRedefinitionDuplicateChainsInOneBody(t *testing.T) {
	model := `package test {
		private import ScalarValues::Real;
		part def Leaf { attribute value : Real default = 0.0; }
		part def Mid { part leaf : Leaf; }
		part def Top { part mid : Mid; }
		part chained : Top { attribute :>> mid.leaf.value = 1.0; attribute :>> mid.leaf.value = 2.0; }
		part bodied : Top { part :>> mid { part :>> leaf { attribute :>> value = 1.0; attribute :>> value = 2.0; } } }
	}`
	for _, root := range []string{"chained", "bodied"} {
		t.Run(root, func(t *testing.T) {
			ctx, idx := libraryShapeContext(t, model)
			top := instantiateQualified(t, ctx, idx, "test::"+root)
			leaf := readInstance(t, ctx, readInstance(t, ctx, top, "mid"), "leaf")
			fv, err := leaf.GetFeatureValue(ctx, "value")
			if err != nil {
				t.Fatalf("GetFeatureValue(value): %v", err)
			}
			if got := realValue(t, fv.HeldValue()); got != 2.0 {
				t.Fatalf("%s.mid.leaf.value = %v, want the later declaration's 2.0", root, got)
			}
		})
	}
}

// A chain written in a usage nested inside a definition governs the inherited
// binding from the usage's own body, as a redefining body written there does:
// a fresh object materializes and the bound one keeps its value.
func TestNestedRedefinitionChainInsideANestedUsage(t *testing.T) {
	ctx, idx := libraryShapeContext(t, `package test {
		private import ScalarValues::Real;
		part def Leaf { attribute value : Real default = 1.0; }
		part def Mid { part leaf : Leaf; }
		part existing : Mid;
		part def Top { part mid : Mid = existing; }
		part def World {
			part a : Top { attribute :>> mid.leaf.value = 9.0; }
		}
		part w : World;
	}`)
	w := instantiateQualified(t, ctx, idx, "test::w")
	a := readInstance(t, ctx, w, "a")
	mid := readInstance(t, ctx, a, "mid")
	leaf := readInstance(t, ctx, mid, "leaf")
	fv, err := leaf.GetFeatureValue(ctx, "value")
	if err != nil {
		t.Fatalf("GetFeatureValue(value): %v", err)
	}
	if got := realValue(t, fv.HeldValue()); got != 9.0 {
		t.Fatalf("w.a.mid.leaf.value = %v, want the chain's 9.0", got)
	}
	existing := instantiateQualified(t, ctx, idx, "test::existing")
	if mid == existing {
		t.Fatalf("w.a.mid adopted the bound object, want a fresh one")
	}
	exLeaf := readInstance(t, ctx, existing, "leaf")
	exFv, err := exLeaf.GetFeatureValue(ctx, "value")
	if err != nil {
		t.Fatalf("GetFeatureValue(value): %v", err)
	}
	if got := realValue(t, exFv.HeldValue()); got != 1.0 {
		t.Fatalf("existing.leaf.value = %v, want its own 1.0", got)
	}
}

// Two owners carrying different chain values give the derived features below
// different answers: a derived shared default is the instance's own, not the
// type's, whichever owner is read first.
func TestNestedRedefinitionSharedDefaultsStayInstanceLocal(t *testing.T) {
	model := `package test {
		private import ScalarValues::Real;
		part def Leaf { attribute value : Real default = 1.0; attribute doubled = value * 2.0; }
		part def Mid { part leaf : Leaf; }
		part def Top { part mid : Mid; }
		part a : Top { attribute :>> mid.leaf.value = 9.0; }
		part b : Top { attribute :>> mid.leaf.value = 3.0; }
	}`
	for _, first := range []string{"a", "b"} {
		t.Run(first+"_read_first", func(t *testing.T) {
			ctx, idx := libraryShapeContext(t, model)
			second := "a"
			if first == "a" {
				second = "b"
			}
			want := map[string]float64{"a": 18.0, "b": 6.0}
			for _, name := range []string{first, second} {
				top := instantiateQualified(t, ctx, idx, "test::"+name)
				leaf := readInstance(t, ctx, readInstance(t, ctx, top, "mid"), "leaf")
				fv, err := leaf.GetFeatureValue(ctx, "doubled")
				if err != nil {
					t.Fatalf("GetFeatureValue(doubled): %v", err)
				}
				if got := realValue(t, fv.HeldValue()); got != want[name] {
					t.Fatalf("%s.mid.leaf.doubled = %v, want %v", name, got, want[name])
				}
			}
		})
	}
}

// A valued chain below a bound part governs the inherited binding under every
// name the part's redefinition group gives it: whichever name is read first
// materializes a fresh object the chain applies below, and the bound object
// keeps its own value.
func TestNestedRedefinitionGovernsAnAliasedBoundPart(t *testing.T) {
	model := `package test {
		private import ScalarValues::Real;
		part def Leaf { attribute value : Real default = 1.0; }
		part def Mid { part leaf : Leaf; }
		part existing : Mid;
		part def Base { part mid : Mid = existing; }
		part def Derived :> Base {
			part renamed :>> mid;
			attribute :>> mid.leaf.value = 9.0;
		}
		part d : Derived;
	}`
	for _, first := range []string{"mid", "renamed"} {
		t.Run(first+"_read_first", func(t *testing.T) {
			ctx, idx := libraryShapeContext(t, model)
			d := instantiateQualified(t, ctx, idx, "test::d")
			second := "renamed"
			if first == "renamed" {
				second = "mid"
			}
			for _, name := range []string{first, second} {
				leaf := readInstance(t, ctx, readInstance(t, ctx, d, name), "leaf")
				fv, err := leaf.GetFeatureValue(ctx, "value")
				if err != nil {
					t.Fatalf("GetFeatureValue(%s.leaf.value): %v", name, err)
				}
				if got := realValue(t, fv.HeldValue()); got != 9.0 {
					t.Fatalf("%s.leaf.value = %v, want the chain's 9.0", name, got)
				}
			}
			existing := instantiateQualified(t, ctx, idx, "test::existing")
			if readInstance(t, ctx, d, "mid") == existing {
				t.Fatalf("d.mid adopted the bound object, want a fresh one")
			}
			leaf := readInstance(t, ctx, existing, "leaf")
			fv, err := leaf.GetFeatureValue(ctx, "value")
			if err != nil {
				t.Fatalf("GetFeatureValue(existing.leaf.value): %v", err)
			}
			if got := realValue(t, fv.HeldValue()); got != 1.0 {
				t.Fatalf("existing.leaf.value = %v, want its own 1.0", got)
			}
		})
	}
}
