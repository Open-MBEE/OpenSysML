// QuantityCalculations over ISQ quantities in SI units: the functions the
// domain library declares compute unit-aware, and a unit an operation composes
// displays in canonical form (`m**2`, not `m*m` or `(m)**2`). `min` is named
// qualified because the SI wildcard import also brings in the minute, `SI::min`.
package test {
	public import ISQ::*;
	public import SI::*;
	public import QuantityCalculations::*;
	private import MeasurementReferences::DimensionOneValue;

	part def Plot {
		attribute side : LengthValue = 3 [m];
		attribute area : AreaValue = side * side;
		attribute areaByPower : AreaValue = side ** 2;
		attribute root : LengthValue = sqrt(area);
		attribute back : LengthValue = area / side;
		attribute depth : LengthValue = abs(-4 [m]);
		attribute perimeter : LengthValue = (side, side, side, side)->sum();
		attribute longest : LengthValue = max(side, 400 [cm]);
		attribute shortest : LengthValue = QuantityCalculations::min(side, 400 [cm]);
		attribute stepsDown : LengthValue = floor(2.7 [m]);
		attribute stepsNear : LengthValue = round(2.7 [m]);
		attribute wider : ScalarValues::Boolean = 400 [cm] > side;
		attribute same : ScalarValues::Boolean = 300 [cm] == side;
		attribute label : ScalarValues::String = ToString(side);
		attribute metres : ScalarValues::Real = ToReal(side);
		attribute count : ScalarValues::Integer = ToInteger(side);
		attribute ratio : DimensionOneValue = ToDimensionOneValue(2.5);
		attribute stretched : LengthValue = ratio * side;
		attribute grown : DimensionOneValue = ratio + 1.5;
	}
}
