package runtime

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// TestRuntimeRobustnessQuantities covers quantities: missing values, writes of another
// dimension, measurement references, tensors and coordinate frames.
func TestRuntimeRobustnessQuantities(t *testing.T) {
	t.Run("quantity_incommensurable_comparison", testQuantityIncommensurableComparison)
	t.Run("quantity_index_is_not_a_unit", testQuantityIndexIsNotAUnit)
	t.Run("quantity_unit_shadowed_by_sibling", testQuantityUnitShadowedBySibling)
	t.Run("quantity_qualified_unit_is_not_shadowing", testQuantityQualifiedUnitIsNotShadowing)
	t.Run("quantity_shadowed_unit_without_a_qualifier", testQuantityShadowedUnitWithoutAQualifier)
	t.Run("quantity_cyclic_unit_definition", testQuantityCyclicUnitDefinition)
	t.Run("quantity_calculation_that_has_no_value", testQuantityCalculationThatHasNoValue)
	t.Run("quantity_write_of_another_dimension", testQuantityWriteOfAnotherDimension)
	t.Run("measurement_reference_failure_modes", testMeasurementReferenceFailureModes)
	t.Run("tensor_quantity_failure_modes", testTensorQuantityFailureModes)
	t.Run("coordinate_frame_failure_modes", testCoordinateFrameFailureModes)
}

// testQuantityWriteOfAnotherDimension: a write whose quantity measures in a
// dimension the target's declared quantity value type does not is refused with
// a typed error, while a commensurable unit at another scale is written.
func testQuantityWriteOfAnotherDimension(t *testing.T) {
	const duration = "ISQ::DurationValue"
	for _, tc := range []struct {
		name     string
		declared string
		value    string
		want     error
	}{
		{"speed into a duration", duration, "3.0 [SI::m / SI::s]", ErrTypeMismatch},
		{"length into a duration", duration, "5.0 [SI::m]", ErrTypeMismatch},
		{"dimensionless into a duration", duration, "5.0 [MeasurementReferences::one]", ErrTypeMismatch},
		{"another scale of the same dimension", duration, "5.0 [SI::min]", nil},
		{"a unit the target fixes no dimension against", "Quantities::ScalarQuantityValue", "5.0 [SI::m]", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := fmt.Sprintf(`
				package test {
					private import SI::*;
					action w {
						attribute t : %s = 0.0 [s];
						first start;
						action step { assign t := %s; }
						done;
						succession first start then step;
						succession first step then done;
					}
				}
			`, tc.declared, tc.value)
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "w", ast.DefAction)
			if sym == nil {
				t.Fatal("action w not found")
			}
			_, err := ctx.ExecuteAction(sym)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("ExecuteAction err = %v, want the write to conform", err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("ExecuteAction err = %v, want %v", err, tc.want)
			}
		})
	}
}

// testMeasurementReferenceFailureModes: a measurement reference the runtime
// cannot honestly compute with is a typed error at the write or the call, never a
// value: a unit of another dimension does not conform, a conversion between
// dimensions is incommensurable, a number is not a reference, a reference is not
// a number, a unit is not a scale, a scale the library places on no other
// reference converts to none, and the tensors the library declares have no value.
func testMeasurementReferenceFailureModes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		declared string
		value    string
		want     error
	}{
		{"speed unit into a length unit", "ISQ::LengthUnit", "SI::m / SI::s", ErrTypeMismatch},
		{"dimensionless unit into a length unit", "ISQ::LengthUnit", "MeasurementReferences::one", ErrTypeMismatch},
		{"unit into a length value", "ISQ::LengthValue", "SI::m", ErrTypeMismatch},
		{"composed unit of the same dimension", "ISQ::AreaUnit", "SI::m * SI::m", nil},
		{"composed unit as a derived unit", "MeasurementReferences::DerivedUnit", "SI::km / SI::L", nil},
		{"another scale of the same dimension", "ISQ::LengthUnit", "SI::km", nil},
		{"composed unit into a time scale", "Time::TimeScale", "SI::h * SI::s / SI::min", ErrTypeMismatch},
		{"composed unit into an interval scale", "MeasurementReferences::IntervalScale", "SI::h * SI::s / SI::min", ErrTypeMismatch},
		{"time scale as a value", "Time::TimeScale", "Time::UTC", nil},
		{"interval scale as a value", "MeasurementReferences::IntervalScale", "SI::'°C_abs'", nil},
		{"scale as a unit", "MeasurementReferences::IntervalScale", "SI::'°C'", ErrTypeMismatch},
		{"unit as a scale", "ISQ::LengthUnit", "Time::UTC", ErrTypeMismatch},
		{"conversion to a scale", "ISQ::ThermodynamicTemperatureValue", "QuantityCalculations::ConvertQuantity(300.0 [SI::K], SI::'°C_abs')", nil},
		{"conversion from a scale", "ISQ::ThermodynamicTemperatureValue", "QuantityCalculations::ConvertQuantity(26.85 [SI::'°C_abs'], SI::K)", nil},
		{"conversion to a scale placed on another dimension", "ISQ::LengthValue", "QuantityCalculations::ConvertQuantity(3.0 [SI::m], SI::'°C_abs')", ErrIncommensurableUnits},
		{"conversion to a scale with no mapping", "ISQ::DurationValue", "QuantityCalculations::ConvertQuantity(3.0 [SI::s], Time::UTC)", errUnevaluableLibraryFunction},
		{"conversion from a scale with no mapping", "ISQ::DurationValue", "QuantityCalculations::ConvertQuantity(3.0 [Time::UTC], SI::s)", errUnevaluableLibraryFunction},
		{"incommensurable conversion", "ISQ::LengthValue", "QuantityCalculations::ConvertQuantity(3.0 [SI::m], SI::s)", ErrIncommensurableUnits},
		{"conversion to a number", "ISQ::LengthValue", "QuantityCalculations::ConvertQuantity(3.0 [SI::m], 3)", ErrTypeMismatch},
		{"reference scaled by a number", "ISQ::LengthUnit", "SI::m * 3", ErrTypeMismatch},
		{"reference raised to a reference", "ISQ::LengthUnit", "SI::m ** SI::m", ErrTypeMismatch},
		{"declaration member of a reference", "Quantities::QuantityDimension", "SI::m.quantityDimension", nil},
		{"quantity dimension of a composed unit", "Quantities::QuantityDimension", "(SI::m / SI::s).quantityDimension", errUnevaluableLibraryFunction},
		{"unit conversion of a composed unit", "MeasurementReferences::UnitConversion", "(SI::m / SI::s).unitConversion", errUnevaluableLibraryFunction},
		{"unit power factors of a composed unit", "MeasurementReferences::UnitPowerFactor", "(SI::m ** 2).unitPowerFactors", errUnevaluableLibraryFunction},
		{"definitional quantity values of a composed unit", "MeasurementReferences::DefinitionalQuantityValue", "(SI::m / SI::s).definitionalQuantityValues", errUnevaluableLibraryFunction},
		{"vector reference over two units", "Quantities::VectorQuantityValue", "VectorCalculations::'['((1.0, 2.0), (SI::m, SI::s))", ErrTypeMismatch},
		{"vector reference over a unit", "Quantities::VectorQuantityValue", "VectorCalculations::'['((1.0, 2.0), SI::m)", ErrTypeMismatch},
		{"transformation that is a vector", "Quantities::VectorQuantityValue", "VectorCalculations::transform(VectorFunctions::VectorOf((1.0, 2.0)) [SI::m], VectorFunctions::VectorOf((0.0, 1.0)) [SI::m])", ErrTypeMismatch},
		{"scale scaled by a unit", "MeasurementReferences::CoordinateFrame", "Time::UTC / SI::s", errUnevaluableLibraryFunction},
		{"outer product", "Quantities::TensorQuantityValue", "VectorCalculations::outer((1.0, 2.0), (3.0, 4.0))", errUnevaluableLibraryFunction},
		{"tensor sum of number sequences", "Quantities::TensorQuantityValue", "TensorCalculations::'+'((1.0, 2.0), (3.0, 4.0))", ErrTypeMismatch},
		{"tensor product", "Quantities::TensorQuantityValue", "TensorCalculations::tensorTensorMult(VectorFunctions::VectorOf((1.0, 2.0)) [SI::m], VectorFunctions::VectorOf((3.0, 4.0)) [SI::m])", errUnevaluableLibraryFunction},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := fmt.Sprintf(`
				package test {
					part def Holder {
						attribute value : %s = %s;
					}
				}
			`, tc.declared, tc.value)
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "Holder", ast.DefPart)
			if sym == nil {
				t.Fatal("part def Holder not found")
			}
			inst, err := ctx.Instantiate(sym)
			if err != nil {
				t.Fatalf("Instantiate err = %v", err)
			}
			_, err = inst.GetFeatureValue(ctx, "value")
			if tc.want == nil {
				if err != nil {
					t.Fatalf("value err = %v, want the reference to be held", err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("value err = %v, want %v", err, tc.want)
			}
		})
	}
}

// testTensorQuantityFailureModes: a tensor the library does not determine is a typed
// error at the call, never a value: components that do not fill the reference,
// operands of two shapes, incommensurable components, a unit predicate over a
// shape with no identity, and the five calculations the library leaves bodiless
// and underspecified, each naming itself.
func testTensorQuantityFailureModes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		want  error
		names string
	}{
		{"element count", "TensorCalculations::'['((1.0, 2.0, 3.0), stressRef)", ErrMultiplicityViolation, "n = mRef.flattenedSize"},
		{"one reference for four components", "TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), oneRef)", ErrMultiplicityViolation, "oneRef"},
		{"a unit as the reference", "TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), (Pa, Pa, Pa, Pa))", ErrTypeMismatch, "TensorMeasurementReference"},
		{"shape mismatch", "stress + TensorCalculations::'['((1.0, 2.0, 3.0), rowRef)", ErrMultiplicityViolation, "dimensions [2, 2] and [3] differ"},
		{"tensor minus scalar", "stress - 2 [Pa]", ErrMultiplicityViolation, "dimensions [2, 2] and [] differ"},
		{"incommensurable components", "stress + TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), lengthRef)", ErrIncommensurableUnits, ""},
		{"non-square unit predicate", "TensorCalculations::isUnitTensorQuantity(TensorCalculations::'['((1.0, 2.0, 3.0), rowRef))", errUnevaluableLibraryFunction, "only a square tensor of order two has an identity"},
		{"unset covariance", "stress.contravariantOrder", errUnevaluableLibraryFunction, "orderSum"},
		{"tensor times vector", "TensorCalculations::tensorVectorMult(stress, VectorFunctions::VectorOf((1.0, 2.0)) [Pa])", errUnevaluableLibraryFunction, "TensorCalculations::tensorVectorMult"},
		{"vector times tensor", "TensorCalculations::vectorTensorMult(VectorFunctions::VectorOf((1.0, 2.0)) [Pa], stress)", errUnevaluableLibraryFunction, "TensorCalculations::vectorTensorMult"},
		{"tensor times tensor", "TensorCalculations::tensorTensorMult(stress, stress)", errUnevaluableLibraryFunction, "TensorCalculations::tensorTensorMult"},
		{"tensor times tensor by operator", "stress * stress", errUnevaluableLibraryFunction, "TensorCalculations::tensorTensorMult"},
		{"outer product", "VectorCalculations::outer(VectorFunctions::VectorOf((1.0, 2.0)) [Pa], VectorFunctions::VectorOf((1.0, 2.0)) [Pa])", errUnevaluableLibraryFunction, "VectorCalculations::outer"},
		{"transform", "TensorCalculations::transform(stressRef, stress)", errUnevaluableLibraryFunction, "TensorCalculations::transform"},
		{"rank three, too few indexes", "cube#(1, 2)", ErrMultiplicityViolation, "2 indexes address an array of rank 3"},
		{"rank three, too many indexes", "cube#(1, 1, 1, 1)", ErrMultiplicityViolation, "4 indexes address an array of rank 3"},
		{"rank three, first index low", "cube#(0, 1, 1)", ErrIndexOutOfRange, "index 1 is 0, dimension 1 has 1..2"},
		{"rank three, middle index high", "cube#(1, 3, 1)", ErrIndexOutOfRange, "index 2 is 3, dimension 2 has 1..2"},
		{"rank three, last index high", "cube#(1, 1, 3)", ErrIndexOutOfRange, "index 3 is 3, dimension 3 has 1..2"},
		{"rank three, non-integer index", "cube#(1, 1.5, 1)", ErrTypeMismatch, "requires an Integer index"},
		{"rank three, too few components", "TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0), cubeRef)", ErrMultiplicityViolation, "7 elements for a reference of dimensions [2, 2, 2]"},
		{"rank three, too many components", "TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0), cubeRef)", ErrMultiplicityViolation, "9 elements for a reference of dimensions [2, 2, 2]"},
		{"rank three against rank two", "cube + stress", ErrMultiplicityViolation, "dimensions [2, 2, 2] and [2, 2] differ"},
		{"rank three shapes differ", "cube - TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0, 11.0, 12.0), slabRef)", ErrMultiplicityViolation, "dimensions [2, 2, 2] and [2, 3, 2] differ"},
		{"rank three unit predicate", "TensorCalculations::isUnitTensorQuantity(cube)", errUnevaluableLibraryFunction, "only a square tensor of order two has an identity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := fmt.Sprintf(`
				package test {
					private import ISQ::*;
					private import SI::*;
					private import MeasurementReferences::*;
					private import Quantities::*;
					attribute stressRef : TensorMeasurementReference { :>> dimensions = (2, 2); :>> mRefs = (Pa, Pa, Pa, Pa); }
					attribute cubeRef : TensorMeasurementReference { :>> dimensions = (2, 2, 2); :>> mRefs = (Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa); }
					attribute slabRef : TensorMeasurementReference { :>> dimensions = (2, 3, 2); :>> mRefs = (Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa, Pa); }
					attribute cube = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0), cubeRef);
					attribute lengthRef : TensorMeasurementReference { :>> dimensions = (2, 2); :>> mRefs = (Pa, m, Pa, Pa); }
					attribute rowRef : TensorMeasurementReference { :>> dimensions = (3); :>> mRefs = (Pa, Pa, Pa); }
					attribute oneRef : TensorMeasurementReference { :>> dimensions = (2, 2); :>> mRefs = Pa; }
					attribute stress = TensorCalculations::'['((1.0, 2.0, 3.0, 4.0), stressRef);
					part def Holder {
						attribute value : TensorQuantityValue = %s;
					}
				}
			`, tc.value)
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "Holder", ast.DefPart)
			if sym == nil {
				t.Fatal("part def Holder not found")
			}
			inst, err := ctx.Instantiate(sym)
			if err != nil {
				t.Fatalf("Instantiate err = %v", err)
			}
			_, err = inst.GetFeatureValue(ctx, "value")
			if !errors.Is(err, tc.want) {
				t.Fatalf("value err = %v, want %v", err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.names) {
				t.Errorf("value err = %v, want it to state %q", err, tc.names)
			}
		})
	}
}

// testCoordinateFrameFailureModes: what the runtime cannot honestly compute about
// a coordinate frame, a scale or a transformation is a typed error naming what is
// missing or malformed: a frame with no mRefs, a vector with a number per axis
// short, a transformation applied to a vector in another frame, a subtype of
// CoordinateTransformation the library gives no shape, a placement whose origin
// is no vector quantity or whose basis is singular, a translation in an
// incommensurable unit, a scale placed on two references at odds, and matrices,
// sequences and placements missing what they declare. A scalar written where a
// reference, vector or step is declared is refused by the write itself.
func testCoordinateFrameFailureModes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		body     string
		declared string
		value    string
		want     error
		message  string
	}{
		{"frame with no mRefs", `attribute wcf : CoordinateFrame;`,
			"CoordinateFrame", "wcf", ErrNoValue, "coordinate frame wcf states no mRefs; TensorMeasurementReference declares mRefs: ScalarMeasurementReference[1..*], one per axis"},
		{"frame whose mRefs are not one per axis", `attribute bad : CartesianSpatial3dCoordinateFrame { :>> mRefs = (m, m); }`,
			"CoordinateFrame", "bad", ErrMultiplicityViolation, "bad.mRefs: multiplicity violation: 2 value(s) bound to a feature with multiplicity lower bound 3"},
		{"frame whose mRefs are not one per stated dimension", `attribute bad : CoordinateFrame { :>> dimensions = 2; :>> mRefs = (m, m, m); }`,
			"CoordinateFrame", "bad", ErrMultiplicityViolation, "bad states 3 mRefs for dimensions [2], whose flattenedSize is 2"},
		{"frame whose dimensions overflow", `attribute bad : CoordinateFrame { :>> dimensions : Positive[2] = (4611686018427387904, 4); :>> mRefs = (m, m, m); }`,
			"CoordinateFrame", "bad", errIntegerUnaddressable, "bad: flattenedSize of dimensions [4611686018427387904, 4] is 18446744073709551616"},
		{"frame whose mRef is a number", `attribute bad : CoordinateFrame { :>> mRefs = (m, 2); }`,
			"CoordinateFrame", "bad", ErrTypeMismatch, "bad.mRefs: type mismatch: cannot write 2 (an Integer) to a feature typed by ScalarMeasurementReference"},
		{"vector short of an axis", ``,
			"Position3dVector", "(1.0, 2.0) [spatialCF]", ErrMultiplicityViolation, "2 elements over the coordinate frame spatialCF, whose flattenedSize is 3; elements: Number[1..n] with n = mRef.flattenedSize"},
		{"vector of a number too many", ``,
			"Position3dVector", "(1.0, 2.0, 3.0, 4.0) [spatialCF]", ErrMultiplicityViolation, "4 elements over the coordinate frame spatialCF, whose flattenedSize is 3"},
		{"vector of no numbers", ``,
			"Position3dVector", "(m, m, m) [spatialCF]", errNotAQuantity, "want 3 numbers, one per axis"},
		{"scale composed with a unit", ``,
			"CoordinateFrame", "Time::UTC * s", errUnevaluableLibraryFunction, "UTC is a measurement scale"},
		{"frame composed with a number", ``,
			"CoordinateFrame", "spatialCF / 2", ErrTypeMismatch, "operator '/' is not defined for a coordinate frame and an Integer"},
		{"frame composed with a frame", ``,
			"CoordinateFrame", "spatialCF * spatialCF", ErrTypeMismatch, "operator '*' is not defined for a coordinate frame and coordinate frame; MeasurementRefCalculations::'CoordinateFrame*' takes a MeasurementUnit"},
		{"composed frame into a frame of another dimension", ``,
			"CartesianSpatial3dCoordinateFrame", "spatialCF / s", ErrTypeMismatch, "axis 1 measures in dimension L·T^-1, where CartesianSpatial3dCoordinateFrame admits L"},
		{"frame into a unit", ``,
			"LengthUnit", "spatialCF", ErrTypeMismatch, "cannot write the coordinate frame spatialCF [m, m, m], a CartesianSpatial3dCoordinateFrame, to a feature typed by LengthUnit"},
		{"frame into a scale", ``,
			"Time::TimeScale", "spatialCF", ErrTypeMismatch, "to a feature typed by TimeScale"},
		{"vector into the wrong frame's vector type", ``,
			"ISQSpaceTime::CartesianVelocity3dCoordinateFrame", "spatialCF", ErrTypeMismatch, "a CartesianSpatial3dCoordinateFrame, to a feature typed by CartesianVelocity3dCoordinateFrame"},
		{"vector over a frame into a vector type admitting another", `attribute velocityCF : CartesianVelocity3dCoordinateFrame = spatialCF / s;`,
			"CartesianPosition3dVector", "(1.0, 2.0, 3.0) [velocityCF]", ErrTypeMismatch, "cannot write ⟨1.0, 2.0, 3.0⟩ [velocityCF] (a vector quantity over the coordinate frame velocityCF [m/s, m/s, m/s], a CartesianVelocity3dCoordinateFrame) to a feature typed by CartesianPosition3dVector, whose mRef admits CartesianSpatial3dCoordinateFrame"},
		{"vector over a composed frame into a vector type admitting another", ``,
			"CartesianPosition3dVector", "(1.0, 2.0, 3.0) [spatialCF / s]", ErrTypeMismatch, "cannot write ⟨1.0, 2.0, 3.0⟩ [spatialCF / s] (a vector quantity over the coordinate frame spatialCF / s [m/s, m/s, m/s], a coordinate frame whose axis 1 measures in dimension L·T^-1, where CartesianSpatial3dCoordinateFrame admits L) to a feature typed by CartesianPosition3dVector, whose mRef admits CartesianSpatial3dCoordinateFrame"},
		{"transformation applied to a vector in another frame", placementBody,
			"Position3dVector", "transform(shifted.transformation, (1.0, 2.0, 3.0) [spatialCF])", ErrTypeMismatch, "sourceVector.mRef is spatialCF, not datum, the source of transformation"},
		{"transformation applied to a vector in a unit", placementBody,
			"Position3dVector", "transform(shifted.transformation, VectorFunctions::VectorOf((1.0, 2.0, 3.0)) [mm])", ErrTypeMismatch, "written over the unit mm and no coordinate frame"},
		{"transformation applied to numbers", placementBody,
			"Position3dVector", "transform(shifted.transformation, (1.0, 2.0, 3.0))", ErrTypeMismatch, `parameter "sourceVector" requires a vector quantity over datum`},
		{"transformation of no recognized shape", `
			attribute def Bespoke :> CoordinateTransformation;
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : Bespoke { :>> source = datum; } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", errUnevaluableLibraryFunction, "transformation is a Bespoke, a CoordinateTransformation of no shape the library gives a meaning"},
		{"placement whose origin is a string", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : CoordinateFramePlacement { :>> source = datum; :>> origin = "2024-01-01T00:00:00Z"; } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrTypeMismatch, `transformation.origin: type mismatch: cannot write "2024-01-01T00:00:00Z" (string) to a feature typed by VectorQuantityValue`},
		{"placement whose origin is missing", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : CoordinateFramePlacement { :>> source = datum; } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrNoValue, "states no origin; CoordinateFramePlacement declares origin: VectorQuantityValue[1]"},
		{"placement whose origin is in another frame", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : CoordinateFramePlacement { :>> source = datum; :>> origin = (1.0, 2.0, 3.0) [spatialCF]; } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrTypeMismatch, "origin is a vector quantity in spatialCF, not a vector quantity over datum, its source"},
		{"placement whose origin has too few components", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : CoordinateFramePlacement { :>> source = datum; :>> origin = VectorFunctions::VectorOf((1.0, 2.0)) [mm]; } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrMultiplicityViolation, "origin has 2 components over datum of 3 axes"},
		{"placement of a singular basis", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : CoordinateFramePlacement { :>> source = datum; :>> origin = (0.0, 0.0, 0.0) [datum]; :>> basisDirections = ((1.0, 0.0, 0.0) [datum], (2.0, 0.0, 0.0) [datum], (0.0, 0.0, 1.0) [datum]); } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", semantics.ErrArithmeticDomain, "basis directions are linearly dependent and span no frame"},
		{"placement of a zero basis direction", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : CoordinateFramePlacement { :>> source = datum; :>> origin = (0.0, 0.0, 0.0) [datum]; :>> basisDirections = ((0.0, 0.0, 0.0) [datum], (0.0, 1.0, 0.0) [datum], (0.0, 0.0, 1.0) [datum]); } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", semantics.ErrArithmeticDomain, "basisDirections#(1) is the zero vector, which points nowhere"},
		{"placement of too few basis directions", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : CoordinateFramePlacement { :>> source = datum; :>> origin = (0.0, 0.0, 0.0) [datum]; :>> basisDirections = ((1.0, 0.0, 0.0) [datum], (0.0, 1.0, 0.0) [datum]); } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrMultiplicityViolation, "states 2 basisDirections over datum of 3 axes; a placement states none or one per axis"},
		{"placement reorienting axes in different units", `
			attribute mixed : CoordinateFrame { :>> mRefs = (m, mm, m); }
			attribute odd : CoordinateFrame { :>> mRefs = (m, mm, m); :>> transformation : CoordinateFramePlacement { :>> source = mixed; :>> origin = (0.0, 0.0, 0.0) [mixed]; :>> basisDirections = ((0.0, 1.0, 0.0) [mixed], (1.0, 0.0, 0.0) [mixed], (0.0, 0.0, 1.0) [mixed]); } }`,
			"Quantities::VectorQuantityValue", "transform(odd.transformation, (1.0, 2.0, 3.0) [mixed])", ErrIncommensurableUnits, "reorients mixed, whose axes m and mm are in different units and cannot mix"},
		{"translation in an incommensurable unit", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : TranslationRotationSequence { :>> source = datum; :>> elements = (new Translation(VectorFunctions::VectorOf((1.0, 0.0, 0.0)) [s])); } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrIncommensurableUnits, "elements#(1).translationVector: incommensurable units: cannot express s (second) in mm (0.001·metre)"},
		{"translation in another frame of the same units", `
			attribute other : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); }
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : TranslationRotationSequence { :>> source = datum; :>> elements = (new Translation((1.0, 0.0, 0.0) [other])); } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrTypeMismatch, "translationVector is a vector quantity in other, not a vector quantity over datum, its source"},
		{"translation in a commensurable unit converts", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : TranslationRotationSequence { :>> source = datum; :>> elements = (new Translation(VectorFunctions::VectorOf((1.0, 0.0, 0.0)) [m])); } }`,
			"Position3dVector", "transform(odd.transformation, (1000.0, 2.0, 3.0) [datum])", nil, ""},
		{"sequence of no elements", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : TranslationRotationSequence { :>> source = datum; } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrNoValue, "states no elements; TranslationRotationSequence declares elements: TranslationOrRotation[1..*]"},
		{"sequence whose element is a number", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : TranslationRotationSequence { :>> source = datum; :>> elements = (1, 2); } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrTypeMismatch, "transformation.elements: type mismatch: cannot write 1 (an Integer) to a feature typed by TranslationOrRotation"},
		{"rotation about the zero vector", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : TranslationRotationSequence { :>> source = datum; :>> elements = (new Rotation((0.0, 0.0, 0.0) [datum], 90 ['°'])); } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", semantics.ErrArithmeticDomain, "axisDirection is the zero vector, which points nowhere"},
		{"rotation by a length", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : TranslationRotationSequence { :>> source = datum; :>> elements = (new Rotation((0.0, 0.0, 1.0) [datum], 90 [mm])); } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrTypeMismatch, "new Rotation: feature value Rotation.angle: type mismatch: cannot write 90 [mm] (dimension L) to a feature typed by AngularMeasureValue (dimensionless)"},
		{"rotation by a number", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : TranslationRotationSequence { :>> source = datum; :>> elements = (new Rotation((0.0, 0.0, 1.0) [datum], 90)); } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrTypeMismatch, "new Rotation: feature value Rotation.angle: type mismatch: cannot write 90 (an Integer) to a feature typed by AngularMeasureValue"},
		{"rotation of a plane frame", `
			attribute plane : CoordinateFrame { :>> mRefs = (mm, mm); }
			attribute odd : CoordinateFrame { :>> mRefs = (mm, mm); :>> transformation : TranslationRotationSequence { :>> source = plane; :>> elements = (new Rotation((0.0, 1.0) [plane], 90 ['°'])); } }`,
			"Quantities::VectorQuantityValue", "transform(odd.transformation, (1.0, 2.0) [plane])", errUnevaluableLibraryFunction, "rotates about an axis, which a frame of 2 axes has no meaning for"},
		{"affine matrix of too few elements", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : AffineTransformationMatrix3d { :>> source = datum; :>> rotationMatrix { :>> elements = (1.0, 0.0, 0.0, 0.0, 1.0, 0.0); } :>> translationVector { :>> elements = (0.0, 0.0, 0.0); } } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrMultiplicityViolation, "rotationMatrix.elements: multiplicity violation: 6 value(s) bound to a feature with multiplicity lower bound 9"},
		{"affine matrix of strings", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : AffineTransformationMatrix3d { :>> source = datum; :>> rotationMatrix { :>> elements = ("a", "b", "c", "d", "e", "f", "g", "h", "i"); } :>> translationVector { :>> elements = (0.0, 0.0, 0.0); } } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrTypeMismatch, `rotationMatrix.elements: type mismatch: cannot write "a" (string) to a feature typed by Real`},
		{"affine matrix with no translation", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : AffineTransformationMatrix3d { :>> source = datum; :>> rotationMatrix { :>> elements = (1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0); } } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrNoValue, "translationVector states no elements"},
		{"affine matrix over a plane frame", `
			attribute plane : CoordinateFrame { :>> mRefs = (mm, mm); }
			attribute odd : CoordinateFrame { :>> mRefs = (mm, mm); :>> transformation : AffineTransformationMatrix3d { :>> source = plane; :>> rotationMatrix { :>> elements = (1.0, 0.0, 0.0, 0.0, 1.0, 0.0, 0.0, 0.0, 1.0); } :>> translationVector { :>> elements = (0.0, 0.0, 0.0); } } }`,
			"Quantities::VectorQuantityValue", "transform(odd.transformation, (1.0, 2.0) [plane])", ErrMultiplicityViolation, "is an AffineTransformationMatrix3d over plane of 2 axes; it asserts source.dimensions == 3"},
		{"transformation with no source", `
			attribute lone : NullTransformation { :>> target = datum; }`,
			"Position3dVector", "transform(lone, (1.0, 2.0, 3.0) [datum])", ErrNoValue, "lone states no source or no target"},
		{"transformation with no target", `
			attribute lone : NullTransformation { :>> source = datum; }`,
			"Position3dVector", "transform(lone, (1.0, 2.0, 3.0) [datum])", ErrNoValue, "lone states no source or no target"},
		{"transformation between frames of different dimensions", `
			attribute plane : CoordinateFrame { :>> mRefs = (mm, mm); }
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : NullTransformation { :>> source = plane; } }`,
			"Quantities::VectorQuantityValue", "transform(odd.transformation, (1.0, 2.0) [plane])", ErrMultiplicityViolation, "relates plane of dimensions [2] (2 axes) to odd of dimensions [3] (3 axes); CoordinateTransformation asserts source.dimensions == target.dimensions"},
		{"transformation between frames of one axis but different dimensions", `
			attribute lineA : CoordinateFrame { :>> dimensions = 1; :>> mRefs = mm; }
			attribute lineB : CoordinateFrame { :>> dimensions = (); :>> mRefs = mm; :>> transformation : NullTransformation { :>> source = lineA; } }`,
			"Quantities::VectorQuantityValue", "transform(lineB.transformation, 3.0 [lineA])", ErrMultiplicityViolation, "relates lineA of dimensions [1] (1 axes) to lineB of dimensions [] (1 axes); CoordinateTransformation asserts source.dimensions == target.dimensions"},
		{"transformation whose target is another frame", `
			attribute odd : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); :>> transformation : NullTransformation { :>> source = datum; :>> target = datum; } }`,
			"Position3dVector", "transform(odd.transformation, (1.0, 2.0, 3.0) [datum])", ErrTypeMismatch, "target is datum, but the transformation is odd's own, whose target it is"},
		{"scale whose placement and mapping disagree", `
			attribute def Muddled :> IntervalScale {
				:>> unit = SI::'°C';
				private attribute triplePoint : DefinitionalQuantityValue { :>> num = 0.01; :>> definition = "triple point"; }
				private attribute mapping : QuantityValueMapping { :>> mappedQuantityValue = triplePoint; :>> referenceQuantityValue = K.temperatureOfWaterAtTriplePointInK; }
				:>> quantityValueMapping = mapping;
				:>> transformation : CoordinateFramePlacement { :>> source = K; :>> origin = 300.0 [K]; }
			}
			attribute muddled : Muddled;`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(300.0 [K], muddled)", errUnevaluableLibraryFunction, "its transformation places zero at 300.0 [K] but its quantityValueMapping places it at 273.15"},
		{"scale whose mapping maps a quantity, not a definitional value", `
			attribute def Muddled :> IntervalScale {
				:>> unit = SI::'°C';
				private attribute mapping : QuantityValueMapping { :>> mappedQuantityValue = 0.01 [SI::'°C']; :>> referenceQuantityValue = K.temperatureOfWaterAtTriplePointInK; }
				:>> quantityValueMapping = mapping;
			}
			attribute muddled : Muddled;`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(300.0 [K], muddled)", ErrTypeMismatch, "measurement scale muddled: quantityValueMapping.mappedQuantityValue is a quantity in SI::'°C', want a DefinitionalQuantityValue"},
		{"scale placed on a reference of another dimension", `
			attribute def Muddled :> IntervalScale { :>> unit = SI::'°C'; :>> transformation : CoordinateFramePlacement { :>> source = m; :>> origin = 1.0 [m]; } }
			attribute muddled : Muddled;`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(300.0 [K], muddled)", ErrIncommensurableUnits, ""},
		{"scale whose origin is a string", `
			attribute def Muddled :> Time::TimeScale { :>> unit = s; :>> transformation : CoordinateFramePlacement { :>> source = Time::UTC; :>> origin = "2024-01-01T00:00:00Z"; } }
			attribute muddled : Muddled;`,
			"ISQ::DurationValue", "ConvertQuantity(3.0 [Time::UTC], muddled)", ErrTypeMismatch, `transformation.origin: type mismatch: cannot write "2024-01-01T00:00:00Z" (string) to a feature typed by VectorQuantityValue`},
		{"scale whose one basis direction is the identity in another unit", `
			attribute shifted : IntervalScale { :>> unit = m; :>> transformation : CoordinateFramePlacement { :>> source = m; :>> origin = 10.0 [m]; :>> basisDirections = 1000.0 [mm]; } }`,
			"LengthValue", "ConvertQuantity(3.0 [shifted], m)", nil, ""},
		{"scale whose basis direction scales the axis in another unit", `
			attribute shifted : IntervalScale { :>> unit = s; :>> transformation : CoordinateFramePlacement { :>> source = s; :>> origin = 10.0 [s]; :>> basisDirections = 1.0 [min]; } }`,
			"DurationValue", "ConvertQuantity(3.0 [shifted], s)", errUnevaluableLibraryFunction, "the basisDirection 1.0 [min] of its transformation transformation is not the identity 1 [s]"},
		{"scale placed on a frame of two axes", `
			attribute plane : CoordinateFrame { :>> mRefs = (K, K); }
			attribute shifted : IntervalScale { :>> unit = K; :>> transformation : CoordinateFramePlacement { :>> source = plane; :>> origin = 10.0 [K]; } }`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(3.0 [shifted], K)", ErrMultiplicityViolation, "relates plane of dimensions [2] (2 axes) to shifted of dimensions [] (1 axes); CoordinateTransformation asserts source.dimensions == target.dimensions"},
		{"scale with two basis directions", `
			attribute shifted : IntervalScale { :>> unit = K; :>> transformation : CoordinateFramePlacement { :>> source = K; :>> origin = 10.0 [K]; :>> basisDirections = (1.0 [K], 1.0 [K]); } }`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(3.0 [shifted], K)", ErrMultiplicityViolation, "states 2 basisDirections over K of one axis"},
		{"scale whose basis direction is in an incommensurable unit", `
			attribute shifted : IntervalScale { :>> unit = K; :>> transformation : CoordinateFramePlacement { :>> source = K; :>> origin = 10.0 [K]; :>> basisDirections = 1.0 [m]; } }`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(3.0 [shifted], K)", ErrIncommensurableUnits, "the basisDirection 1.0 [m] of its transformation transformation is not on K, its source"},
		{"scale whose basis direction is over another frame", `
			attribute line : CoordinateFrame { :>> mRefs = (K); }
			attribute shifted : IntervalScale { :>> unit = K; :>> transformation : CoordinateFramePlacement { :>> source = K; :>> origin = 10.0 [K]; :>> basisDirections = (1.0) [line]; } }`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(3.0 [shifted], K)", ErrTypeMismatch, "is a vector quantity in line, not a quantity on K, its source"},
		{"scale whose basis direction scales the axis", `
			attribute shifted : IntervalScale { :>> unit = K; :>> transformation : CoordinateFramePlacement { :>> source = K; :>> origin = 10.0 [K]; :>> basisDirections = 2.0 [K]; } }`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(3.0 [shifted], K)", errUnevaluableLibraryFunction, "is not the identity 1 [K], and the library gives a scale no other basis"},
		{"scale with no unit", `
			attribute def Unitless :> IntervalScale;
			attribute unitless : Unitless;`,
			"IntervalScale", "unitless", ErrNoValue, "states no unit; MeasurementScale declares unit: MeasurementUnit"},
		{"scale placed on itself", `
			attribute loop : IntervalScale { :>> unit = K; :>> transformation : CoordinateFramePlacement { :>> source = loop; :>> origin = 1.0 [K]; } }`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(3.0 [loop], K)", ErrCyclicFeatureValue, "coordinate frame loop is defined in terms of itself"},
		{"scales placed on each other", `
			attribute a : IntervalScale { :>> unit = K; :>> transformation : CoordinateFramePlacement { :>> source = b; :>> origin = 1.0 [b]; } }
			attribute b : IntervalScale { :>> unit = K; :>> transformation : CoordinateFramePlacement { :>> source = a; :>> origin = 2.0 [a]; } }`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(3.0 [a], K)", ErrCyclicFeatureValue, "coordinate frame a is defined in terms of itself"},
		{"scales mapped onto each other", `
			attribute a : IntervalScale {
				:>> unit = K;
				private attribute p : DefinitionalQuantityValue { :>> num = 1; :>> definition = "p"; }
				private attribute m : QuantityValueMapping { :>> mappedQuantityValue = p; :>> referenceQuantityValue = 2.0 [b]; }
				:>> quantityValueMapping = m;
			}
			attribute b : IntervalScale {
				:>> unit = K;
				private attribute p : DefinitionalQuantityValue { :>> num = 1; :>> definition = "p"; }
				private attribute m : QuantityValueMapping { :>> mappedQuantityValue = p; :>> referenceQuantityValue = 2.0 [a]; }
				:>> quantityValueMapping = m;
			}`,
			"ThermodynamicTemperatureValue", "ConvertQuantity(3.0 [K], a)", ErrCyclicFeatureValue, "coordinate frame a is defined in terms of itself"},
		// A point on an interval scale has the affine operations and no other: the
		// refusal names the operation and the scale.
		{"point added to a point", ``,
			"ThermodynamicTemperatureValue", "warm + 10.0 [SI::'°C_abs']", errScalePoint, "operator '+' on a point of the interval scale SI::'°C_abs': two points have no sum"},
		{"time instant added to a time instant", ``,
			"Time::TimeInstantValue", "5.0 [Time::UTC] + 3.0 [Time::UTC]", errScalePoint, "operator '+' on a point of the interval scale Time::UTC: two points have no sum; their difference `x - y` is a magnitude in s"},
		{"point taken from a magnitude", ``,
			"ThermodynamicTemperatureValue", "300.0 [K] - warm", errScalePoint, "subtracting a point from a magnitude on a point of the interval scale SI::'°C_abs'"},
		{"point scaled by a number", ``,
			"ThermodynamicTemperatureValue", "2 * warm", errScalePoint, "operator '*' on a point of the interval scale SI::'°C_abs': a point has no multiple"},
		{"number scaled by a point", ``,
			"ThermodynamicTemperatureValue", "warm * 2.0", errScalePoint, "operator '*' on a point of the interval scale SI::'°C_abs'"},
		{"time instant scaled by a number", ``,
			"Time::TimeInstantValue", "2 * 5.0 [Time::UTC]", errScalePoint, "operator '*' on a point of the interval scale Time::UTC"},
		{"point multiplied by a quantity", ``,
			"ScalarQuantityValue", "warm * 2.0 [s]", errScalePoint, "operator '*' on a point of the interval scale SI::'°C_abs': a point has no multiple and its scale is no unit to compose"},
		{"quantity multiplied by a point", ``,
			"ScalarQuantityValue", "2.0 [s] * warm", errScalePoint, "operator '*' on a point of the interval scale SI::'°C_abs'"},
		{"point divided by a number", ``,
			"ThermodynamicTemperatureValue", "warm / 2", errScalePoint, "operator '/' on a point of the interval scale SI::'°C_abs'"},
		{"point divided by a quantity", ``,
			"ScalarQuantityValue", "warm / 2.0 [s]", errScalePoint, "operator '/' on a point of the interval scale SI::'°C_abs'"},
		{"quantity divided by a point", ``,
			"ScalarQuantityValue", "2.0 [s] / warm", errScalePoint, "operator '/' on a point of the interval scale SI::'°C_abs'"},
		{"point raised to a power", ``,
			"ScalarQuantityValue", "warm ** 2", errScalePoint, "operator '**' on a point of the interval scale SI::'°C_abs'"},
		{"point raised to a power by exponentiation", ``,
			"ScalarQuantityValue", "warm ^ 2", errScalePoint, "operator '**' on a point of the interval scale SI::'°C_abs'"},
		{"square root of a point", ``,
			"ScalarQuantityValue", "sqrt(warm)", errScalePoint, "sqrt on a point of the interval scale SI::'°C_abs'"},
		{"negated point", ``,
			"ThermodynamicTemperatureValue", "-warm", errScalePoint, "negation on a point of the interval scale SI::'°C_abs': a point has no negative"},
		{"points summed", ``,
			"ThermodynamicTemperatureValue", "sum((warm, 20.0 [SI::'°C_abs']))", errScalePoint, "QuantityCalculations::sum on a point of the interval scale SI::'°C_abs': points have no sum or product to fold"},
		{"points multiplied together", ``,
			"ScalarQuantityValue", "product((warm, 20.0 [SI::'°C_abs']))", errScalePoint, "QuantityCalculations::product on a point of the interval scale SI::'°C_abs'"},
		{"point moved by a magnitude of another dimension", ``,
			"ThermodynamicTemperatureValue", "warm + 1.0 [m]", ErrIncommensurableUnits, "a point on the interval scale SI::'°C_abs' moves by a magnitude in its unit '°C'"},
		{"point compared with a magnitude of another dimension", ``,
			"Boolean", "warm < 1.0 [m]", ErrIncommensurableUnits, ""},
		{"point equated with a magnitude of another dimension", ``,
			"Boolean", "warm == 1.0 [m]", ErrIncommensurableUnits, ""},
		{"point on an unanchored scale compared with a magnitude", ``,
			"Boolean", "5.0 [Time::UTC] == 5.0 [s]", errUnevaluableLibraryFunction, "Time::UTC states neither a transformation placing it on another reference nor a quantityValueMapping"},
		{"point on an ordinal scale moved by a magnitude", `attribute mohs : OrdinalScale { :>> unit = K; }`,
			"ScalarQuantityValue", "7 [mohs] + 1 [K]", errScalePoint, "operator '+' on a point of the ordinal scale test::Holder::mohs: only an IntervalScale relates its points by differences in its unit"},
		{"points on an ordinal scale subtracted", `attribute mohs : OrdinalScale { :>> unit = K; }`,
			"ScalarQuantityValue", "9 [mohs] - 7 [mohs]", errScalePoint, "operator '-' on a point of the ordinal scale test::Holder::mohs"},
		{"point on an ordinal scale compared with a magnitude", `attribute mohs : OrdinalScale { :>> unit = K; }`,
			"Boolean", "7 [mohs] < 300.0 [K]", errScalePoint, "comparison with another reference on a point of the ordinal scale test::Holder::mohs"},
		{"point on a cyclic ratio scale scaled", `attribute compass : CyclicRatioScale { :>> unit = rad; :>> modulus = 6.283185307179586; }`,
			"ScalarQuantityValue", "2 * 1.0 [compass]", errScalePoint, "operator '*' on a point of the cyclic ratio scale test::Holder::compass"},
		{"point on a logarithmic scale moved by a magnitude", `attribute decibel : LogarithmicScale { :>> unit = W; :>> logarithmBase = 10; :>> factor = 10; :>> exponent = 1; }`,
			"ScalarQuantityValue", "30.0 [decibel] + 1.0 [W]", errScalePoint, "operator '+' on a point of the logarithmic scale test::Holder::decibel: only an IntervalScale relates its points by differences in its unit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := fmt.Sprintf(`
				package test {
					private import ISQ::*;
					private import SI::*;
					private import ISQSpaceTime::*;
					private import MeasurementReferences::*;
					private import QuantityCalculations::*;
					private import VectorCalculations::*;
					part def Holder {
						attribute spatialCF : CartesianSpatial3dCoordinateFrame { :>> mRefs = (m, m, m); }
						attribute datum : CartesianSpatial3dCoordinateFrame { :>> mRefs = (mm, mm, mm); }
						attribute warm : ThermodynamicTemperatureValue = ConvertQuantity(300.0 [K], SI::'°C_abs');
						%s
						attribute value : %s = %s;
					}
				}
			`, tc.body, tc.declared, tc.value)
			idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, src))
			sym := findSymbolByName(idx.DocumentRoot("<test>"), "Holder", ast.DefPart)
			if sym == nil {
				t.Fatal("part def Holder not found")
			}
			inst, err := ctx.Instantiate(sym)
			if err != nil {
				t.Fatalf("Instantiate err = %v", err)
			}
			_, err = inst.GetFeatureValue(ctx, "value")
			if tc.want == nil {
				if err != nil {
					t.Fatalf("value err = %v, want a value", err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("value err = %v, want %v", err, tc.want)
			}
			if !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("value err = %v, want it to name %q", err, tc.message)
			}
		})
	}
}

// testQuantityIncommensurableComparison: comparing quantities whose units
// measure different things reports ErrIncommensurableUnits instead of comparing
// the bare magnitudes, which would make 1.5 [m/s] <= 2.0 [s] true.
func testQuantityIncommensurableComparison(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import SI::*;
			requirement def Touchdown {
				attribute speed = 1.5 [m/s];
				attribute duration = 2.0 [s];
				require constraint { speed <= duration }
			}
		}
	`))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Touchdown", ast.DefRequirement)
	if sym == nil {
		t.Fatal("Touchdown requirement not found")
	}

	satisfied, err := ctx.EvaluateRequirement(sym, rootScope)
	if !errors.Is(err, ErrIncommensurableUnits) {
		t.Fatalf("satisfied = %v, err = %v; want ErrIncommensurableUnits", satisfied, err)
	}
	if errors.Is(err, ErrViolated) {
		t.Error("incommensurable units are not a violation: neither verdict is an answer")
	}
}

// testQuantityIndexIsNotAUnit: a bracketed expression whose index names
// something that is not a measurement unit reports errNotAQuantity rather than
// evaluating to the bare magnitude.
func testQuantityIndexIsNotAUnit(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import SI::*;
			attribute notAUnit = 3.0;
			constraint bogus {
				1.5 [test::notAUnit] <= 2.0 [m]
			}
		}
	`))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "bogus", ast.DefConstraint)
	if sym == nil {
		t.Fatal("bogus constraint not found")
	}

	satisfied, err := ctx.EvaluateConstraint(sym, rootScope)
	if !errors.Is(err, errNotAQuantity) {
		t.Fatalf("satisfied = %v, err = %v; want errNotAQuantity", satisfied, err)
	}
	if !strings.Contains(err.Error(), semantics.ErrNotAUnit.Error()) {
		t.Errorf("err = %v; want it to report that the index names no measurement unit", err)
	}
}

// testQuantityUnitShadowedBySibling: a unit position naming a sibling that is
// not a measurement unit reports which declaration it resolved to and the unit
// that declaration hid, rather than a magnitude in the wrong unit.
func testQuantityUnitShadowedBySibling(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import SI::*;
			constraint def Tall {
				attribute m : ScalarValues::Real = 2.0;
				1.0 [m] > 500.0 [m]
			}
		}
	`))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Tall", ast.DefConstraint)
	if sym == nil {
		t.Fatal("Tall constraint not found")
	}

	satisfied, err := ctx.EvaluateConstraint(sym, sym.OwnerScope)
	if !errors.Is(err, errNotAQuantity) {
		t.Fatalf("satisfied = %v, err = %v; want errNotAQuantity", satisfied, err)
	}
	if !errors.Is(err, semantics.ErrNotAUnit) {
		t.Errorf("err = %v; want it to report that the name is no measurement unit", err)
	}
	var shadowed *semantics.ShadowedUnitError
	if !errors.As(err, &shadowed) {
		t.Fatalf("err = %v; want a *semantics.ShadowedUnitError", err)
	}
	if shadowed.Resolved == nil || shadowed.Namespace != "test::Tall" {
		t.Errorf("error names %v in %q; want the sibling declared in test::Tall", shadowed.Resolved, shadowed.Namespace)
	}
	if shadowed.Shadowed == nil || shadowed.Suggestion != "SI::m" {
		t.Errorf("error suggests %q; want the qualified spelling SI::m of the hidden unit", shadowed.Suggestion)
	}
}

// testQuantityQualifiedUnitIsNotShadowing: a qualified name in unit position
// resolves to what it names, so a non-unit is reported as one without a
// shadowing explanation or a spelling that would not resolve.
func testQuantityQualifiedUnitIsNotShadowing(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import SI::*;
			attribute m : ScalarValues::Real = 2.0;
			constraint def Tall {
				1.0 [test::m] > 500.0 [SI::m]
			}
		}
	`))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "Tall", ast.DefConstraint)
	if sym == nil {
		t.Fatal("Tall constraint not found")
	}

	satisfied, err := ctx.EvaluateConstraint(sym, sym.OwnerScope)
	if !errors.Is(err, semantics.ErrNotAUnit) {
		t.Fatalf("satisfied = %v, err = %v; want ErrNotAUnit", satisfied, err)
	}
	var shadowed *semantics.ShadowedUnitError
	if !errors.As(err, &shadowed) {
		t.Fatalf("err = %v; want a *semantics.ShadowedUnitError", err)
	}
	if shadowed.Shadowed != nil || shadowed.Suggestion != "" {
		t.Errorf("error suggests %q for a qualified name; want no shadowing explanation", shadowed.Suggestion)
	}
	if !strings.Contains(err.Error(), "test::m resolves to") {
		t.Errorf("err = %v; want it to name the declaration as written", err)
	}
}

// testQuantityShadowedUnitWithoutAQualifier: a hidden unit owned by no namespace
// has no qualified spelling to offer, so the diagnostic names it without
// advising the name that just failed.
func testQuantityShadowedUnitWithoutAQualifier(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		attribute u : ISQBase::LengthUnit = SI::m;
		package test {
			attribute u : ScalarValues::Real = 2.0;
			constraint def Tall {
				1.0 [u] > 0.5 [SI::m]
			}
		}
	`))
	sym := findSymbolByName(idx.DocumentRoot("<test>"), "Tall", ast.DefConstraint)
	if sym == nil {
		t.Fatal("Tall constraint not found")
	}

	satisfied, err := ctx.EvaluateConstraint(sym, sym.OwnerScope)
	if !errors.Is(err, semantics.ErrNotAUnit) {
		t.Fatalf("satisfied = %v, err = %v; want ErrNotAUnit", satisfied, err)
	}
	var shadowed *semantics.ShadowedUnitError
	if !errors.As(err, &shadowed) {
		t.Fatalf("err = %v; want a *semantics.ShadowedUnitError", err)
	}
	if shadowed.Shadowed == nil {
		t.Fatalf("err = %v; want it to name the unit the declaration hid", err)
	}
	if shadowed.Suggestion != "" {
		t.Errorf("error suggests %q; want no spelling when none qualifies the unit", shadowed.Suggestion)
	}
	if strings.Contains(err.Error(), "write u") {
		t.Errorf("err = %v; want it not to advise the name that failed", err)
	}
}

// testQuantityCyclicUnitDefinition: two units defined in terms of each other are
// reported as a cycle instead of recursing until the stack or step budget runs
// out.
func testQuantityCyclicUnitDefinition(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, `
		package test {
			public import SI::*;
			attribute unitA : ISQBase::LengthUnit = unitB;
			attribute unitB : ISQBase::LengthUnit = unitA;
			constraint cyclic {
				1.0 [test::unitA] <= 2.0 [test::unitA]
			}
		}
	`))
	rootScope := idx.DocumentRoot("<test>")
	sym := findSymbolByName(rootScope, "cyclic", ast.DefConstraint)
	if sym == nil {
		t.Fatal("cyclic constraint not found")
	}

	done := make(chan error, 1)
	go func() {
		_, err := ctx.EvaluateConstraint(sym, rootScope)
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, semantics.ErrUnitCycle) {
			t.Fatalf("err = %v; want ErrUnitCycle", err)
		}
	case <-watchdog(10 * time.Second):
		t.Fatal("evaluating a cyclic unit definition did not terminate")
	}
}
