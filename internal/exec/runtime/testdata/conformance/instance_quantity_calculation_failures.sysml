// The square root of a quantity whose unit has an odd exponent is rejected:
// sqrt(m**3) is no unit the runtime can name, nor is sqrt(rad) — an angle
// is dimension one, but a named unit all the same — and the diagnostic says
// so. A domain function the runtime cannot compute fails by name, with its reason.
package test {
	public import ISQ::*;
	public import SI::*;
	public import QuantityCalculations::*;

	part def Block {
		attribute volume : VolumeValue = 8 [m**3];
		attribute edge = sqrt(volume);
		attribute negative = sqrt(-4 [m**2]);
		attribute turn : AngularMeasureValue = 9 [rad];
		attribute halfTurn = sqrt(turn);
		attribute ratio = sqrt(9 [m/m]);
		attribute spread = VectorCalculations::outer((1.0, 2.0), (3.0, 4.0));
	}
}
