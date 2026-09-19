// A measurement unit named in an expression is a value: the reference it
// declares, carrying its identity and its reduction to base units. A quantity
// reads its `num` and `mRef`, is built from a number and a reference by
// QuantityCalculations::'[' and converts to a reference by ConvertQuantity,
// and the MeasurementRefCalculations compose references into new ones. Two
// references are equal when they reduce alike, so `SI::'m/s'` is `m / s`.
package test {
	public import ISQ::*;
	public import SI::*;
	public import MeasurementReferences::*;
	public import QuantityCalculations::*;
	private import ScalarValues::*;

	part def Survey {
		attribute side : LengthValue = 3 [km];
		attribute unit : LengthUnit = m;
		attribute sideUnit : LengthUnit = side.mRef;
		attribute magnitude : Number = side.num;
		attribute perSecond : SpeedUnit = m / s;
		attribute perSecondNamed : SpeedUnit = SI::'m/s';
		attribute area : AreaUnit = m * m;
		attribute areaByPower : AreaUnit = m ** 2;
		attribute areaByCaret : AreaUnit = MeasurementRefCalculations::'^'(m, 2);
		attribute inMetres : LengthValue = ConvertQuantity(side, unit);
		attribute inCentimetres : LengthValue = ConvertQuantity(side, cm);
		attribute speed : SpeedValue = ConvertQuantity(2 [m/s], SI::'km/h');
		attribute built : LengthValue = '['(2.5, km);
		attribute sameUnit : Boolean = sideUnit == km;
		attribute otherUnit : Boolean = sideUnit == unit;
		attribute sameReduction : Boolean = perSecond == perSecondNamed;
		attribute isLengthUnit : Boolean = unit istype LengthUnit;
		attribute isUnit : Boolean = area istype MeasurementUnit;
		attribute label : String = MeasurementRefCalculations::ToString(perSecond);
		attribute velocity : Quantities::VectorQuantityValue = VectorFunctions::VectorOf((1.0, 2.0, 3.0)) [m];
		attribute velocityUnit : LengthUnit = velocity.mRef;
	}
}
