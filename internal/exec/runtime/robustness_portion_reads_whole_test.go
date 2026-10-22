package runtime

import (
	"errors"
	"math/big"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// A portion (snapshot, timeslice) reads the whole's value of a feature its own body leaves unvalued:
// the whole's typed error is reported through the portion; a portion of another type reads nothing.
func TestRuntimeRobustnessPortionReadsWhole(t *testing.T) {
	t.Run("whole_feature_error_is_reported_through_the_portion", func(t *testing.T) {
		inst, ctx := instantiatePart(t, "Sys", `
			package test {
				part def Panel {
					attribute width : Integer;
					attribute area : Integer = width * 2;
				}
				part def Sys {
					part panel : Panel {
						attribute :>> width = 1 / 0;
						snapshot shot : Panel;
					}
				}
			}`)
		panel, err := inst.GetFeatureValue(ctx, "panel")
		if err != nil {
			t.Fatalf("GetFeatureValue(panel) = %v", err)
		}
		obj := ctx.instances[panel.HeldValue().Instance]
		shot, err := obj.GetFeatureValue(ctx, "shot")
		if err != nil {
			t.Fatalf("GetFeatureValue(shot) = %v", err)
		}
		_, err = ctx.instances[shot.HeldValue().Instance].GetFeatureValue(ctx, "width")
		if !errors.Is(err, ErrDivisionByZero) {
			t.Fatalf("shot.width = %v, want the whole's ErrDivisionByZero", err)
		}
	})

	t.Run("portion_of_another_type_reads_its_own_features", func(t *testing.T) {
		inst, ctx := instantiatePart(t, "Sys", `
			package test {
				part def Panel { attribute width : Integer; }
				part def Other { attribute width : Integer; }
				part def Sys {
					part panel : Panel {
						attribute :>> width = 3;
						snapshot other : Other;
					}
				}
			}`)
		panel, err := inst.GetFeatureValue(ctx, "panel")
		if err != nil {
			t.Fatalf("GetFeatureValue(panel) = %v", err)
		}
		other, err := ctx.instances[panel.HeldValue().Instance].GetFeatureValue(ctx, "other")
		if err != nil {
			t.Fatalf("GetFeatureValue(other) = %v", err)
		}
		fv, err := ctx.instances[other.HeldValue().Instance].GetFeatureValue(ctx, "width")
		if err != nil {
			t.Fatalf("other.width = %v", err)
		}
		if fv.HeldValue().Kind == ValConst {
			t.Errorf("other.width = %s, want no value read from an unrelated whole", FormatValue(fv.HeldValue()))
		}
	})

	t.Run("write_to_the_whole_is_seen_by_the_portion", func(t *testing.T) {
		inst, ctx := instantiatePart(t, "Sys", `
			package test {
				part def Panel { attribute width : Integer; }
				part def Sys {
					part panel : Panel {
						attribute :>> width = 3;
						snapshot shot : Panel;
					}
				}
			}`)
		panel, err := inst.GetFeatureValue(ctx, "panel")
		if err != nil {
			t.Fatalf("GetFeatureValue(panel) = %v", err)
		}
		whole := ctx.instances[panel.HeldValue().Instance]
		shotFV, err := whole.GetFeatureValue(ctx, "shot")
		if err != nil {
			t.Fatalf("GetFeatureValue(shot) = %v", err)
		}
		shot := ctx.instances[shotFV.HeldValue().Instance]
		if fv, err := shot.GetFeatureValue(ctx, "width"); err != nil || FormatValue(fv.HeldValue()) != "3" {
			t.Fatalf("shot.width = %v, %v, want 3", fv, err)
		}
		if err := whole.SetFeatureValue(ctx, "width", Value{Kind: ValConst, Const: semantics.BigIntValue(big.NewInt(8))}); err != nil {
			t.Fatalf("SetFeatureValue(width) = %v", err)
		}
		if fv, err := shot.GetFeatureValue(ctx, "width"); err != nil || FormatValue(fv.HeldValue()) != "8" {
			t.Fatalf("shot.width after the write = %v, %v, want 8", fv, err)
		}
	})
}
