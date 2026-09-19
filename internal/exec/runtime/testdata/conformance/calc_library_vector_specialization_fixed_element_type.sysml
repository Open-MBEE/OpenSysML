// An object typed directly by a vector specialization fixing Integer elements
// and dimension 2 is validated against both: a Real element is refused by the
// element type before the vector is read.
package test {
	private import ScalarValues::*;
	private import VectorValues::*;

	attribute def IntegerPair :> NumericalVectorValue {
		attribute :>> elements : Integer;
		attribute :>> dimension = 2;
	}

	attribute v : IntegerPair {
		attribute :>> elements = (1.5, 2);
	}

	calc def head {
		return : Integer = v.elements#(1);
	}
}
