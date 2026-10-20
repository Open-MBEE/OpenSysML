package runtime

import (
	"errors"
	"testing"
)

const kernelCollectionRobustnessModel = `
	package test {
		part def P1 {
			part pa;
			part pb;
		}
		part p1 : P1;
	}
`

// TestRuntimeRobustnessKernelCollections pins what a derived Kernel collection read leaves
// alone: non-collections and frame roots stay typed ErrNoSuchFeature, and nothing panics.
func TestRuntimeRobustnessKernelCollections(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, kernelCollectionRobustnessModel))
	matches := idx.LookupQualified("test::p1")
	if len(matches) != 1 {
		t.Fatalf("test::p1: %d matching symbols, want 1", len(matches))
	}
	inst, err := ctx.Instantiate(matches[0])
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	fv, err := inst.GetFeatureValue(ctx, "suboccurrences")
	if err != nil {
		t.Fatalf("GetFeatureValue(suboccurrences): %v", err)
	}
	if got := len(elementsOf(fv.HeldValue())); got != 2 {
		t.Fatalf("suboccurrences: %d elements, want 2", got)
	}
	for _, tc := range []struct{ name, feature string }{
		{"non_collection_member", "superoccurrence"},
		{"frame_root", "timeSlices"},
		{"frame_root_subsetter", "snapshots"},
		{"frame_root_superset", "portions"},
		{"inverse_member", "successors"},
		{"unioning_member", "withoutOccurrences"},
		{"at_least_one_member", "timeEnclosedOccurrences"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := inst.GetFeatureValue(ctx, tc.feature); !errors.Is(err, ErrNoSuchFeature) {
				t.Errorf("GetFeatureValue(%s) = %v, want %v", tc.feature, err, ErrNoSuchFeature)
			}
			if err := inst.SetFeatureValue(ctx, tc.feature, sequenceOf(nil)); !errors.Is(err, ErrNoSuchFeature) {
				t.Errorf("SetFeatureValue(%s) = %v, want %v", tc.feature, err, ErrNoSuchFeature)
			}
		})
	}
	t.Run("derived_collection_is_read_only", func(t *testing.T) {
		if err := inst.SetFeatureValue(ctx, "suboccurrences", sequenceOf(nil)); !errors.Is(err, ErrNoSuchFeature) {
			t.Errorf("SetFeatureValue(suboccurrences) = %v, want %v", err, ErrNoSuchFeature)
		}
	})
}
