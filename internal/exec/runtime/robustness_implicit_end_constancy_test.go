package runtime

import (
	"errors"
	"strings"
	"testing"
)

const implicitEndConstancyFixture = `package test {
	part def Thing;
	connection def Link {
		end part src : Thing;
		end part dst : Thing;
	}
	part def Sys {
		part a : Thing;
		part b : Thing;
		connection link : Link connect src ::> a to dst ::> b;
		action def SetSrc { first w; action w { assign link.src := b; } }
		action setSrc : SetSrc;
	}
	part sys : Sys;
}`

// An end feature that may vary in time is implicitly constant (SysML 2.0
// §8.4.2.2): a behavior's write is refused like a declared constant's, while
// the binding that gives the end its value stays allowed.
func TestRuntimeRobustnessImplicitEndConstancy(t *testing.T) {
	setup := func(t *testing.T) (*Context, *Instance, *Instance, int64, int64, func() error) {
		t.Helper()
		idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, implicitEndConstancyFixture))
		self, err := ctx.Instantiate(oneSymbol(t, idx, "test::sys"))
		if err != nil {
			t.Fatalf("instantiate sys: %v", err)
		}
		fv, err := self.GetFeatureValue(ctx, "link")
		if err != nil {
			t.Fatalf("read link: %v", err)
		}
		linkID, ok := fv.HeldValue().Object()
		if !ok {
			t.Fatalf("link holds %v, want an object", fv.HeldValue())
		}
		link, ok := ctx.Instance(linkID)
		if !ok {
			t.Fatalf("link names object #%d, which the context does not hold", linkID)
		}
		av, err := self.GetFeatureValue(ctx, "a")
		if err != nil {
			t.Fatalf("read a: %v", err)
		}
		aID, ok := av.HeldValue().Object()
		if !ok {
			t.Fatalf("a holds %v, want an object", av.HeldValue())
		}
		bv, err := self.GetFeatureValue(ctx, "b")
		if err != nil {
			t.Fatalf("read b: %v", err)
		}
		bID, ok := bv.HeldValue().Object()
		if !ok {
			t.Fatalf("b holds %v, want an object", bv.HeldValue())
		}
		run := func() error {
			_, err := ctx.ExecuteActionPerformedBy(oneSymbol(t, idx, "test::Sys::setSrc"), self, nil)
			return err
		}
		return ctx, self, link, aID, bID, run
	}
	srcHolds := func(t *testing.T, ctx *Context, link *Instance, want int64) {
		t.Helper()
		fv, err := link.GetFeatureValue(ctx, "src")
		if err != nil {
			t.Fatalf("read link.src: %v", err)
		}
		got, ok := fv.HeldValue().Object()
		if !ok || got != want {
			t.Fatalf("link.src holds %v (object %t), want object #%d", fv.HeldValue(), ok, want)
		}
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

	t.Run("assigning_an_end_is_refused_and_leaves_its_value", func(t *testing.T) {
		ctx, _, link, aID, _, run := setup(t)
		requireRefused(t, run(), "src is constant as an end feature")
		srcHolds(t, ctx, link, aID)
	})
	t.Run("set_feature_value_on_an_end_is_refused", func(t *testing.T) {
		ctx, _, link, aID, bID, _ := setup(t)
		requireRefused(t, link.SetFeatureValue(ctx, "src", Value{Kind: ValInstance, Instance: bID}), "src")
		srcHolds(t, ctx, link, aID)
	})
	t.Run("binding_an_end_is_allowed", func(t *testing.T) {
		ctx, _, link, aID, bID, _ := setup(t)
		srcHolds(t, ctx, link, aID)
		if err := link.BindFeatureValue(ctx, "src", Value{Kind: ValInstance, Instance: bID}); err != nil {
			t.Fatalf("binding the end's value: %v", err)
		}
		srcHolds(t, ctx, link, bID)
	})
}
