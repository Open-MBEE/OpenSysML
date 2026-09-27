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
