// One local name declared by several imported function packages: the call
// runs the declaration whose parameter types the arguments fit most closely.
package test {
	private import ScalarValues::*;
	private import IntegerFunctions::*;
	private import RealFunctions::*;
	private import RationalFunctions::*;
	private import ComplexFunctions::*;

	part def Probe {
		attribute absInteger = abs(-2);
		attribute absReal = abs(-2.5);
		attribute absComplex = abs(rect(3.0, 4.0));
		attribute zeroComplex = isZero(rect(0.0, 0.0));
		attribute maxMixed = max(3, 2.5);
	}
}
