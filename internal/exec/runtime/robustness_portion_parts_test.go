package runtime

import (
	"errors"
	"slices"
	"testing"
)

// A portion (snapshot, timeslice) reads the whole's parts as its own without the whole's portions, so
// it never holds itself; the lifetime guard that refuses a portion holding its whole stays in force.
func TestRuntimeRobustnessPortionParts(t *testing.T) {
	instantiateSys := func(t *testing.T, src string) (*Context, *Instance) {
		t.Helper()
		ctx, inst, err := instantiateWithLibraries(t, src, "test::Sys")
		if err != nil {
			t.Fatalf("Instantiate(test::Sys) = %v", err)
		}
		return ctx, inst
	}
	heldObject := func(t *testing.T, ctx *Context, inst *Instance, name string) *Instance {
		t.Helper()
		fv, err := inst.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("GetFeatureValue(%s) = %v", name, err)
		}
		obj, ok := ctx.instances[fv.HeldValue().Instance]
		if !ok {
			t.Fatalf("%s holds no object: %v", name, fv.HeldValue())
		}
		return obj
	}

	t.Run("portion_reads_the_wholes_parts_without_itself", func(t *testing.T) {
		ctx, inst := instantiateSys(t, `
			package test {
				part def Node;
				part def Craft {
					part hull : Node;
					timeslice part cruise;
					snapshot part launch;
				}
				part def Sys { part craft : Craft; }
			}`)
		craft := heldObject(t, ctx, inst, "craft")
		hull := heldObject(t, ctx, craft, "hull")
		cruise := heldObject(t, ctx, craft, "cruise")
		for _, name := range []string{"subparts", "subitems", "suboccurrences"} {
			fv, err := cruise.GetFeatureValue(ctx, name)
			if err != nil {
				t.Fatalf("cruise.%s = %v, want the whole's parts without the whole's portions", name, err)
			}
			elements := elementsOf(fv.HeldValue())
			if len(elements) != 1 || elements[0].Instance != hull.ID {
				t.Fatalf("cruise.%s = %v, want [hull #%d]", name, elements, hull.ID)
			}
		}
	})

	t.Run("portion_typed_by_the_whole_gets_portions_of_its_own", func(t *testing.T) {
		ctx, inst := instantiateSys(t, `
			package test {
				part def Craft {
					snapshot part launch : Craft;
				}
				part def Sys { part craft : Craft; }
			}`)
		craft := heldObject(t, ctx, inst, "craft")
		launch := heldObject(t, ctx, craft, "launch")
		own := heldObject(t, ctx, launch, "launch")
		if own == launch || own == craft {
			t.Fatalf("launch.launch = #%d, want a portion of launch #%d, not launch or craft #%d", own.ID, launch.ID, craft.ID)
		}
		fv, err := launch.GetFeatureValue(ctx, "subparts")
		if err != nil {
			t.Fatalf("launch.subparts = %v", err)
		}
		if elements := elementsOf(fv.HeldValue()); len(elements) != 1 || elements[0].Instance != own.ID {
			t.Fatalf("launch.subparts = %v, want [launch.launch #%d]", elements, own.ID)
		}
	})

	t.Run("portions_still_to_be_made_are_not_the_portions_parts", func(t *testing.T) {
		ctx, inst := instantiateSys(t, `
			package test {
				part def Craft {
					timeslice part shots : Craft[3];
				}
				part def Sys { part craft : Craft; }
			}`)
		craft := heldObject(t, ctx, inst, "craft")
		shots, err := craft.GetFeatureValue(ctx, "shots")
		if err != nil {
			t.Fatalf("craft.shots = %v", err)
		}
		first, ok := ctx.instances[listedElements(shots.HeldValue())[0].Instance]
		if !ok {
			t.Fatalf("craft.shots holds no first object: %v", shots.HeldValue())
		}
		own, err := first.GetFeatureValue(ctx, "shots")
		if err != nil {
			t.Fatalf("shots#(1).shots = %v, want three portions of its own", err)
		}
		if got := standingElements(own.HeldValue()); len(got) != 3 || slices.ContainsFunc(got, func(v Value) bool { return v.Instance == first.ID }) {
			t.Fatalf("shots#(1).shots = %v, want three portions of #%d, none of them itself", got, first.ID)
		}
		if sub, err := first.GetFeatureValue(ctx, "subparts"); err != nil || len(standingElements(sub.HeldValue())) != 3 {
			t.Fatalf("shots#(1).subparts = %v, %v, want its own three shots", sub, err)
		}
	})

	t.Run("a_portion_holding_its_whole_is_still_refused", func(t *testing.T) {
		ctx, inst := instantiateSys(t, `
			package test {
				part def Craft {
					timeslice part cruise : Craft;
					part other : Craft[0..1];
				}
				part def Sys { part craft : Craft; }
			}`)
		craft := heldObject(t, ctx, inst, "craft")
		cruise := heldObject(t, ctx, craft, "cruise")
		err := cruise.SetFeatureValue(ctx, "other", Value{Kind: ValInstance, Instance: craft.ID})
		if !errors.Is(err, errOccurrenceLifetime) {
			t.Fatalf("cruise.other := craft = %v, want errOccurrenceLifetime", err)
		}
	})
}
