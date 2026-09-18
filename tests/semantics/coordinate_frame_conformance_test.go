package semantics_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// coordinateFrameModel indexes the libraries with the Annex A frame declarations:
// a spatial frame, the frames `/ s` composes from it, and vectors written in it.
func coordinateFrameModel(t *testing.T) (*semantics.Model, *symbols.Index) {
	t.Helper()
	idx := libs.NewModelIndex()
	idx.AddDocument("<t>", parser.New(source.New("<t>", []byte(`package T {
		private import ISQ::*;
		private import ISQSpaceTime::*;
		private import SI::*;
		private import Time::*;
		private import MeasurementReferences::*;
		attribute spatialCF : CartesianSpatial3dCoordinateFrame[1] { :>> mRefs = (m, m, m); }
		attribute velocityCF : CartesianVelocity3dCoordinateFrame[1] = spatialCF / s;
		attribute accelerationCF : CartesianAcceleration3dCoordinateFrame[1] = velocityCF / s;
		attribute scaled = spatialCF * s;
		attribute plain : CoordinateFrame = spatialCF / s;
		attribute p : Position3dVector = (1.0, 2.0, 3.0) [spatialCF];
		attribute v = (1.0, 2.0, 3.0) [velocityCF];
		attribute composedV : CartesianVelocity3dVector = (1.0, 2.0, 3.0) [spatialCF / s];
		attribute speed = 3.0 [m / s];
		attribute length = 3.0 [m];
		attribute utc : TimeScale = UTC;
		attribute sum = spatialCF + s;
		attribute byNumber = spatialCF / 2;
		attribute byFrame = s / spatialCF;
		attribute vast : CoordinateFrame { :>> dimensions = 4611686018427387904; :>> mRefs = m; }
		attribute vastVelocity = vast / s;
		attribute overflowing : CoordinateFrame { :>> dimensions : Positive[2] = (4611686018427387904, 4); :>> mRefs = m; }
		attribute overflowingVelocity = overflowing / s;
	}`))).ParseFile())
	idx.ExpandWildcardImports()
	return semantics.NewModel(resolve.New(idx)), idx
}

func boundValue(t *testing.T, idx *symbols.Index, fqn string) (*symbols.Scope, ast.Node) {
	t.Helper()
	sym := dimensionSymbol(t, idx, fqn)
	u, ok := sym.Decl.(*ast.Usage)
	if !ok || u.Value == nil {
		t.Fatalf("%s is not a usage with a value", fqn)
	}
	return sym.OwnerScope, u.Value
}

// TestCoordinateFrameExpr: `cf / u` and `cf * u` over a frame and a unit are a
// CoordinateFrame, as MeasurementRefCalculations declare. Against a frame type they
// hold when the fixed dimensions agree and every axis measures in the dimension the
// type's `mRefs` admit — `spatialCF / s` is a velocity frame, not a spatial one —
// and never against a unit, a scale, a quantity value or a number.
func TestCoordinateFrameExpr(t *testing.T) {
	m, idx := coordinateFrameModel(t)
	frame := dimensionSymbol(t, idx, "MeasurementReferences::CoordinateFrame")
	for _, tc := range []struct {
		feature string
		want    string
		holds   bool
		found   string
	}{
		{"T::velocityCF", "ISQSpaceTime::CartesianVelocity3dCoordinateFrame", true, ""},
		{"T::velocityCF", "MeasurementReferences::3dCoordinateFrame", true, ""},
		{"T::velocityCF", "MeasurementReferences::CoordinateFrame", true, ""},
		{"T::velocityCF", "MeasurementReferences::VectorMeasurementReference", true, ""},
		{"T::velocityCF", "MeasurementReferences::TensorMeasurementReference", true, ""},
		{"T::velocityCF", "ISQSpaceTime::CartesianSpatial3dCoordinateFrame", false, "axes measure in dimension L·T^-1, where CartesianSpatial3dCoordinateFrame admits L"},
		{"T::velocityCF", "ISQSpaceTime::CartesianAcceleration3dCoordinateFrame", false, "axes measure in dimension L·T^-1, where CartesianAcceleration3dCoordinateFrame admits L·T^-2"},
		{"T::velocityCF", "ISQBase::LengthUnit", false, "a coordinate frame composed from another's axes"},
		{"T::velocityCF", "ISQSpaceTime::SpeedUnit", false, "a coordinate frame composed from another's axes"},
		{"T::velocityCF", "MeasurementReferences::ScalarMeasurementReference", false, "a coordinate frame composed from another's axes"},
		{"T::velocityCF", "Time::TimeScale", false, "a coordinate frame of dimensions [3], where TimeScale fixes dimensions []"},
		{"T::velocityCF", "ISQSpaceTime::SpeedValue", false, "a coordinate frame composed from another's axes"},
		{"T::velocityCF", "ScalarValues::Real", false, "a coordinate frame composed from another's axes"},
		{"T::accelerationCF", "ISQSpaceTime::CartesianAcceleration3dCoordinateFrame", true, ""},
		{"T::accelerationCF", "ISQSpaceTime::CartesianVelocity3dCoordinateFrame", false, "axes measure in dimension L·T^-2"},
		{"T::scaled", "ISQSpaceTime::CartesianSpatial3dCoordinateFrame", false, "axes measure in dimension L·T, where CartesianSpatial3dCoordinateFrame admits L"},
		{"T::scaled", "MeasurementReferences::CoordinateFrame", true, ""},
		{"T::plain", "MeasurementReferences::CoordinateFrame", true, ""},
		{"T::vastVelocity", "MeasurementReferences::CoordinateFrame", true, ""},
		{"T::vastVelocity", "ISQSpaceTime::CartesianVelocity3dCoordinateFrame", false, "dimensions [4611686018427387904], where CartesianVelocity3dCoordinateFrame fixes dimensions [3]"},
		{"T::overflowingVelocity", "MeasurementReferences::CoordinateFrame", true, ""},
		{"T::overflowingVelocity", "ISQSpaceTime::CartesianVelocity3dCoordinateFrame", false, "dimensions [4611686018427387904 4], where CartesianVelocity3dCoordinateFrame fixes dimensions [3]"},
	} {
		t.Run(tc.feature+" : "+tc.want, func(t *testing.T) {
			scope, e := boundOperatorExpr(t, idx, tc.feature)
			if got := m.CoordinateFrameExprType(scope, e); got != frame {
				t.Fatalf("type of %s = %v, want CoordinateFrame", tc.feature, got)
			}
			if got := m.ExprResultType(scope, e); got != frame {
				t.Fatalf("result type of %s = %v, want CoordinateFrame", tc.feature, got)
			}
			want := dimensionSymbol(t, idx, tc.want)
			c, ok := m.CoordinateFrameExprConformance(scope, e, want)
			if !ok || !c.Known {
				t.Fatalf("%s against %s is not judged (%v, %v)", tc.feature, tc.want, ok, c.Known)
			}
			if c.Holds != tc.holds {
				t.Fatalf("%s conforms to %s = %v, want %v (found %q)", tc.feature, tc.want, c.Holds, tc.holds, c.Found)
			}
			if !strings.Contains(c.Found, tc.found) {
				t.Fatalf("%s against %s found %q, want it to mention %q", tc.feature, tc.want, c.Found, tc.found)
			}
			if got := m.ExprConformsTo(scope, e, want); got.Known != c.Known || got.Holds != c.Holds {
				t.Fatalf("ExprConformsTo(%s, %s) = %+v, want %+v", tc.feature, tc.want, got, c)
			}
		})
	}
	for _, feature := range []string{"T::sum", "T::byNumber", "T::byFrame"} {
		scope, e := boundOperatorExpr(t, idx, feature)
		if got := m.CoordinateFrameExprType(scope, e); got != nil {
			t.Errorf("%s typed %v, want no coordinate frame", feature, got)
		}
		if _, ok := m.CoordinateFrameExprConformance(scope, e, frame); ok {
			t.Errorf("%s judged as a coordinate frame", feature)
		}
	}
}

// TestFixedDimensions: a '3dCoordinateFrame' fixes dimensions = 3 through its
// redefinition of Array's `dimensions`, a feature typed by one inherits the fix, and
// a plain CoordinateFrame fixes none.
func TestFixedDimensions(t *testing.T) {
	m, idx := coordinateFrameModel(t)
	for _, tc := range []struct {
		fqn   string
		want  []int64
		fixed bool
	}{
		{"MeasurementReferences::3dCoordinateFrame", []int64{3}, true},
		{"Time::TimeScale", []int64{}, true},
		{"ISQSpaceTime::CartesianVelocity3dCoordinateFrame", []int64{3}, true},
		{"T::spatialCF", []int64{3}, true},
		{"MeasurementReferences::CoordinateFrame", nil, false},
		{"T::plain", nil, false},
	} {
		got, fixed := m.FixedDimensions(dimensionSymbol(t, idx, tc.fqn))
		if fixed != tc.fixed || len(got) != len(tc.want) {
			t.Errorf("FixedDimensions(%s) = %v, %v; want %v, %v", tc.fqn, got, fixed, tc.want, tc.fixed)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("FixedDimensions(%s) = %v, want %v", tc.fqn, got, tc.want)
			}
		}
	}
}

// TestComposedFrameConforms judges a frame the runtime composed, whose axes and
// dimensions are all known: a two-axis frame is refused by a '3dCoordinateFrame'
// type by its dimensions, and an axis of no known dimension is not judged.
func TestComposedFrameConforms(t *testing.T) {
	m, idx := coordinateFrameModel(t)
	metre, err := m.UnitTermOf(dimensionSymbol(t, idx, "SI::m"))
	if err != nil {
		t.Fatal(err)
	}
	length, ok := m.DimensionOfUnit(metre)
	if !ok {
		t.Fatal("metre has no dimension")
	}
	spatial := dimensionSymbol(t, idx, "ISQSpaceTime::CartesianSpatial3dCoordinateFrame")
	three := semantics.ComposedFrame{Dimensions: []int64{3}, HasDimensions: true, AxisDimensions: []semantics.UnitTerm{length.Term, length.Term, length.Term}}
	if c := m.ComposedFrameConforms(three, spatial); !c.Known || !c.Holds {
		t.Errorf("three length axes against a spatial frame: %+v", c)
	}
	two := semantics.ComposedFrame{Dimensions: []int64{2}, HasDimensions: true, AxisDimensions: []semantics.UnitTerm{length.Term, length.Term}}
	c := m.ComposedFrameConforms(two, spatial)
	if !c.Known || c.Holds || !strings.Contains(c.Found, "dimensions [2], where CartesianSpatial3dCoordinateFrame fixes dimensions [3]") {
		t.Errorf("two axes against a spatial frame: %+v", c)
	}
	unknown := semantics.ComposedFrame{Dimensions: []int64{3}, HasDimensions: true}
	if c := m.ComposedFrameConforms(unknown, spatial); !c.Known || !c.Holds {
		t.Errorf("axes of unknown dimension against a spatial frame: %+v", c)
	}
	uniform := semantics.ComposedFrame{Dimensions: []int64{3}, HasDimensions: true, AxisDimensions: []semantics.UnitTerm{length.Term}, Uniform: true}
	if c := m.ComposedFrameConforms(uniform, spatial); !c.Known || !c.Holds {
		t.Errorf("axes uniformly of length against a spatial frame: %+v", c)
	}
	velocity := dimensionSymbol(t, idx, "ISQSpaceTime::CartesianVelocity3dCoordinateFrame")
	c = m.ComposedFrameConforms(uniform, velocity)
	if !c.Known || c.Holds || !strings.Contains(c.Found, "whose axes measure in dimension L, where CartesianVelocity3dCoordinateFrame admits L·T^-1") {
		t.Errorf("axes uniformly of length against a velocity frame: %+v", c)
	}
	if c := m.ComposedFrameConforms(three, dimensionSymbol(t, idx, "ISQBase::LengthValue")); !c.Known || c.Holds {
		t.Errorf("a frame against a quantity value: %+v", c)
	}
}

// TestFrameQuantityConformance: numbers written in a frame (`(1, 2, 3) [spatialCF]`)
// are a VectorQuantityValue of the frame's kind, not a scalar; a number in a unit
// stays a ScalarQuantityValue; and a scale is a scalar reference, not a vector one.
func TestFrameQuantityConformance(t *testing.T) {
	m, idx := coordinateFrameModel(t)
	for _, tc := range []struct {
		feature string
		want    string
		holds   bool
		found   string
	}{
		{"T::p", "ISQSpaceTime::Position3dVector", true, ""},
		{"T::p", "Quantities::VectorQuantityValue", true, ""},
		{"T::p", "ISQBase::LengthValue", false, "a vector quantity in spatialCF (a CartesianSpatial3dCoordinateFrame)"},
		{"T::p", "Quantities::ScalarQuantityValue", false, "a vector quantity in spatialCF"},
		{"T::v", "Quantities::VectorQuantityValue", true, ""},
		{"T::v", "ISQSpaceTime::Position3dVector", false, "a vector quantity in velocityCF"},
		{"T::composedV", "ISQSpaceTime::CartesianVelocity3dVector", true, ""},
		{"T::composedV", "Quantities::VectorQuantityValue", true, ""},
		{"T::composedV", "ISQSpaceTime::CartesianPosition3dVector", false, "a vector quantity in spatialCF/s, a coordinate frame whose axes measure in dimension L·T^-1, where CartesianSpatial3dCoordinateFrame admits L"},
		{"T::composedV", "ISQSpaceTime::SpeedValue", false, "a vector quantity in spatialCF/s"},
		{"T::composedV", "Quantities::ScalarQuantityValue", false, "a vector quantity in spatialCF/s"},
		{"T::speed", "ISQSpaceTime::SpeedValue", true, ""},
		{"T::speed", "ISQSpaceTime::CartesianVelocity3dVector", false, ""},
		{"T::length", "ISQBase::LengthValue", true, ""},
		{"T::length", "Quantities::VectorQuantityValue", true, ""},
		{"T::length", "ISQSpaceTime::Position3dVector", false, "a quantity in metre (a LengthUnit)"},
	} {
		t.Run(tc.feature+" : "+tc.want, func(t *testing.T) {
			scope, value := boundValue(t, idx, tc.feature)
			c := m.ExprConformsTo(scope, value, dimensionSymbol(t, idx, tc.want))
			if !c.Known {
				t.Fatalf("%s against %s is not judged", tc.feature, tc.want)
			}
			if c.Holds != tc.holds {
				t.Fatalf("%s conforms to %s = %v, want %v (found %q)", tc.feature, tc.want, c.Holds, tc.holds, c.Found)
			}
			if !strings.Contains(c.Found, tc.found) {
				t.Fatalf("%s against %s found %q, want it to mention %q", tc.feature, tc.want, c.Found, tc.found)
			}
		})
	}
	for _, tc := range []struct {
		fqn    string
		vector bool
	}{
		{"T::spatialCF", true},
		{"T::velocityCF", true},
		{"Time::UTC", false},
		{"SI::°C_abs", false},
		{"SI::m", false},
	} {
		if got := m.IsVectorReference(dimensionSymbol(t, idx, tc.fqn)); got != tc.vector {
			t.Errorf("IsVectorReference(%s) = %v, want %v", tc.fqn, got, tc.vector)
		}
	}
	scope, value := boundValue(t, idx, "T::utc")
	if c := m.ExprConformsTo(scope, value, dimensionSymbol(t, idx, "Time::TimeScale")); !c.Known || !c.Holds {
		t.Errorf("UTC against TimeScale: %+v", c)
	}
	if c := m.ExprConformsTo(scope, value, dimensionSymbol(t, idx, "MeasurementReferences::CoordinateFrame")); !c.Known || !c.Holds {
		t.Errorf("UTC against CoordinateFrame: %+v", c)
	}
	if c := m.ExprConformsTo(scope, value, dimensionSymbol(t, idx, "ISQBase::DurationUnit")); !c.Known || c.Holds {
		t.Errorf("UTC against DurationUnit: %+v", c)
	}
}
