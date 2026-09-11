package passes

import "testing"

// dimensionDiags reports the type-tier diagnostics of a model written against
// the bundled quantity libraries.
func dimensionDiags(t *testing.T, body string) []Diagnostic {
	t.Helper()
	return libraryTypeDiags(t, "package P {\n"+
		"private import ISQ::*;\nprivate import SI::*;\n"+body+"\n}")
}

func wantOneDimensionError(t *testing.T, body, want string) {
	t.Helper()
	diags := dimensionDiags(t, body)
	if len(diags) != 1 {
		t.Fatalf("want exactly one type diagnostic, got %v", diags)
	}
	if diags[0].Severity != SeverityError {
		t.Errorf("diagnostic is %v, want an error: %s", diags[0].Severity, diags[0].Message)
	}
	if diags[0].Message != want {
		t.Errorf("message = %q, want %q", diags[0].Message, want)
	}
}

func wantNoDimensionDiags(t *testing.T, body string) {
	t.Helper()
	if diags := dimensionDiags(t, body); len(diags) != 0 {
		t.Fatalf("want no type diagnostics, got %v", diags)
	}
}

// TestBoundQuantityOfAnotherDimension: the initial value's unit measures in a
// dimension the target's quantity value type does not, and the message names
// both.
func TestBoundQuantityOfAnotherDimension(t *testing.T) {
	wantOneDimensionError(t, `attribute bad : DurationValue = 5 [m];`,
		"cannot bind m (dimension L) to a feature typed by DurationValue (dimension T)")
}

// TestBoundNestedCollectionQuantityOfAnotherDimension: a collection binds flat,
// so a unit inside a nested literal is measured against the target too.
func TestBoundNestedCollectionQuantityOfAnotherDimension(t *testing.T) {
	wantOneDimensionError(t, `attribute ls : LengthValue[*] = (1 [m], (2 [m], 3 [s]));`,
		"cannot bind s (dimension T) to a feature typed by LengthValue (dimension L)")
	wantNoDimensionDiags(t, `attribute ls : LengthValue[*] = (1 [m], (2 [m], 3 [mm]));`)
}

// TestBoundCollectionQuantityOfAnotherDimension: a collection value binds each element its
// body produces, or keeps, so a unit written in the body is measured against the target too.
func TestBoundCollectionQuantityOfAnotherDimension(t *testing.T) {
	const parts = `private import ControlFunctions::*; part def Part; part parts : Part[*];`
	wantOneDimensionError(t, parts+`attribute t : DurationValue = parts.{ in p : Part; 5 [m] };`,
		"cannot bind m (dimension L) to a feature typed by DurationValue (dimension T)")
	wantOneDimensionError(t, parts+`attribute t : DurationValue = parts->collect { in p : Part; 5 [m] };`,
		"cannot bind m (dimension L) to a feature typed by DurationValue (dimension T)")
	wantOneDimensionError(t, parts+`attribute t : DurationValue = parts.{ in p : Part; (5 [s], 5 [m]) };`,
		"cannot bind m (dimension L) to a feature typed by DurationValue (dimension T)")
	wantOneDimensionError(t, parts+`attribute t : DurationValue = (5 [m], 6 [s]).?{ in l : LengthValue; true };`,
		"cannot bind m (dimension L) to a feature typed by DurationValue (dimension T)")
	wantOneDimensionError(t, parts+`attribute t : DurationValue = (5 [s], 6 [m])->select { in l : LengthValue; true };`,
		"cannot bind m (dimension L) to a feature typed by DurationValue (dimension T)")
	wantOneDimensionError(t, parts+`attribute t : DurationValue = (5 [m])->reduce { in a : LengthValue; in b : LengthValue; 1 [s] };`,
		"cannot bind m (dimension L) to a feature typed by DurationValue (dimension T)")
	wantNoDimensionDiags(t, parts+`
		attribute t : DurationValue = parts.{ in p : Part; 5 [min] };
		attribute t2 : DurationValue = parts->collect { in p : Part; (5 [s], 5 [min]) };
		attribute t3 : DurationValue = (5 [s], 6 [min]).?{ in d : DurationValue; true };
		attribute t4 : DurationValue = (5 [s], 6 [min])->reduce { in a : DurationValue; in b : DurationValue; 1 [min] };
		attribute l : LengthValue = parts.{ in p : Part; 5 [m] };`)
}

// A body mapping every element to a quantity held by no value, or an operation over such a
// collection, binds no quantity, so no dimension is measured against the target.
func TestBoundCollectionQuantityOfNothing(t *testing.T) {
	const parts = `private import ControlFunctions::*; part def Part { attribute none : LengthValue[0]; } part parts : Part[*];`
	wantNoDimensionDiags(t, parts+`
		attribute t : DurationValue = parts.{ in p : Part; p.none };
		attribute t2 : DurationValue = parts->collect { in p : Part; (p.none, p.none) };
		attribute t3 : DurationValue = (parts.{ in p : Part; p.none }).{ in l : LengthValue; 5 [m] };
		attribute t4 : DurationValue = (parts.{ in p : Part; p.none }).?{ in l : LengthValue; true };
		attribute t5 : DurationValue = (parts.{ in p : Part; p.none })->reduce { in a : LengthValue; in b : LengthValue; 1 [m] };`)
	wantOneDimensionError(t, parts+`attribute t : DurationValue = parts.{ in p : Part; (p.none, 5 [m]) };`,
		"cannot bind m (dimension L) to a feature typed by DurationValue (dimension T)")
}

// TestBoundQuantityOfTheSameDimensionAtAnotherScale: a dimension has no scale,
// so any unit measuring in it conforms.
func TestBoundQuantityOfTheSameDimensionAtAnotherScale(t *testing.T) {
	wantNoDimensionDiags(t, `attribute t : DurationValue = 5 [min];`)
}

// TestBoundQuantityComposedToItsDimension: a composed unit is judged by the
// dimension it reduces to, not by how it was written.
func TestBoundQuantityComposedToItsDimension(t *testing.T) {
	wantNoDimensionDiags(t, `attribute v : SpeedValue = 10 [m] / 2 [s];`)
	wantOneDimensionError(t, `attribute t : DurationValue = 10 [m] / 2 [s];`,
		"cannot bind a value of dimension L·T^-1 to a feature typed by DurationValue (dimension T)")
}

// TestAssignedQuantityOfAnotherDimension: an assignment is judged where a bound
// value is, so the assign walker reports the same clash.
func TestAssignedQuantityOfAnotherDimension(t *testing.T) {
	wantOneDimensionError(t, `part def Host {
		attribute wrong : DurationValue = 0 [s];
		perform action b { action step { assign wrong := 3 [m/s]; } first step; }
	}`, "cannot bind m/s (dimension L·T^-1) to a feature typed by DurationValue (dimension T)")
}

// TestAssignedQuantityOfTheSameDimension: a commensurable assignment stands.
func TestAssignedQuantityOfTheSameDimension(t *testing.T) {
	wantNoDimensionDiags(t, `part def Host {
		attribute t : DurationValue = 0 [s];
		perform action b { action step { assign t := 3 [min]; } first step; }
	}`)
}

// TestBoundQuantityThroughAnAlias: a quantity kind named through an alias fixes
// the dimension the aliased type does.
func TestBoundQuantityThroughAnAlias(t *testing.T) {
	wantNoDimensionDiags(t, `attribute k : ISQ::TemperatureValue = 5 [K];`)
	wantOneDimensionError(t, `attribute k : ISQ::TemperatureValue = 5 [m];`,
		"cannot bind m (dimension L) to a feature typed by ThermodynamicTemperatureValue (dimension Θ)")
}

// TestBoundQuantityWhereTheTargetFixesNoDimension: a target that declares no
// quantity kind, and one whose measurement reference is any unit, state nothing
// a value must answer to.
func TestBoundQuantityWhereTheTargetFixesNoDimension(t *testing.T) {
	wantNoDimensionDiags(t, `attribute r : ScalarValues::Real = 5;`)
	wantNoDimensionDiags(t, `attribute q : Quantities::ScalarQuantityValue = 5 [m];`)
	wantNoDimensionDiags(t, `attribute u = 5 [m];`)
}

// TestBoundBareNumberStatesNoDimension: a plain number names no unit, so it
// determines no dimension and the dimensional rule stays silent — the type
// conformance it already answered to is unchanged.
func TestBoundBareNumberStatesNoDimension(t *testing.T) {
	wantOneDimensionError(t, `attribute t : DurationValue = 5;`,
		"cannot bind Natural value to a feature typed by DurationValue")
	wantNoDimensionDiags(t, `attribute r : ScalarValues::Real = 5;`)
}

// TestBoundDimensionlessQuantity: DimensionOneValue fixes the dimension of a
// count, so a metre is refused there as a second is, and a ratio of lengths —
// which measures in it — is not.
func TestBoundDimensionlessQuantity(t *testing.T) {
	wantOneDimensionError(t, `attribute n : MeasurementReferences::DimensionOneValue = 5 [m];`,
		"cannot bind m (dimension L) to a feature typed by DimensionOneValue (dimensionless)")
	wantNoDimensionDiags(t, `attribute n : MeasurementReferences::DimensionOneValue = 10 [m] / 5 [m];`)
	wantOneDimensionError(t, `attribute t : DurationValue = 10 [m] / 5 [m];`,
		"cannot bind a dimensionless value to a feature typed by DurationValue (dimension T)")
}

// TestRecursiveRollupThroughACall: a feature whose value calls a function on a
// chain back to itself — the mass rollup `mass + sum(subcomponents.totalMass)` —
// types in finite time: the argument is left untyped where it leads back to the
// call being typed, rather than typing the call again.
func TestRecursiveRollupThroughACall(t *testing.T) {
	wantNoDimensionDiags(t, `private import NumericalFunctions::*;
	part def MassedComponent {
		part subcomponents : MassedComponent [*] default null;
		attribute mass :> ISQ::mass;
		attribute totalMass :> ISQ::mass = mass + sum(subcomponents.totalMass);
	}`)
}

// TestRecursiveRollupThroughACollectBody: the rollup written with a collect whose
// body names the feature being typed — its result type is the body's — terminates too.
func TestRecursiveRollupThroughACollectBody(t *testing.T) {
	wantNoDimensionDiags(t, `private import NumericalFunctions::*;
	private import ControlFunctions::*;
	part def MassedComponent {
		part subcomponents : MassedComponent [*] default null;
		attribute mass :> ISQ::mass;
		attribute totalMass :> ISQ::mass = mass + sum(subcomponents->collect { in c : MassedComponent; c.totalMass });
		attribute heaviest :> ISQ::mass = subcomponents->collect { in c : MassedComponent; c.heaviest }->reduce '+';
	}`)
}

// TestBoundMeasurementUnit: a unit binds to the unit definition typing it, to any
// measurement-reference supertype, and to no quantity value type; the checker
// judges each as the runtime's write conformance does. A quantity bound to a
// unit-typed feature (`hp : PowerUnit = 745.7 [W]` in the OMG examples) is not
// judged, as the pilot implementation accepts it.
func TestBoundMeasurementUnit(t *testing.T) {
	wantNoDimensionDiags(t, `attribute u : LengthUnit = m;`)
	wantNoDimensionDiags(t, `attribute u : LengthUnit = km;`)
	wantNoDimensionDiags(t, `attribute u : MeasurementReferences::ScalarMeasurementReference = m;`)
	wantNoDimensionDiags(t, `attribute u : MeasurementReferences::MeasurementUnit = m;`)
	wantOneDimensionError(t, `attribute u : LengthUnit = s;`,
		"cannot bind a value of type DurationUnit to a feature typed by LengthUnit")
	wantOneDimensionError(t, `attribute q : LengthValue = m;`,
		"cannot bind a value of type LengthUnit to a feature typed by LengthValue")
	wantNoDimensionDiags(t, `attribute u : PowerUnit = 745.7 [W];`)
}

// TestBoundComposedMeasurementUnit: a product, quotient or power of units is a
// DerivedUnit of the composed dimension, so it binds to a unit definition of
// that dimension and is refused by one of another with the dimension named.
func TestBoundComposedMeasurementUnit(t *testing.T) {
	wantNoDimensionDiags(t, `attribute a : AreaUnit = m * m;`)
	wantNoDimensionDiags(t, `attribute kpl : MeasurementReferences::DerivedUnit = km / L;`)
	wantNoDimensionDiags(t, `attribute v : SpeedUnit = m / s;`)
	wantNoDimensionDiags(t, `attribute v : SpeedUnit = km / h;`)
	wantNoDimensionDiags(t, `attribute c : VolumeUnit = m ** 3;`)
	wantNoDimensionDiags(t, `attribute u : MeasurementReferences::MeasurementUnit = m / s;`)
	wantOneDimensionError(t, `attribute a : AreaUnit = m * s;`,
		"cannot bind a measurement reference of dimension L·T to a feature typed by AreaUnit")
	wantOneDimensionError(t, `attribute u : LengthUnit = m / s;`,
		"cannot bind a measurement reference of dimension L·T^-1 to a feature typed by LengthUnit")
	// A quantity value type of the same dimension is no type of a unit: the
	// runtime refuses the write, so the checker refuses the binding.
	wantOneDimensionError(t, `attribute a : AreaValue = m * m;`,
		"cannot bind a measurement reference typed DerivedUnit to a feature typed by AreaValue")
	wantOneDimensionError(t, `attribute v : SpeedValue = km / h;`,
		"cannot bind a measurement reference typed DerivedUnit to a feature typed by SpeedValue")
	wantNoDimensionDiags(t, `attribute a : AreaValue = 2 [m] * 3 [m];`)
	wantNoDimensionDiags(t, `attribute n : ScalarValues::Natural = 1 * 1;`)
}

// TestBoundMeasurementScale: a measurement scale measures in a unit's dimension
// but no unit is a scale, so a unit of that dimension, however composed, is
// refused by a scale-typed feature, while a scale binds to its own types.
func TestBoundMeasurementScale(t *testing.T) {
	wantOneDimensionError(t, `attribute t : Time::TimeScale = s;`,
		"cannot bind a value of type DurationUnit to a feature typed by TimeScale")
	wantOneDimensionError(t, `attribute t : Time::TimeScale = s * s / s;`,
		"cannot bind a measurement reference typed DerivedUnit to a feature typed by TimeScale")
	wantOneDimensionError(t, `attribute t : MeasurementReferences::IntervalScale = h * s / min;`,
		"cannot bind a measurement reference typed DerivedUnit to a feature typed by IntervalScale")
	wantOneDimensionError(t, `attribute t : MeasurementReferences::MeasurementScale = K * K / K;`,
		"cannot bind a measurement reference typed DerivedUnit to a feature typed by MeasurementScale")
	wantNoDimensionDiags(t, `attribute t : DurationUnit = s * s / s;`)
	wantNoDimensionDiags(t, `attribute t : Time::TimeScale = Time::UTC;`)
	wantNoDimensionDiags(t, `attribute t : MeasurementReferences::IntervalScale = SI::'°C_abs';`)
	wantNoDimensionDiags(t, `attribute t : MeasurementReferences::ScalarMeasurementReference = SI::'°C_abs';`)
	wantOneDimensionError(t, `attribute t : ThermodynamicTemperatureUnit = SI::'°C_abs';`,
		"cannot bind a value of type IntervalScale to a feature typed by ThermodynamicTemperatureUnit")
}

// TestPointOnMeasurementScale: the checker admits a point's affine arithmetic and
// comparisons, warns of what evaluation refuses, and leaves a typed feature undecided.
func TestPointOnMeasurementScale(t *testing.T) {
	const warm = `attribute warm : ThermodynamicTemperatureValue = 26.85 [SI::'°C_abs'];
		attribute t0 : Time::TimeInstantValue = 5.0 [Time::UTC];
		`
	wantNoDimensionDiags(t, warm+`attribute a : ThermodynamicTemperatureValue = 26.85 [SI::'°C_abs'] + 10.0 [SI::'°C'];`)
	wantNoDimensionDiags(t, warm+`attribute a : ThermodynamicTemperatureValue = 10.0 [K] + 26.85 [SI::'°C_abs'];`)
	wantNoDimensionDiags(t, warm+`attribute a : ThermodynamicTemperatureValue = 26.85 [SI::'°C_abs'] - 10.0 [K];`)
	wantNoDimensionDiags(t, warm+`attribute d : ThermodynamicTemperatureValue = 26.85 [SI::'°C_abs'] - 20.0 [SI::'°C_abs'];`)
	wantNoDimensionDiags(t, warm+`attribute d : DurationValue = 5.0 [Time::UTC] - 3.0 [Time::UTC];`)
	wantNoDimensionDiags(t, warm+`attribute t : Time::TimeInstantValue = 5.0 [Time::UTC] + 3.0 [s];`)
	wantNoDimensionDiags(t, warm+`attribute b : Boolean = 26.85 [SI::'°C_abs'] < 300.0 [K];`)
	wantNoDimensionDiags(t, warm+`attribute b : Boolean = 300.0 [K] == 26.85 [SI::'°C_abs'];`)
	wantNoDimensionDiags(t, warm+`attribute a : ThermodynamicTemperatureValue = warm + 10.0 [SI::'°C'];`)
	wantNoDimensionDiags(t, warm+`attribute a : ThermodynamicTemperatureValue = warm - 20.0 [SI::'°C_abs'];`)
	wantNoDimensionDiags(t, warm+`attribute a : ThermodynamicTemperatureValue = 300.0 [K] - warm;`)
	wantNoDimensionDiags(t, warm+`attribute a : ThermodynamicTemperatureValue = warm + 20.0 [SI::'°C_abs'];`)
	wantNoDimensionDiags(t, warm+`attribute a : ThermodynamicTemperatureValue = 2 * warm;`)
	wantNoDimensionDiags(t, warm+`attribute b : Boolean = warm < 300.0 [K];`)
	wantOneDimensionWarning(t, warm+`attribute b = 26.85 [SI::'°C_abs'] < 1.0 [m];`,
		"operator '<' combines incommensurable quantities: SI::'°C_abs' (dimension Θ) and m (dimension L)")
	wantOneDimensionWarning(t, warm+`attribute a = 26.85 [SI::'°C_abs'] + 1.0 [m];`,
		"operator '+' combines incommensurable quantities: SI::'°C_abs' (dimension Θ) and m (dimension L)")
	wantOneDimensionWarning(t, warm+`attribute a = +(26.85 [SI::'°C_abs']) + 1.0 [m];`,
		"operator '+' combines incommensurable quantities: SI::'°C_abs' (dimension Θ) and m (dimension L)")
	wantOneDimensionWarning(t, warm+`attribute a = 26.85 [SI::'°C_abs'] + 10.0 [SI::'°C_abs'];`,
		"operator '+' adds two points on the measurement scale SI::'°C_abs', which have no sum; their difference is a magnitude in the scale's unit")
	wantOneDimensionWarning(t, warm+`attribute a = 300.0 [K] - 26.85 [SI::'°C_abs'];`,
		"operator '-' subtracts a point on the measurement scale SI::'°C_abs' from a magnitude, which is neither a point nor a magnitude; write `point - magnitude`")
	wantOneDimensionWarning(t, warm+`attribute a = 2 * 26.85 [SI::'°C_abs'];`,
		"operator '*' on a point on the measurement scale SI::'°C_abs': a point has no multiple and its scale is no unit to compose; convert it to a unit with ConvertQuantity first")
	wantOneDimensionWarning(t, warm+`attribute a = 26.85 [SI::'°C_abs'] * 2.0 [s];`,
		"operator '*' on a point on the measurement scale SI::'°C_abs': a point has no multiple and its scale is no unit to compose; convert it to a unit with ConvertQuantity first")
	wantOneDimensionWarning(t, warm+`attribute a = 2.0 [s] / 5.0 [Time::UTC];`,
		"operator '/' on a point on the measurement scale Time::UTC: a point has no multiple and its scale is no unit to compose; convert it to a unit with ConvertQuantity first")
	wantOneDimensionWarning(t, warm+`attribute a = 26.85 [SI::'°C_abs'] ** 2;`,
		"operator '**' on a point on the measurement scale SI::'°C_abs': a point has no multiple and its scale is no unit to compose; convert it to a unit with ConvertQuantity first")
	wantOneDimensionWarning(t, warm+`attribute a = -(26.85 [SI::'°C_abs']);`,
		"operator '-' negates a point on the measurement scale SI::'°C_abs', which has no negative; negate a difference between points instead")
}

func wantOneDimensionWarning(t *testing.T, body, want string) {
	t.Helper()
	diags := dimensionDiags(t, body)
	if len(diags) != 1 {
		t.Fatalf("want exactly one type diagnostic, got %v", diags)
	}
	if diags[0].Severity != SeverityWarning {
		t.Errorf("diagnostic is %v, want a warning: %s", diags[0].Severity, diags[0].Message)
	}
	if diags[0].Message != want {
		t.Errorf("message = %q, want %q", diags[0].Message, want)
	}
}

// TestBoundComposedCoordinateFrame: a frame divided or multiplied by a unit is the
// CoordinateFrame the frame calcs compose, so it binds to a frame type whose axes
// measure in the composed dimension (Annex A's `velocityCF = spatialCF / s`), is
// refused by one of another dimension or dimensions, and by any non-frame type;
// numbers written in a frame are a vector quantity of the frame's kind.
func TestBoundComposedCoordinateFrame(t *testing.T) {
	const frames = `private import ISQSpaceTime::*;
		private import MeasurementReferences::*;
		attribute spatialCF : CartesianSpatial3dCoordinateFrame[1] { :>> mRefs = (m, m, m); }
		attribute velocityCF : CartesianVelocity3dCoordinateFrame[1] = spatialCF / s;
		`
	wantNoDimensionDiags(t, frames+`attribute accelerationCF : CartesianAcceleration3dCoordinateFrame[1] = velocityCF / s;`)
	wantNoDimensionDiags(t, frames+`attribute cf : CoordinateFrame = spatialCF * s;`)
	wantNoDimensionDiags(t, frames+`attribute cf : VectorMeasurementReference = spatialCF / s;`)
	wantNoDimensionDiags(t, frames+`attribute p : Position3dVector = (1.0, 2.0, 3.0) [spatialCF];`)
	wantNoDimensionDiags(t, frames+`attribute v : Quantities::VectorQuantityValue = (1.0, 2.0, 3.0) [velocityCF];`)
	wantNoDimensionDiags(t, frames+`attribute v : CartesianVelocity3dVector = (1.0, 2.0, 3.0) [spatialCF / s];`)
	wantNoDimensionDiags(t, frames+`attribute a : CartesianAcceleration3dVector = (1.0, 2.0, 3.0) [velocityCF / s];`)
	wantOneDimensionError(t, frames+`attribute bad : CartesianPosition3dVector = (1.0, 2.0, 3.0) [velocityCF];`,
		"cannot bind a vector quantity in velocityCF (a CartesianVelocity3dCoordinateFrame) to a feature typed by CartesianPosition3dVector")
	wantOneDimensionError(t, frames+`attribute bad : CartesianPosition3dVector = (1.0, 2.0, 3.0) [spatialCF / s];`,
		"cannot bind a vector quantity in spatialCF/s, a coordinate frame whose axes measure in dimension L·T^-1, where CartesianSpatial3dCoordinateFrame admits L to a feature typed by CartesianPosition3dVector")
	wantOneDimensionError(t, frames+`attribute bad : SpeedValue = (1.0, 2.0, 3.0) [spatialCF / s];`,
		"cannot bind a vector quantity in spatialCF/s, a coordinate frame composed from another's axes to a feature typed by SpeedValue")
	wantOneDimensionError(t, frames+`attribute bad : CartesianSpatial3dCoordinateFrame = spatialCF / s;`,
		"cannot bind a coordinate frame whose axes measure in dimension L·T^-1, where CartesianSpatial3dCoordinateFrame admits L to a feature typed by CartesianSpatial3dCoordinateFrame")
	wantOneDimensionError(t, frames+`attribute bad : CartesianAcceleration3dCoordinateFrame = spatialCF / s;`,
		"cannot bind a coordinate frame whose axes measure in dimension L·T^-1, where CartesianAcceleration3dCoordinateFrame admits L·T^-2 to a feature typed by CartesianAcceleration3dCoordinateFrame")
	wantOneDimensionError(t, frames+`attribute bad : Time::TimeScale = spatialCF / s;`,
		"cannot bind a coordinate frame of dimensions [3], where TimeScale fixes dimensions [] to a feature typed by TimeScale")
	wantOneDimensionError(t, frames+`attribute bad : LengthUnit = spatialCF / s;`,
		"cannot bind a coordinate frame composed from another's axes to a feature typed by LengthUnit")
	wantOneDimensionError(t, frames+`attribute bad : SpeedUnit = spatialCF / s;`,
		"cannot bind a coordinate frame composed from another's axes to a feature typed by SpeedUnit")
	wantOneDimensionError(t, frames+`attribute bad : SpeedValue = spatialCF / s;`,
		"cannot bind a coordinate frame composed from another's axes to a feature typed by SpeedValue")
	wantOneDimensionError(t, frames+`attribute bad : ScalarValues::Real = spatialCF / s;`,
		"cannot bind a coordinate frame composed from another's axes to a feature typed by Real")
}

// TestComposedMeasurementUnitAsAnArgument: an operator expression over units is
// a measurement reference where an overload is chosen by argument type, so
// ToString(m / s) selects MeasurementRefCalculations::ToString and a quantity
// parameter refuses it by name; one over numbers (`1 * 1`) is the number it
// evaluates to, not the DerivedUnit unit notation would read it as.
func TestComposedMeasurementUnitAsAnArgument(t *testing.T) {
	wantNoDimensionDiags(t, `private import MeasurementRefCalculations::*;
		attribute text : ScalarValues::String = ToString(m / s);`)
	wantNoDimensionDiags(t, `private import QuantityCalculations::*;
		attribute q : LengthValue = ConvertQuantity(3 [km], m);`)
	wantNoDimensionDiags(t, `private import QuantityCalculations::*;
		attribute q : LengthValue = ConvertQuantity(3 [km], km / m * m);`)
	wantOneDimensionError(t, `private import QuantityCalculations::*;
		attribute q : LengthValue = ConvertQuantity(m, m);`,
		"argument 1 of ConvertQuantity expects ScalarQuantityValue, found LengthUnit")
	wantOneDimensionError(t, `private import QuantityCalculations::*;
		attribute q : LengthValue = ConvertQuantity(m / m, m);`,
		"argument 1 of ConvertQuantity expects ScalarQuantityValue, found DerivedUnit")
	wantNoDimensionDiags(t, `private import QuantityCalculations::*;
		attribute q : LengthValue = ConvertQuantity(1 * 1, m);`)
	wantNoDimensionDiags(t, `private import QuantityCalculations::*;
		attribute q : LengthValue = ConvertQuantity(1 / 1, m);`)
	wantNoDimensionDiags(t, `private import QuantityCalculations::*;
		attribute q : LengthValue = ConvertQuantity(1 ** 2, m);`)
}

// TestMeasurementUnitPowerOfAnUnknownExponent: a unit raised to a Real the checker
// cannot fold is a DerivedUnit whose dimension only the runtime knows, so it is an
// argument for a measurement-reference parameter and refused by a quantity one,
// while a binding to a unit definition is left to the runtime to judge.
func TestMeasurementUnitPowerOfAnUnknownExponent(t *testing.T) {
	wantNoDimensionDiags(t, `attribute e : ScalarValues::Real = 2.0;
		attribute a : AreaUnit = m ** e;`)
	wantNoDimensionDiags(t, `attribute e : ScalarValues::Real = 2.0;
		attribute a : LengthUnit = m ** e;`)
	wantNoDimensionDiags(t, `attribute e : ScalarValues::Real = 2.0;
		attribute a : MeasurementReferences::DerivedUnit = m ** e;`)
	wantNoDimensionDiags(t, `private import MeasurementRefCalculations::*;
		attribute e : ScalarValues::Real = 2.0;
		attribute text : ScalarValues::String = ToString(m ** e);`)
	wantNoDimensionDiags(t, `private import QuantityCalculations::*;
		attribute e : ScalarValues::Real = 2.0;
		attribute q : AreaValue = ConvertQuantity(3 [m ** 2], m ** e);`)
	wantOneDimensionError(t, `private import QuantityCalculations::*;
		attribute e : ScalarValues::Real = 2.0;
		attribute q : AreaValue = ConvertQuantity(m ** e, m ** 2);`,
		"argument 1 of ConvertQuantity expects ScalarQuantityValue, found DerivedUnit")
	wantNoDimensionDiags(t, `private import QuantityCalculations::*;
		attribute e : ScalarValues::Real = 2.0;
		attribute q : LengthValue = ConvertQuantity(2 ** e, m);`)
}

// TestInferredMeasurementUnitFeature: an untyped feature bound to a unit expression
// is typed by its value, a DerivedUnit, so it passes for a measurement reference
// and selects the overload taking one, and a quantity parameter refuses it by name.
func TestInferredMeasurementUnitFeature(t *testing.T) {
	wantNoDimensionDiags(t, `private import QuantityCalculations::*;
		attribute area = m * m;
		attribute q : AreaValue = ConvertQuantity(3 [m ** 2], area);`)
	wantNoDimensionDiags(t, `private import MeasurementRefCalculations::*;
		attribute area = m * m;
		attribute text : ScalarValues::String = ToString(area);`)
	wantNoDimensionDiags(t, `private import QuantityCalculations::*;
		private import MeasurementRefCalculations::*;
		attribute area = m * m;
		attribute again = area;
		attribute text : ScalarValues::String = ToString(again);`)
	wantOneDimensionError(t, `private import QuantityCalculations::*;
		attribute area = m * m;
		attribute q : AreaValue = ConvertQuantity(area, m ** 2);`,
		"argument 1 of ConvertQuantity expects ScalarQuantityValue, found DerivedUnit")
}

// TestMeasurementUnitDeclarationMembers: a member chain into a unit's declaration
// (`km.unitConversion.conversionFactor`, `m.quantityDimension.quantityPowerFactors`)
// is typed by the library's records, so it binds where the types agree and is
// refused where they do not — the same verdict the runtime reaches reading them.
func TestMeasurementUnitDeclarationMembers(t *testing.T) {
	wantNoDimensionDiags(t, `attribute f : ScalarValues::Real = SI::km.unitConversion.conversionFactor;`)
	wantNoDimensionDiags(t, `attribute u : LengthUnit = SI::km.unitConversion.referenceUnit;`)
	wantNoDimensionDiags(t, `attribute e : ScalarValues::Boolean = SI::km.unitConversion.isExact;`)
	wantNoDimensionDiags(t, `attribute n : ScalarValues::String = SI::km.unitConversion.prefix.longName;`)
	wantNoDimensionDiags(t, `attribute x : ScalarValues::Real = SI::m.quantityDimension.quantityPowerFactors#(1).exponent;`)
	wantNoDimensionDiags(t, `attribute u : MeasurementReferences::MeasurementUnit = SI::m.unitPowerFactors#(1).unit;`)
	wantNoDimensionDiags(t, `attribute k : ScalarValues::Real = SI::K.definitionalQuantityValues#(1).num;`)
	wantNoDimensionDiags(t, `attribute b : ScalarValues::Boolean = SI::m.isBound;`)
	wantNoDimensionDiags(t, `private import QuantityCalculations::*;
		attribute d : LengthValue = 3 [km];
		attribute q : LengthValue = ConvertQuantity(d, km.unitConversion.referenceUnit);`)
	wantOneDimensionError(t, `attribute f : ScalarValues::String = SI::km.unitConversion.conversionFactor;`,
		"cannot bind Real value to a feature typed by String")
}
