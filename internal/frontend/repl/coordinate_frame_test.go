package repl

import (
	"strings"
	"testing"
)

// coordinateFrameSession declares the Annex A frames, a vector over one, a placed
// frame with a translation-rotation sequence, and a part holding a composed frame.
func coordinateFrameSession(t *testing.T) *Session {
	t.Helper()
	s := NewSession()
	res := s.Submit(`package Frames {
		private import ISQ::*;
		private import ISQSpaceTime::*;
		private import SI::*;
		private import Time::*;
		private import MeasurementReferences::*;
		private import SpatialItems::*;
		private import QuantityCalculations::*;
		private import VectorCalculations::*;
		attribute spatialCF : CartesianSpatial3dCoordinateFrame[1] { :>> mRefs = (m, m, m); }
		attribute velocityCF : CartesianVelocity3dCoordinateFrame[1] = spatialCF / s;
		attribute accelerationCF : CartesianAcceleration3dCoordinateFrame[1] = velocityCF / s;
		attribute p : Position3dVector = (1.0, 2.0, 3.0) [spatialCF];
		attribute datum : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); }
		attribute lbcf : CartesianSpatial3dCoordinateFrame {
			:>> mRefs = (mm, mm, mm);
			:>> transformation : TranslationRotationSequence {
				:>> source = datum;
				:>> elements = (new Translation((10.0, 0.0, 0.0) [datum]), new Rotation((0.0, 0.0, 1.0) [datum], 180 ['°']));
			}
		}
		part def Vehicle { attribute body : CoordinateFrame; attribute clock : TimeScale; }
		part vehicle : Vehicle { :>> body = spatialCF / s; :>> clock = UTC; }
		part car : SpatialItem { attribute carDatum :>> coordinateFrame { :>> mRefs = (mm, mm, mm); } }
	}`)
	if len(res.Diagnostics) > 0 {
		t.Fatalf("fixture has diagnostics: %v", res.Diagnostics)
	}
	return s
}

// A frame named at the prompt renders as its declaration over its axis units;
// one composed by arithmetic keeps its declaration's name and composes each axis.
func TestEvalCoordinateFrames(t *testing.T) {
	s := coordinateFrameSession(t)
	wants(t, run(t, s, "%eval Frames::spatialCF"), "= spatialCF [m, m, m]")
	wants(t, run(t, s, "%eval Frames::velocityCF"), "= velocityCF [m/s, m/s, m/s]")
	wants(t, run(t, s, "%eval Frames::accelerationCF"), "= accelerationCF [m/s**2, m/s**2, m/s**2]")
	wants(t, run(t, s, "%eval Frames::spatialCF / s == Frames::velocityCF"), "= true")
	wants(t, run(t, s, "%eval Frames::spatialCF.mRefs"), "= [m, m, m]")
	wants(t, run(t, s, "%eval Frames::spatialCF.dimensions"), "= [3]")
	wants(t, run(t, s, "%eval Frames::car.carDatum"), "= carDatum [mm, mm, mm]")
	wants(t, run(t, s, "%eval Frames::car.coordinateFrame == Frames::car.carDatum"), "= true")
	wants(t, run(t, s, "%eval Frames::spatialCF istype MeasurementReferences::VectorMeasurementReference"), "= true")
	wants(t, run(t, s, "%eval Frames::p"), "= ⟨1.0, 2.0, 3.0⟩ [spatialCF]")
	wants(t, run(t, s, "%eval Frames::p.mRef"), "= spatialCF [m, m, m]")
	wants(t, run(t, s, "%eval Frames::p.mRef == Frames::spatialCF"), "= true")
	wants(t, run(t, s, "%eval Frames::p.num"), "= [1.0, 2.0, 3.0]")
}

// Arithmetic over vectors in one frame stays in it; a number keeps the frame and a
// quantity composes it, as `CoordinateFrame*` and `/` do; vectors over two frames,
// or over a frame and none, are not combined, and a framed vector equals no
// unframed one.
func TestEvalFramedVectorArithmetic(t *testing.T) {
	s := coordinateFrameSession(t)
	wants(t, run(t, s, "%eval Frames::p + Frames::p"), "= ⟨2.0, 4.0, 6.0⟩ [spatialCF]")
	wants(t, run(t, s, "%eval (Frames::p - (1.0, 1.0, 1.0) [Frames::spatialCF]).mRef == Frames::spatialCF"), "= true")
	wants(t, run(t, s, "%eval -Frames::p"), "= ⟨-1.0, -2.0, -3.0⟩ [spatialCF]")
	wants(t, run(t, s, "%eval 2 * Frames::p"), "= ⟨2.0, 4.0, 6.0⟩ [spatialCF]")
	wants(t, run(t, s, "%eval (Frames::p / 2).mRef == Frames::spatialCF"), "= true")
	wants(t, run(t, s, "%eval (Frames::p / 1 [s]).mRef == Frames::velocityCF"), "= true")
	wants(t, run(t, s, "%eval Frames::p / 1 [s]"), "= ⟨1.0, 2.0, 3.0⟩ [spatialCF / s]")
	wants(t, run(t, s, "%eval VectorCalculations::inner(Frames::p, Frames::p)"), "= 14.0")
	wants(t, run(t, s, "%eval Frames::p + (1.0, 2.0, 3.0) [Frames::datum]"),
		"error:", "type mismatch", "combines vectors over the coordinate frames spatialCF and datum; transform one into the other's frame first")
	wants(t, run(t, s, "%eval Frames::p + VectorFunctions::VectorOf((1.0, 2.0, 3.0)) [m]"),
		"error:", "type mismatch", "combines a vector over the coordinate frame spatialCF with one over no frame")
	wants(t, run(t, s, "%eval VectorCalculations::inner(Frames::p, (1.0, 2.0, 3.0) [Frames::datum])"),
		"error:", "combines vectors over the coordinate frames spatialCF and datum")
	wants(t, run(t, s, "%eval Frames::p == VectorFunctions::VectorOf((1.0, 2.0, 3.0)) [m]"), "= false")
	wants(t, run(t, s, "%eval Frames::p == (1.0, 2.0, 3.0) [Frames::datum]"), "= false")
	wants(t, run(t, s, "%eval Frames::p == (1.0, 2.0, 3.0) [Frames::spatialCF]"), "= true")
}

// A frame written as a feature chain indexes a vector literal as a name does.
func TestEvalVectorOverChainedFrame(t *testing.T) {
	s := coordinateFrameSession(t)
	wants(t, run(t, s, "%eval (1.0, 2.0, 3.0) [Frames::vehicle.body]"), "= ⟨1.0, 2.0, 3.0⟩ [body]")
	wants(t, run(t, s, "%eval (1.0, 2.0, 3.0) [Frames::vehicle.body].mRef == Frames::vehicle.body"), "= true")
	wants(t, run(t, s, "%eval (1.0, 2.0, 3.0) [Frames::car.carDatum].mRef.mRefs"), "= [mm, mm, mm]")
}

// A scale is a value whose unit reads and whose placement converts; one the
// library places on no other reference converts to none, by name.
func TestEvalMeasurementScales(t *testing.T) {
	s := coordinateFrameSession(t)
	wants(t, run(t, s, "%eval Time::UTC"), "= UTC [s]")
	wants(t, run(t, s, "%eval Time::UTC.unit"), "= s")
	wants(t, run(t, s, "%eval 3 [Time::UTC]"), "= 3 [UTC]")
	wants(t, run(t, s, "%eval Time::UTC istype MeasurementReferences::MeasurementScale"), "= true")
	wants(t, run(t, s, "%eval SI::'°C_abs'.transformation.origin"), "= 273.15 [K]")
	wants(t, run(t, s, "%eval QuantityCalculations::ConvertQuantity(300.0 [K], SI::'°C_abs')"), "= 26.85", "['°C_abs']")
	wants(t, run(t, s, "%eval QuantityCalculations::ConvertQuantity(26.85 [SI::'°C_abs'], SI::K)"), "= 300.0 [K]")
	wants(t, run(t, s, "%eval QuantityCalculations::ConvertQuantity(3 [Time::UTC], SI::s)"),
		"error:", "library function is not evaluable",
		"Time::UTC states neither a transformation placing it on another reference nor a quantityValueMapping")
}

// transform carries a vector from the source frame into the placed one; the
// angle converts through SI's seven-digit degree factor, so 180° is not exactly π.
func TestEvalTransform(t *testing.T) {
	s := coordinateFrameSession(t)
	wants(t, run(t, s, "%eval Frames::lbcf.transformation"), "= transformation (datum → lbcf)")
	wants(t, run(t, s, "%eval Frames::lbcf.transformation == Frames::lbcf.transformation"), "= true")
	wants(t, run(t, s, "%eval Frames::lbcf.transformation.source"), "= datum [mm, mm, mm]")
	wants(t, run(t, s, "%eval Frames::lbcf.transformation.target == Frames::lbcf"), "= true")
	wants(t, run(t, s, "%eval Frames::transform(Frames::lbcf.transformation, (11.0, 2.0, 0.0) [Frames::datum])"),
		"= ⟨-0.99999", "-2.00000", "0.0⟩ [lbcf]")
	wants(t, run(t, s, "%eval Frames::transform(Frames::lbcf.transformation, (11.0, 2.0, 0.0) [Frames::datum]).mRef == Frames::lbcf"), "= true")
	wants(t, run(t, s, "%eval Frames::transform(Frames::lbcf.transformation, (1.0, 2.0, 3.0) [Frames::spatialCF])"),
		"error:", "type mismatch", "sourceVector.mRef is spatialCF, not datum, the source of transformation")
}

// What the library defines over no frame is a typed failure naming the frame.
func TestEvalCoordinateFrameLimitsAreTyped(t *testing.T) {
	s := coordinateFrameSession(t)
	wants(t, run(t, s, "%eval Frames::spatialCF + s"), "error:", "operator '+' is not defined for a coordinate frame and a measurement reference")
	wants(t, run(t, s, "%eval Frames::spatialCF / 2"), "error:", "operator '/' is not defined for a coordinate frame and an Integer")
	wants(t, run(t, s, "%eval MeasurementRefCalculations::ToString(Frames::velocityCF)"),
		"error:", "requires a ScalarMeasurementReference, got the coordinate frame velocityCF [m/s, m/s, m/s] of 3 axes")
	wants(t, run(t, s, "%eval (1.0, 2.0) [Frames::spatialCF]"),
		"error:", "2 elements over the coordinate frame spatialCF, whose flattenedSize is 3")
}

// %features lists the frame and the scale a part holds beside its quantities; a
// composed frame is named by the feature it was written to.
func TestFeaturesListCoordinateFrames(t *testing.T) {
	s := coordinateFrameSession(t)
	run(t, s, "%instantiate Frames::vehicle")
	wants(t, run(t, s, "%features Frames::vehicle"), "\n  body = body [m/s, m/s, m/s]", "\n  clock = UTC [s]")
}

// A frame's own transformation lists its source and steps; the library's
// `target = that`, which the runtime answers as the frame, is not an object member.
func TestFeaturesListFrameTransformation(t *testing.T) {
	s := coordinateFrameSession(t)
	run(t, s, "%instantiate Frames")
	got := run(t, s, "%features Frames::lbcf")
	wants(t, got, "\n  mRefs = [mm, mm, mm]", "\n  transformation = Instance(ID: ", "\n    source = datum [mm, mm, mm]",
		"\n      translationVector = ⟨10.0, 0.0, 0.0⟩ [datum]", "\n      angle = 180 ['°']", "\n  dimensions = 3")
	for _, absent := range []string{"target", "that"} {
		if strings.Contains(got, absent) {
			t.Errorf("%%features Frames::lbcf lists %q:\n%s", absent, got)
		}
	}
}

// A scale's listing reads the library's `mRefs = self` and `elements = self` as the
// scale's value, named once rather than expanded into itself; `dimensions = ()` is null.
func TestFeaturesListMeasurementScale(t *testing.T) {
	s := coordinateFrameSession(t)
	run(t, s, "%instantiate SI::'°C_abs'")
	got := run(t, s, "%features SI::'°C_abs'")
	wants(t, got, "\n  unit = '°C'", "\n    origin = 273.15 [K]", "\n  dimensions = null",
		"\n  mRefs = ['°C_abs' ['°C']]", "\n  elements = ['°C_abs' ['°C']]", "\n  flattenedSize = 1")
	if strings.Contains(got, "error") || strings.Count(got, "\n  mRefs = ") != 1 || strings.Contains(got, "\n    mRefs = ") {
		t.Errorf("%%features SI::'°C_abs' errs or lists the scale inside itself:\n%s", got)
	}
}
