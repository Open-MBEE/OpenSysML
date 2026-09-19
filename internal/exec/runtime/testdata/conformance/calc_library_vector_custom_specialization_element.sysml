// A NumericalVectorValue specialization fixing Integer elements refuses a vector
// with a Real component, naming the element.
package test {
	private import ScalarValues::*;
	private import VectorValues::*;
	private import VectorFunctions::*;

	attribute def IntegerVector :> NumericalVectorValue {
		attribute :>> elements : Integer;
	}

	calc def counts {
		return : IntegerVector = VectorOf((1, 2.5, 3));
	}
}
