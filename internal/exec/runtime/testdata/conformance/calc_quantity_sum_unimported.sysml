// Summing quantities without QuantityCalculations imported: unqualified `sum`
// resolves to the numeric NumericalFunctions::sum, which sums quantities too.
package test {
	public import ISQ::*;
	public import SI::*;
	private import NumericalFunctions::*;

	calc def Total {
		return : LengthValue = (1 [m], 2 [m])->sum();
	}
}
