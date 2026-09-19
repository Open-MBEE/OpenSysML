package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// realValue reports the real a feature value holds.
func realValue(t *testing.T, v Value) float64 {
	t.Helper()
	if v.Kind != ValConst || v.Const.Kind != semantics.ValReal {
		t.Fatalf("value is %v, want a real", v)
	}
	return v.Const.Real
}

// A usage's `that` names the object featuring the usage's values, so a chain
// from it reads that object's feature values ([KerML, 8.4.2]).
func TestThatReadsTheFeaturingObject(t *testing.T) {
	const src = `
	package test {
		private import ScalarValues::Real;
		part def P { attribute a : Real = 1.5; }
		part def Holder { part p : P { attribute b : Real = that.a; } }
	}`
	inst, ctx := instantiatePart(t, "Holder", src)
	p := fvInstance(t, ctx, inst, "p")
	fv, err := p.GetFeatureValue(ctx, "b")
	if err != nil {
		t.Fatalf("GetFeatureValue b: %v", err)
	}
	if got := realValue(t, fv.HeldValue()); got != 1.5 {
		t.Errorf("b = %v, want the featuring object's a (1.5)", got)
	}
}

// The innermost usage features the value, so `that` in a nested usage's body
// reads that usage's object rather than an outer one.
func TestThatReadsTheInnermostFeaturingObject(t *testing.T) {
	const src = `
	package test {
		private import ScalarValues::Real;
		part def Inner { attribute a : Real = 2.0; }
		part def Outer { attribute a : Real = 9.0; part i : Inner { attribute b : Real = that.a; } }
		part def Holder { part o : Outer; }
	}`
	inst, ctx := instantiatePart(t, "Holder", src)
	i := fvInstance(t, ctx, inst, "o", "i")
	fv, err := i.GetFeatureValue(ctx, "b")
	if err != nil {
		t.Fatalf("GetFeatureValue b: %v", err)
	}
	if got := realValue(t, fv.HeldValue()); got != 2.0 {
		t.Errorf("b = %v, want the inner object's a (2.0)", got)
	}
}

// `self` names the object itself ([KerML] Base::Anything::self), as the library's
// `ScalarMeasurementReference::mRefs = self` relies on.
func TestSelfReadsTheObjectItself(t *testing.T) {
	const src = `
	package test {
		private import Base::Anything;
		part def P { ref me : Anything = self; }
		part holder { part p : P; }
	}`
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
	inst := instantiateNamed(t, ctx, idx, "test::holder")
	p := fvInstance(t, ctx, inst, "p")
	fv, err := p.GetFeatureValue(ctx, "me")
	if err != nil {
		t.Fatalf("GetFeatureValue me: %v", err)
	}
	if got := fv.HeldValue(); got.Kind != ValInstance || got.Instance != p.ID {
		t.Errorf("me = %v, want the object p itself (instance %d)", got, p.ID)
	}
}
