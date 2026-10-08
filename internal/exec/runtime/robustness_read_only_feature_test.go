package runtime

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

func readOnlyInt(n int64) Value {
	return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: n}}
}

const readOnlyFeatureFixture = `package test {
	private import ScalarValues::*;
	part def Sensor {
		constant attribute id : Integer = 1;
		attribute raw : Integer default 4;
		derived attribute scaled : Integer = raw * 10;
	}
	part def Tagged :> Sensor {
		attribute :>> id = 1;
	}
	part def Probe {
		constant attribute limit : Integer default 5;
		derived attribute twice : Integer = count * 2;
		attribute count : Integer default 3;
		part tagged : Tagged;
		action def SetLimit { first w; action w { assign limit := 7; } }
		action def SetTwice { first w; action w { assign twice := 7; } }
		action def SetCount { first w; action w { assign count := 4; } }
		action def SetTaggedId { first w; action w { assign tagged.id := 2; } }
		action def SetTaggedScaled { first w; action w { assign tagged.scaled := 2; } }
		action def Raise { out limit : Integer; first w; action w { assign limit := 7; } }
		action def ReturnLimit { constant attribute limit : Integer = 5; first r; action r : Raise; }
		action setLimit : SetLimit;
		action setTwice : SetTwice;
		action setCount : SetCount;
		action setTaggedId : SetTaggedId;
		action setTaggedScaled : SetTaggedScaled;
		action returnLimit : ReturnLimit;
	}
	part probe : Probe;
}`

func TestRuntimeRobustnessReadOnlyFeatureWrites(t *testing.T) {
	setup := func(t *testing.T) (*Context, *Instance, func(string) error) {
		t.Helper()
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, readOnlyFeatureFixture))
		self, err := ctx.Instantiate(oneSymbol(t, idx, "test::probe"))
		if err != nil {
			t.Fatalf("instantiate probe (a constant initialized by its default): %v", err)
		}
		run := func(action string) error {
			_, err := ctx.ExecuteActionPerformedBy(oneSymbol(t, idx, "test::Probe::"+action), self, nil)
			return err
		}
		return ctx, self, run
	}
	held := func(t *testing.T, ctx *Context, inst *Instance, name string) int64 {
		t.Helper()
		fv, err := inst.GetFeatureValue(ctx, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		v := fv.HeldValue()
		if v.Kind != ValConst || v.Const.Kind != semantics.ValInt {
			t.Fatalf("%s holds %v, want an Integer", name, v)
		}
		return v.Const.Int
	}
	tagged := func(t *testing.T, ctx *Context, self *Instance) *Instance {
		t.Helper()
		fv, err := self.GetFeatureValue(ctx, "tagged")
		if err != nil {
			t.Fatalf("read tagged: %v", err)
		}
		id, ok := fv.HeldValue().Object()
		if !ok {
			t.Fatalf("tagged holds %v, want an object", fv.HeldValue())
		}
		obj, ok := ctx.Instance(id)
		if !ok {
			t.Fatalf("tagged names object #%d, which the context does not hold", id)
		}
		return obj
	}
	requireRefused := func(t *testing.T, err error, wants ...string) {
		t.Helper()
		if !errors.Is(err, ErrReadOnlyFeature) {
			t.Fatalf("err = %v, want ErrReadOnlyFeature", err)
		}
		for _, want := range wants {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("err = %q, want it to name %q", err, want)
			}
		}
	}

	t.Run("assigning_a_constant_leaves_its_value", func(t *testing.T) {
		ctx, self, run := setup(t)
		requireRefused(t, run("setLimit"), "limit is constant")
		if got := held(t, ctx, self, "limit"); got != 5 {
			t.Errorf("limit = %d after the refused write, want its default 5", got)
		}
	})
	t.Run("assigning_a_derived_feature_leaves_its_value", func(t *testing.T) {
		ctx, self, run := setup(t)
		requireRefused(t, run("setTwice"), "twice is derived")
		if got := held(t, ctx, self, "twice"); got != 6 {
			t.Errorf("twice = %d after the refused write, want the bound 6", got)
		}
	})
	t.Run("derived_feature_is_reevaluated_after_its_source_is_written", func(t *testing.T) {
		ctx, self, run := setup(t)
		if err := run("setCount"); err != nil {
			t.Fatalf("writing a writable feature: %v", err)
		}
		if got := held(t, ctx, self, "twice"); got != 8 {
			t.Errorf("twice = %d after count := 4, want 8", got)
		}
	})
	t.Run("chained_write_to_an_inherited_constant_is_refused", func(t *testing.T) {
		ctx, self, run := setup(t)
		requireRefused(t, run("setTaggedId"), "id is constant by Sensor::id")
		if got := held(t, ctx, tagged(t, ctx, self), "id"); got != 1 {
			t.Errorf("tagged.id = %d after the refused write, want 1", got)
		}
	})
	t.Run("chained_write_to_a_derived_feature_is_refused", func(t *testing.T) {
		ctx, self, run := setup(t)
		requireRefused(t, run("setTaggedScaled"), "scaled is derived")
		if got := held(t, ctx, tagged(t, ctx, self), "scaled"); got != 40 {
			t.Errorf("tagged.scaled = %d after the refused write, want 40", got)
		}
	})
	t.Run("output_returned_into_a_constant_is_refused", func(t *testing.T) {
		_, _, run := setup(t)
		requireRefused(t, run("returnLimit"), "output limit returned", "limit is constant")
	})
	t.Run("set_feature_value_refuses_a_read_only_feature", func(t *testing.T) {
		ctx, self, _ := setup(t)
		for _, name := range []string{"limit", "twice"} {
			requireRefused(t, self.SetFeatureValue(ctx, name, readOnlyInt(9)), name)
		}
		if got := held(t, ctx, self, "limit"); got != 5 {
			t.Errorf("limit = %d, want 5", got)
		}
		if got := held(t, ctx, self, "twice"); got != 6 {
			t.Errorf("twice = %d, want 6", got)
		}
		if err := self.SetFeatureValue(ctx, "count", readOnlyInt(9)); err != nil {
			t.Fatalf("writing a writable feature: %v", err)
		}
	})
}
