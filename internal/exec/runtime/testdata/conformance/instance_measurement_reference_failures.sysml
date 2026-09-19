// What the runtime cannot compute about a measurement reference fails by name.
// A conversion between dimensions is incommensurable; a reference is neither a
// number nor an addend; a unit composed at runtime (`m / s`) names no
// declaration whose members (unitConversion, quantityDimension,
// unitPowerFactors) could be read; a vector literal takes a coordinate
// frame, not a sequence of units; `transform` takes a transformation, not a
// vector; and `outer` declares a VectorQuantityValue result no outer
// product is.
package test {
	public import ISQ::*;
	public import SI::*;
	public import MeasurementReferences::*;
	public import QuantityCalculations::*;

	part def Block {
		attribute side : LengthValue = 3 [m];
		attribute inSeconds = ConvertQuantity(side, s);
		attribute scaled = m * 3;
		attribute summed = m + m;
		attribute conversion = (m / s).unitConversion;
		attribute dimension = (m / s).quantityDimension;
		attribute factors = (m ** 2).unitPowerFactors;
		attribute notAUnit = ConvertQuantity(side, 3);
		attribute mixed = VectorCalculations::'['((1.0, 2.0), (m, s));
		attribute turned = VectorCalculations::transform(VectorFunctions::VectorOf((1.0, 2.0)) [m], VectorFunctions::VectorOf((0.0, 1.0)) [m]);
		attribute spread = VectorCalculations::outer((1.0, 2.0), (3.0, 4.0));
	}
}
