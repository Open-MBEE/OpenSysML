// Only the imported function packages contribute overloads: with the Complex
// package left out, a Complex argument fits no visible abs and the call fails at
// the declaration checked rather than reaching a hidden one. Qualifying the name
// reaches the declaration whatever the model imports.
package test {
	private import ScalarValues::*;
	private import IntegerFunctions::*;
	private import RealFunctions::*;
	private import RationalFunctions::*;

	part def Probe {
		attribute absInteger = abs(-2);
		attribute absComplex = abs(ComplexFunctions::rect(3.0, 4.0));
		attribute absQualified = ComplexFunctions::abs(ComplexFunctions::rect(3.0, 4.0));
	}
}
