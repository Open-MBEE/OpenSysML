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
	part def Derived :> Rig {
		part renamed :>> own;
	}
	part rig : Rig;
	part derived : Derived;
}
`

// TestRuntimeRobustnessReferenceReads pins the reads a reference usage refuses with a
// typed error: a cycle of reference subsettings, a target on another object, a member
// its type lacks, and an unbound required reference on an object no model stands behind;
// and that the reference follows its source through a redefinition under another name
// and after the source is written.
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
	t.Run("source_redefined_under_another_name", func(t *testing.T) {
		via, err := evalIn(t, ctx, root, "test::derived.via")
		if err != nil || via.Kind != ValInstance {
			t.Fatalf("test::derived.via = %v, %v; want the renamed object", via, err)
		}
		renamed, err := evalIn(t, ctx, root, "test::derived.renamed")
		if err != nil || renamed.Instance != via.Instance {
			t.Fatalf("test::derived.renamed = %v, %v; want object #%d", renamed, err, via.Instance)
		}
	})
	t.Run("source_written_after_the_reference_was_read", func(t *testing.T) {
		rig, err := evalIn(t, ctx, root, "test::rig")
		if err != nil {
			t.Fatal(err)
		}
		before, err := evalIn(t, ctx, root, "test::rig.via")
		if err != nil || before.Kind != ValInstance {
			t.Fatalf("test::rig.via = %v, %v; want an object", before, err)
		}
		other, err := evalIn(t, ctx, root, "new test::Node()")
		if err != nil {
			t.Fatal(err)
		}
		inst, _ := ctx.Instance(rig.Instance)
		if err := inst.SetFeatureValue(ctx, "own", other); err != nil {
			t.Fatal(err)
		}
		after, err := evalIn(t, ctx, root, "test::rig.via")
		if err != nil || after.Instance != other.Instance {
			t.Fatalf("test::rig.via after own := new Node() = %v, %v; want object #%d (was #%d)", after, err, other.Instance, before.Instance)
		}
	})
}
