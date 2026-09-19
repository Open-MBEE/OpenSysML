// A model's own NumericalVectorValue specialization holds the vector VectorOf
// builds when the element type it fixes holds: Integer components here.
package test {
	private import ScalarValues::*;
	private import VectorValues::*;
	private import VectorFunctions::*;

	attribute def IntegerVector :> NumericalVectorValue {
		attribute :>> elements : Integer;
	}

	calc def counts {
		return : IntegerVector = VectorOf((1, 2, 3, 4));
	}
}
