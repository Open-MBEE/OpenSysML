package runtime

import (
	"errors"
	"testing"
)

const libraryFrameRobustnessModel = `
package test {
	private import ISQ::*;
	private import SI::*;
	private import MeasurementReferences::MeasurementUnit;
	attribute bare : CartesianSpatial3dCoordinateFrame;
	attribute two : Position3dVector = (1.0, 2.0) [universalCartesianSpatial3dCoordinateFrame];
	attribute fourth : MeasurementUnit = universalCartesianSpatial3dCoordinateFrame.mRefs#(4);
}
`

// TestRuntimeRobustnessLibraryFrame pins what a library-declared frame refuses with
// a typed error — a vector of the wrong dimension over it, an axis it lacks — and
// that a frame-typed usage with no body of its own stays open rather than borrowing one.
func TestRuntimeRobustnessLibraryFrame(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, libraryFrameRobustnessModel))
	root := idx.DocumentRoot("<test>")
	for _, tc := range []struct {
		name, expr string
		want       error
	}{
		{"two_elements_over_a_three_axis_frame", "test::two", ErrMultiplicityViolation},
		{"axis_the_frame_lacks", "test::fourth", ErrIndexOutOfRange},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := evalIn(t, ctx, root, tc.expr)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s: %v, want %v", tc.expr, err, tc.want)
			}
		})
	}
	t.Run("frame_typed_usage_without_a_body_is_open", func(t *testing.T) {
		val, err := evalIn(t, ctx, root, "test::bare")
		if err != nil || val.Kind != ValUndetermined {
			t.Fatalf("test::bare = %v, %v; want an undetermined value", val, err)
		}
	})
}
