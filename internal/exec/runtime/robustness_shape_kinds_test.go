package runtime

import (
	"errors"
	"testing"
)

const shapeKindsRobustnessModel = `
	package test {
		part def Sub;
		case def Inspection { subject s : Sub; }
		enum def Mode { on; off; }
		view def Overview;
		part def Vehicle {
			case inspection : Inspection;
			enum mode : Mode = Mode::on;
			view overview : Overview;
		}
		part v : Vehicle;
	}
`

// TestRuntimeRobustnessShapeKinds pins the failure modes of the usage kinds the
// shape admits: a member none of them declares, a chain through an enumeration
// literal and a member no view declares are typed errors, never panics.
func TestRuntimeRobustnessShapeKinds(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, shapeKindsRobustnessModel))
	root := idx.DocumentRoot("<test>")
	for _, tc := range []struct {
		name, expr string
		want       error
	}{
		{"case_member_not_declared", "test::v.inspection.nosuch", ErrNoSuchFeature},
		{"view_member_not_declared", "test::v.overview.nosuch", ErrNoSuchFeature},
		{"chain_through_enum_literal", "test::v.mode.nosuch", ErrNoSuchFeature},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := evalIn(t, ctx, root, tc.expr)
			if !errors.Is(err, tc.want) {
				t.Fatalf("%s: %v, want %v", tc.expr, err, tc.want)
			}
		})
	}
}
