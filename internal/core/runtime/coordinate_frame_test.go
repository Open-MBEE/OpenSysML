package runtime

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const frameKindSrc = `
package Demo {
	private import ISQ::*;
	private import ISQSpaceTime::*;
	private import SI::*;
	private import MeasurementReferences::*;

	attribute datum : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); }
	attribute lifted : CartesianSpatial3dCoordinateFrame {
		:>> mRefs = (mm, mm, mm);
		:>> transformation : CoordinateFramePlacement { :>> source = datum; :>> origin = (0.0, 0.0, 5.0) [datum]; }
	}
	attribute placed : CoordinateFramePlacement { :>> source = datum; :>> target = lifted; :>> origin = (0.0, 0.0, 5.0) [datum]; }
	attribute alsoPlaced : CoordinateFramePlacement { :>> source = datum; :>> target = lifted; :>> origin = (0.0, 0.0, 5.0) [datum]; }
	attribute trs : TranslationRotationSequence { :>> source = datum; :>> target = lifted; :>> elements = new Translation((0.0, 0.0, 5.0) [datum]); }
	attribute alsoTrs : TranslationRotationSequence { :>> source = datum; :>> target = lifted; :>> elements = new Translation((0.0, 0.0, 5.0) [datum]); }
	attribute turned : TranslationRotationSequence { :>> source = datum; :>> target = lifted; :>> elements = new Rotation((0.0, 0.0, 1.0) [datum], 90 ['°']); }
	attribute alsoTurned : TranslationRotationSequence { :>> source = datum; :>> target = lifted; :>> elements = new Rotation((0.0, 0.0, 1.0) [datum], 1.5707961 [rad]); }
	attribute def Bespoke :> CoordinateTransformation;
	attribute bespoke : Bespoke { :>> source = datum; :>> target = lifted; }
	attribute alsoBespoke : Bespoke { :>> source = datum; :>> target = lifted; }
}`

func frameKindContext(t *testing.T) (*Context, *symbols.Scope) {
	t.Helper()
	ctx := libraryContextOver(t, frameKindSrc)
	return ctx, lookupOne(t, ctx.model.resolver.Index(), "Demo").Scope
}

// TestCoordinateFrameDescribed: the kinds describe, render and trace themselves,
// a scale as the measurement scale it is, and survive a nil payload.
func TestCoordinateFrameDescribed(t *testing.T) {
	ctx, scope := frameKindContext(t)
	eval := func(src string) Value {
		t.Helper()
		val, err := evalIn(t, ctx, scope, src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		return val
	}
	cases := []struct {
		expr, operand, value, text string
	}{
		{"datum", "a coordinate frame", "coordinate frame", "datum [mm, mm, mm]"},
		{"datum / s", "a coordinate frame", "coordinate frame", "datum / s [mm/s, mm/s, mm/s]"},
		{"SI::'°C_abs'", "a measurement scale", "coordinate frame", "'°C_abs' ['°C']"},
		{"Time::UTC", "a measurement scale", "coordinate frame", "UTC [s]"},
		{"lifted.transformation", "a coordinate transformation", "coordinate transformation", "transformation (datum → lifted)"},
	}
	for _, tc := range cases {
		val := eval(tc.expr)
		if got := describeOperand(val); got != tc.operand {
			t.Errorf("describeOperand(%s) = %q, want %q", tc.expr, got, tc.operand)
		}
		if got := describeValue(val); got != tc.value {
			t.Errorf("describeValue(%s) = %q, want %q", tc.expr, got, tc.value)
		}
		if got := FormatValue(val); got != tc.text {
			t.Errorf("FormatValue(%s) = %q, want %q", tc.expr, got, tc.text)
		}
		if got := FormatTraceValue(val); got != tc.text {
			t.Errorf("FormatTraceValue(%s) = %q, want %q", tc.expr, got, tc.text)
		}
	}
	if got := FormatTraceValue(Value{Kind: ValCoordinateFrame}); got != "coordinate frame" {
		t.Errorf("a frame with no payload traces as %q", got)
	}
	if got := FormatTraceValue(Value{Kind: ValCoordinateTransformation}); got != "coordinate transformation" {
		t.Errorf("a transformation with no payload traces as %q", got)
	}
	var frame *CoordinateFrame
	var transformation *CoordinateTransformation
	if frame.String() != "<unknown>" || frame.Name() != "<unknown>" || transformation.String() != "<unknown>" {
		t.Error("nil payloads render as <unknown>")
	}
	if !frame.equal(nil) || frame.equal(eval("datum").CoordinateFrame()) || !transformation.equal(nil) {
		t.Error("nil payload equality is identity")
	}
}

// TestCoordinateFrameHashesAsItCompares: a set holds a frame once however it was
// reached, and a composed frame once per distinct axes.
func TestCoordinateFrameHashesAsItCompares(t *testing.T) {
	ctx, scope := frameKindContext(t)
	set := NewSet()
	for _, src := range []string{"datum", "lifted.transformation.source", "datum / s", "datum / SI::s", "datum * s", "lifted", "lifted.transformation", "SI::'°C_abs'", "SI::'°C_abs'"} {
		val, err := evalIn(t, ctx, scope, src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		set.Add(val)
	}
	if set.Size() != 6 {
		t.Errorf("the set holds %d values, want 6: %s", set.Size(), FormatValue(NewSetValue(set)))
	}
	for _, src := range []string{"datum / s", "lifted.transformation", "Time::UTC"} {
		val, err := evalIn(t, ctx, scope, src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if set.Contains(val) != (src != "Time::UTC") {
			t.Errorf("set.Contains(%s) = %v", src, set.Contains(val))
		}
	}
}

// TestCoordinateTransformationHashesAsItCompares: transformations of one shape
// and content are one value whichever object states them, a shapeless one is its
// object, and a snapshot taken before a write is not the one taken after.
func TestCoordinateTransformationHashesAsItCompares(t *testing.T) {
	ctx, scope := frameKindContext(t)
	eval := func(src string) Value {
		t.Helper()
		val, err := evalIn(t, ctx, scope, src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		return val
	}
	sameAndHashed := func(a, b Value, want bool) {
		t.Helper()
		if got := valueEqual(a, b); got != want {
			t.Errorf("%s == %s is %v, want %v", FormatValue(a), FormatValue(b), got, want)
		}
		set := NewSet()
		set.Add(a)
		set.Add(b)
		if (set.Size() == 1) != want {
			t.Errorf("a set of %s and %s holds %d values, want one: %v", FormatValue(a), FormatValue(b), set.Size(), want)
		}
		if !set.Contains(b) {
			t.Errorf("a set holding %s does not contain it", FormatValue(b))
		}
	}
	sameAndHashed(eval("lifted.transformation"), eval("placed"), true)
	sameAndHashed(eval("placed"), eval("alsoPlaced"), true)
	sameAndHashed(eval("trs"), eval("alsoTrs"), true)
	sameAndHashed(eval("trs"), eval("lifted.transformation"), false)
	sameAndHashed(eval("90 ['°']"), eval("1.5707961 [rad]"), true)
	sameAndHashed(eval("turned"), eval("alsoTurned"), true)
	sameAndHashed(eval("turned"), eval("trs"), false)
	sameAndHashed(eval("bespoke"), eval("bespoke"), true)
	sameAndHashed(eval("bespoke"), eval("alsoBespoke"), false)

	before := eval("lifted.transformation")
	placement := ctx.instances[before.CoordinateTransformation().Object]
	if placement == nil {
		t.Fatal("lifted.transformation is held by no object")
	}
	if err := placement.SetFeatureValue(ctx, "origin", eval("(0.0, 0.0, 6.0) [datum]")); err != nil {
		t.Fatalf("SetFeatureValue(origin): %v", err)
	}
	after := eval("lifted.transformation")
	if before.CoordinateTransformation().Object != after.CoordinateTransformation().Object {
		t.Fatal("the write moved the transformation to another object")
	}
	sameAndHashed(before, after, false)
	sameAndHashed(after, eval("placed"), false)
	if got := FormatValue(eval("lifted.transformation.origin")); got != "⟨0.0, 0.0, 6.0⟩ [datum]" {
		t.Errorf("lifted.transformation.origin = %s after the write", got)
	}
}
