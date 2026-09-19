// Summing quantities with QuantityCalculations imported: unqualified `sum`
// resolves to QuantityCalculations::sum, which computes like the numeric sum
// instead of running the vendored body whose private `zero` has no value.
package test {
	public import ISQ::*;
	public import SI::*;
	public import QuantityCalculations::*;

	calc def Total {
		return : LengthValue = (1 [m], 2 [m])->sum();
	}
}
