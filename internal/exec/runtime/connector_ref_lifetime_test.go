package runtime

import "testing"

// A `ref connection` takes the nested redefinition written beside it and outlives the
// part holding it; a composite one ends with that part (SysML v2 §7.6.4, §7.13.2).
func TestConnectorObjectEndsWithItsHolderOnlyWhenComposite(t *testing.T) {
	for _, tc := range []struct {
		keyword string
		ends    bool
	}{
		{keyword: "ref connection", ends: false},
		{keyword: "connection", ends: true},
	} {
		t.Run(tc.keyword, func(t *testing.T) {
			holder, ctx := instantiatePartWithLibraries(t, "Holder", `
				package test {
					private import ScalarValues::Integer;
					part def Module;
					connection def Installation {
						end slot : Slot;
						end module : Module;
						attribute rating : Integer default = 1;
					}
					part def Slot {
						ref installedModule : Module [0..1];
						`+tc.keyword+` installation : Installation [0..1] connect self to installedModule;
					}
					part def Holder {
						part module1 : Module;
						part slot1 : Slot {
							:>> installedModule = module1;
							`+tc.keyword+` :>> installation [1];
							:>> installation.rating = 7;
						}
					}
				}
			`)
			slot := soleHeldObject(t, ctx, holder, "slot1")
			installation := soleHeldObject(t, ctx, slot, "installation")
			rating, err := installation.GetFeatureValue(ctx, "rating")
			if err != nil || FormatValue(rating.HeldValue()) != "7" {
				t.Fatalf("installation.rating = %s, %v; want the nested redefinition's 7", FormatValue(rating.HeldValue()), err)
			}
			if err := ctx.destroy(slot); err != nil {
				t.Fatalf("destroy(slot1): %v", err)
			}
			if _, ended := ctx.Destroyed(installation); ended != tc.ends {
				t.Fatalf("installation destroyed with slot1 = %v, want %v", ended, tc.ends)
			}
		})
	}
}

func soleHeldObject(t *testing.T, ctx *Context, inst *Instance, feature string) *Instance {
	t.Helper()
	fv, err := inst.GetFeatureValue(ctx, feature)
	if err != nil {
		t.Fatalf("GetFeatureValue(%s): %v", feature, err)
	}
	id, ok := fv.HeldValue().Object()
	if !ok {
		t.Fatalf("%s = %s, want one object", feature, FormatValue(fv.HeldValue()))
	}
	held, ok := ctx.Instance(id)
	if !ok {
		t.Fatalf("%s holds object #%d, which the context does not know", feature, id)
	}
	return held
}
