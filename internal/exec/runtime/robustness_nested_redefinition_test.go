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
