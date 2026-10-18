package runtime

import (
	"errors"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const shapeKindsRobustnessModel = `
	package test {
		private import ScalarValues::*;
		part def Sub;
		case def Inspection { subject s : Sub; }
		enum def Mode { on; off; }
		view def Overview { attribute depth : Integer = 1; assert constraint shallow { depth < 1 } }
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
// literal and a member no view declares are typed errors, never panics; a failing
// assertion inside a held view is reported, not skipped, and `all` reaches the view.
func TestRuntimeRobustnessShapeKinds(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, shapeKindsRobustnessModel))
	root := idx.DocumentRoot("<test>")
	t.Run("nested_view_assertion_visited", func(t *testing.T) {
		inst := instantiateQualified(t, ctx, idx, "test::Vehicle")
		report, err := ctx.ValidateObject(inst, []*symbols.Scope{root})
		if err != nil {
			t.Fatalf("validate: %v", err)
		}
		if !report.Complete() || report.Count(ValidationViolated) != 1 || report.Valid() {
			t.Fatalf("validation reached the view's assertion: violated=%d complete=%v valid=%v, want 1 true false",
				report.Count(ValidationViolated), report.Complete(), report.Valid())
		}
	})
	t.Run("extent_reaches_held_view", func(t *testing.T) {
		views, err := evalIn(t, ctx, root, "SequenceFunctions::size(all test::Overview)")
		if err != nil {
			t.Fatalf("size(all Overview): %v", err)
		}
		vehicles, err := evalIn(t, ctx, root, "SequenceFunctions::size(all test::Vehicle)")
		if err != nil || FormatValue(views) != FormatValue(vehicles) || FormatValue(views) == "0" {
			t.Fatalf("size(all Overview) = %s, size(all Vehicle) = %s, %v; want one view per vehicle", FormatValue(views), FormatValue(vehicles), err)
		}
	})
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
