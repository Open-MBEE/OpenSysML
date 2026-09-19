// A VectorQuantityValue specialization fixing Integer components refuses a
// vector quantity with a Real axis, naming the component.
package test {
	private import ScalarValues::*;
	private import VectorValues::*;
	private import VectorFunctions::*;
	private import Quantities::*;
	private import SI::*;

	attribute def Counts :> VectorQuantityValue {
		attribute :>> num : Integer;
	}

	calc def halves {
		return : Counts = VectorOf((1.5, 2.5)) [m];
	}
}
