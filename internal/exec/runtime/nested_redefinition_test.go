package runtime

import (
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
