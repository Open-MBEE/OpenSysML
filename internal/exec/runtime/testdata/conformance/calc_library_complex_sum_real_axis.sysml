// With ComplexFunctions imported and RealFunctions not, sum and product over
// Reals select the Complex overload, whose `reduce '+'` keeps Real elements on
// the real axis: only a collection holding a Complex folds to one.
package test {
	private import ScalarValues::*;
	private import NumericalFunctions::*;
	private import ComplexFunctions::*;

	part def Probe {
		attribute realSum : Real = sum((1.0, 2.0));
		attribute integerProduct = product((2, 3));
		attribute complexSum = sum((rect(1.0, 2.0), 3.0));
	}
}
