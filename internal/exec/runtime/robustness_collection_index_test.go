package runtime

import (
	"errors"
	"testing"
)

const collectionIndexRobustnessModel = `
package test {
	private import ScalarValues::*;
	private import Collections::*;
	attribute os : OrderedSet { :>> elements : Integer = (3, 5, 7); }
	attribute bag : Bag { :>> elements : Integer = (3, 5, 7); }
}
`

// TestRuntimeRobustnessCollectionIndex pins that `#` over an OrderedCollection
// refuses a position its elements lack with a typed error, and that an unordered
// collection, which CollectionFunctions::'#' does not take, is positioned as the one
// value it is.
func TestRuntimeRobustnessCollectionIndex(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, collectionIndexRobustnessModel))
	root := idx.DocumentRoot("<test>")
	for _, tc := range []struct {
		name, expr string
		want       error
	}{
		{"position_past_the_elements", "test::os#(4)", ErrIndexOutOfRange},
		{"position_zero", "test::os#(0)", ErrIndexOutOfRange},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := evalIn(t, ctx, root, tc.expr)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s: %v, want %v", tc.expr, err, tc.want)
			}
		})
	}
	t.Run("unordered_collection_is_one_value", func(t *testing.T) {
		val, err := evalIn(t, ctx, root, "test::bag#(1)")
		if err != nil || val.Kind != ValInstance {
			t.Fatalf("test::bag#(1) = %v, %v; want the Bag object itself", val, err)
		}
	})
}
