package runtime

import (
	"errors"
	"testing"
)

func TestRuntimeRobustnessNestedRedefinition(t *testing.T) {
	// A chain crossing a reference owns no object below it, so the redefinition
	// never applies: the feature still reads its declared default, no panic.
	t.Run("chain_through_reference_reads_default", func(t *testing.T) {
		ctx := contextOver(t, `package test {
			private import ScalarValues::Real;
			part def Leaf { attribute value : Real default = 1.0; }
			part def Mid { ref leaf : Leaf; }
			part def Top { part mid : Mid; }
			part top : Top { attribute :>> mid.leaf.value = 9.0; }
		}`)
		obj, err := ctx.Instantiate(lookupOne(t, ctx.model.resolver.Index(), "test::top"))
		if err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		mid, err := obj.GetFeatureValue(ctx, "mid")
		if err != nil {
			t.Fatalf("GetFeatureValue(mid): %v", err)
		}
		id, isObj := mid.HeldValue().Object()
		if !isObj {
			t.Fatalf("mid holds %s, want an object", mid.HeldValue().Kind)
		}
		midObj, ok := ctx.Instance(id)
		if !ok {
			t.Fatalf("object %d is not materialized", id)
		}
		if _, err := midObj.GetFeatureValue(ctx, "leaf"); err != nil {
			t.Fatalf("GetFeatureValue(leaf): %v", err)
		}
	})

	// A chain below a feature bound to an existing object states two values for
	// the feature below it, like a restating body does: the read errors with
	// ErrValuedFeatureRestated and the bound object keeps its own value.
	t.Run("chain_below_a_value_bound_feature_is_rejected", func(t *testing.T) {
		ctx := contextOver(t, `package test {
			private import ScalarValues::Real;
			part def Leaf { attribute value : Real default = 1.0; }
			part def Mid { part leaf : Leaf; }
			part def Top { part mid : Mid; }
			part existing : Mid;
			part a : Top { part :>> mid = existing; attribute :>> mid.leaf.value = 99.0; }
		}`)
		obj, err := ctx.Instantiate(lookupOne(t, ctx.model.resolver.Index(), "test::a"))
		if err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		if _, err := obj.GetFeatureValue(ctx, "mid"); !errors.Is(err, ErrValuedFeatureRestated) {
			t.Fatalf("GetFeatureValue(mid) = %v, want ErrValuedFeatureRestated", err)
		}
		existing, err := ctx.Instantiate(lookupOne(t, ctx.model.resolver.Index(), "test::existing"))
		if err != nil {
			t.Fatalf("Instantiate(existing): %v", err)
		}
		leaf := readInstance(t, ctx, existing, "leaf")
		fv, err := leaf.GetFeatureValue(ctx, "value")
		if err != nil {
			t.Fatalf("GetFeatureValue(value): %v", err)
		}
		if got := realValue(t, fv.HeldValue()); got != 1.0 {
			t.Fatalf("existing.leaf.value = %v, want its own 1.0", got)
		}
	})

	// A valued chain declared in a body more specific than the bound value's
	// governs it, as a redefining body does: a fresh object materializes, the
	// chain applies below it, and the bound object keeps its own value.
	t.Run("chain_below_an_inherited_bound_feature_governs", func(t *testing.T) {
		ctx := contextOver(t, `package test {
			private import ScalarValues::Real;
			part def Leaf { attribute value : Real default = 1.0; }
			part def Mid { part leaf : Leaf; }
			part existing : Mid;
			part def Top { part mid : Mid = existing; }
			part a : Top { attribute :>> mid.leaf.value = 99.0; }
		}`)
		obj, err := ctx.Instantiate(lookupOne(t, ctx.model.resolver.Index(), "test::a"))
		if err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		mid := readInstance(t, ctx, obj, "mid")
		leaf := readInstance(t, ctx, mid, "leaf")
		fv, err := leaf.GetFeatureValue(ctx, "value")
		if err != nil {
			t.Fatalf("GetFeatureValue(value): %v", err)
		}
		if got := realValue(t, fv.HeldValue()); got != 99.0 {
			t.Fatalf("a.mid.leaf.value = %v, want the chain's 99.0", got)
		}
		existing, err := ctx.Instantiate(lookupOne(t, ctx.model.resolver.Index(), "test::existing"))
		if err != nil {
			t.Fatalf("Instantiate(existing): %v", err)
		}
		if mid == existing {
			t.Fatalf("a.mid adopted the bound object, want a fresh one")
		}
		exLeaf := readInstance(t, ctx, existing, "leaf")
		exFv, err := exLeaf.GetFeatureValue(ctx, "value")
		if err != nil {
			t.Fatalf("GetFeatureValue(value): %v", err)
		}
		if got := realValue(t, exFv.HeldValue()); got != 1.0 {
			t.Fatalf("existing.leaf.value = %v, want its own 1.0", got)
		}
	})

	// A chain declaring only a type below a value-bound feature states no
	// value: nothing conflicts, the bound object is adopted as a body
	// declaring only a type adopts it, and the value below reads unchanged.
	t.Run("type_only_chain_below_a_bound_feature_adopts", func(t *testing.T) {
		ctx := contextOver(t, `package test {
			private import ScalarValues::Real;
			part def Leaf { attribute value : Real default = 1.0; }
			part def Mid { part leaf : Leaf; }
			part def Top { part mid : Mid; }
			part existing : Mid;
			part a : Top { part :>> mid = existing; attribute :>> mid.leaf.value : Real; }
		}`)
		obj, err := ctx.Instantiate(lookupOne(t, ctx.model.resolver.Index(), "test::a"))
		if err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		mid := readInstance(t, ctx, obj, "mid")
		leaf := readInstance(t, ctx, mid, "leaf")
		fv, err := leaf.GetFeatureValue(ctx, "value")
		if err != nil {
			t.Fatalf("GetFeatureValue(value): %v", err)
		}
		if got := realValue(t, fv.HeldValue()); got != 1.0 {
			t.Fatalf("a.mid.leaf.value = %v, want the bound object's 1.0", got)
		}
	})

	// A governed bound feature and a one-segment override can land on one
	// object: marking the first must not drop the second.
	t.Run("governed_and_overridden_features_on_one_object", func(t *testing.T) {
		ctx := contextOver(t, `package test {
			private import ScalarValues::Real;
			part def Leaf { attribute value : Real default = 1.0; }
			part def Mid { part leaf : Leaf; }
			part existing : Mid;
			part def Top { part mid : Mid = existing; attribute x : Real default = 0.0; }
			part def Sport :> Top { attribute :>> mid.leaf.value = 99.0; }
			part def Outer { part s : Sport; }
			part o : Outer { attribute :>> s.x = 7.0; }
		}`)
		obj, err := ctx.Instantiate(lookupOne(t, ctx.model.resolver.Index(), "test::o"))
		if err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		s := readInstance(t, ctx, obj, "s")
		x, err := s.GetFeatureValue(ctx, "x")
		if err != nil {
			t.Fatalf("GetFeatureValue(x): %v", err)
		}
		if got := realValue(t, x.HeldValue()); got != 7.0 {
			t.Fatalf("o.s.x = %v, want the override's 7.0", got)
		}
		mid := readInstance(t, ctx, s, "mid")
		leaf := readInstance(t, ctx, mid, "leaf")
		fv, err := leaf.GetFeatureValue(ctx, "value")
		if err != nil {
			t.Fatalf("GetFeatureValue(value): %v", err)
		}
		if got := realValue(t, fv.HeldValue()); got != 99.0 {
			t.Fatalf("o.s.mid.leaf.value = %v, want the governing chain's 99.0", got)
		}
		existing, err := ctx.Instantiate(lookupOne(t, ctx.model.resolver.Index(), "test::existing"))
		if err != nil {
			t.Fatalf("Instantiate(existing): %v", err)
		}
		exLeaf := readInstance(t, ctx, existing, "leaf")
		exFv, err := exLeaf.GetFeatureValue(ctx, "value")
		if err != nil {
			t.Fatalf("GetFeatureValue(value): %v", err)
		}
		if got := realValue(t, exFv.HeldValue()); got != 1.0 {
			t.Fatalf("existing.leaf.value = %v, want its own 1.0", got)
		}
	})

	// A chain stating a multiplicity below an adopted bound object applies to
	// it the way a redefining body does: the held object is classified by the
	// chain's declaration, so too few values violate it — both forms alike.
	t.Run("multiplicity_chain_below_an_adopted_object_errors", func(t *testing.T) {
		ctx := contextOver(t, `package test {
			private import ScalarValues::Real;
			part def Leaf { attribute value : Real[2] default = (1.0, 2.0); }
			part def Mid { part leaf : Leaf; }
			part def Top { part mid : Mid; }
			part existing : Mid;
			part top : Top { part :>> mid = existing; attribute :>> mid.leaf.value : Real[3]; }
			part top2 : Top {
				part :>> mid = existing {
					part :>> leaf { attribute :>> value : Real[3]; }
				}
			}
		}`)
		for _, root := range []string{"top", "top2"} {
			obj, err := ctx.Instantiate(lookupOne(t, ctx.model.resolver.Index(), "test::"+root))
			if err != nil {
				t.Fatalf("Instantiate(%s): %v", root, err)
			}
			leaf := readInstance(t, ctx, readInstance(t, ctx, obj, "mid"), "leaf")
			if _, err := leaf.GetFeatureValue(ctx, "value"); !errors.Is(err, ErrMultiplicityViolation) {
				t.Fatalf("%s.mid.leaf.value = %v, want ErrMultiplicityViolation", root, err)
			}
		}
	})

	// A chain stating only a type below an adopted bound object changes no
	// bound: the held object still reads its own values.
	t.Run("type_only_chain_below_an_adopted_object_reads_its_values", func(t *testing.T) {
		ctx := contextOver(t, `package test {
			private import ScalarValues::Real;
			part def Leaf { attribute value : Real[2] default = (1.0, 2.0); }
			part def Mid { part leaf : Leaf; }
			part def Top { part mid : Mid; }
			part existing : Mid;
			part top : Top { part :>> mid = existing; attribute :>> mid.leaf.value : Real; }
		}`)
		obj, err := ctx.Instantiate(lookupOne(t, ctx.model.resolver.Index(), "test::top"))
		if err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		leaf := readInstance(t, ctx, readInstance(t, ctx, obj, "mid"), "leaf")
		fv, err := leaf.GetFeatureValue(ctx, "value")
		if err != nil {
			t.Fatalf("GetFeatureValue(value): %v", err)
		}
		var got []float64
		for _, el := range elementsOf(fv.HeldValue()) {
			got = append(got, realValue(t, el))
		}
		if len(got) != 2 || got[0] != 1.0 || got[1] != 2.0 {
			t.Fatalf("top.mid.leaf.value = %v, want the adopted (1.0, 2.0)", got)
		}
	})

	// An object written to a composite feature is adopted and classified by
	// the feature, so a chain's multiplicity reaches it the same as a bound
	// one: the write of an under-sized object is refused.
	t.Run("multiplicity_chain_below_a_written_object_errors", func(t *testing.T) {
		ctx := contextOver(t, `package test {
			private import ScalarValues::Real;
			part def Leaf { attribute value : Real[2] default = (1.0, 2.0); }
			part def Mid { part leaf : Leaf; }
			part def Top { part mid : Mid; }
			part top : Top { attribute :>> mid.leaf.value : Real[3]; }
		}`)
		obj, err := ctx.Instantiate(lookupOne(t, ctx.model.resolver.Index(), "test::top"))
		if err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		mid, err := ctx.Instantiate(lookupOne(t, ctx.model.resolver.Index(), "test::Mid"))
		if err != nil {
			t.Fatalf("Instantiate(Mid): %v", err)
		}
		if err := obj.SetFeatureValue(ctx, "mid", Value{Kind: ValInstance, Instance: mid.ID}); err != nil {
			t.Fatalf("SetFeatureValue(mid): %v", err)
		}
		leaf := readInstance(t, ctx, readInstance(t, ctx, obj, "mid"), "leaf")
		if _, err := leaf.GetFeatureValue(ctx, "value"); !errors.Is(err, ErrMultiplicityViolation) {
			t.Fatalf("top.mid.leaf.value = %v, want ErrMultiplicityViolation", err)
		}
	})

	// A chain whose last segment resolves to no feature declares no nested
	// redefinition: the object materializes and the feature below reads its
	// declared default, no panic and no hang.
	t.Run("chain_to_unresolvable_feature_reads_default", func(t *testing.T) {
		ctx := contextOver(t, `package test {
			private import ScalarValues::Real;
			part def Leaf { attribute value : Real default = 1.0; }
			part def Mid { part leaf : Leaf; }
			part def Top { part mid : Mid; }
			part top : Top { attribute :>> mid.bogus.value = 9.0; }
		}`)
		obj, err := ctx.Instantiate(lookupOne(t, ctx.model.resolver.Index(), "test::top"))
		if err != nil {
			t.Fatalf("Instantiate: %v", err)
		}
		if _, err := obj.GetFeatureValue(ctx, "bogus"); !errors.Is(err, ErrNoSuchFeature) {
			t.Fatalf("GetFeatureValue(bogus) = %v, want ErrNoSuchFeature", err)
		}
	})
}
