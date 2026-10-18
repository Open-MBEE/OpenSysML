package runtime

import (
	"errors"
	"testing"
)

const referenceReadsRobustnessModel = `
package test {
	part def Node;
	part def Rig {
		part own : Node;
		ref part via ::> own;
		ref part loner : Node;
	}
	part def Cycle {
		ref part a : Node ::> b;
		ref part b : Node ::> a;
	}
	part def Far {
		part rig : Rig;
		ref part viaFar ::> rig.own;
	}
	part rig : Rig;
}
`

// TestRuntimeRobustnessReferenceReads pins the reads a reference usage refuses with a
// typed error: a cycle of reference subsettings, a target on another object, a member
// its type lacks, and an unbound required reference on an object no model stands behind.
func TestRuntimeRobustnessReferenceReads(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, referenceReadsRobustnessModel))
	root := idx.DocumentRoot("<test>")
	for _, tc := range []struct {
		name, expr string
		want       error
	}{
		{"cycle_of_reference_subsettings", "new test::Cycle().a", errUninitializedFeatureValue},
		{"target_on_another_object", "new test::Far().viaFar", errUninitializedFeatureValue},
		{"unbound_reference_on_object", "new test::Rig().loner", errUninitializedFeatureValue},
		{"member_not_declared_through_reference", "new test::Rig().via.nosuch", ErrNoSuchFeature},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := evalIn(t, ctx, root, tc.expr)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s: %v, want %v", tc.expr, err, tc.want)
			}
		})
	}
	t.Run("unbound_reference_on_model_object_is_open", func(t *testing.T) {
		val, err := evalIn(t, ctx, root, "test::rig.loner")
		if err != nil || val.Kind != ValUndetermined {
			t.Fatalf("test::rig.loner = %v, %v; want an undetermined value", val, err)
		}
	})
}
