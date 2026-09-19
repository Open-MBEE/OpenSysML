// An object typed directly by a vector specialization fixing dimension 2 is
// validated against it: three elements do not fill the dimension.
package test {
	private import ScalarValues::*;
	private import VectorValues::*;

	attribute def IntegerPair :> NumericalVectorValue {
		attribute :>> elements : Integer;
		attribute :>> dimension = 2;
	}

	attribute v : IntegerPair {
		attribute :>> elements = (1, 2, 3);
	}

	calc def head {
		return : Integer = v.elements#(1);
	}
}
