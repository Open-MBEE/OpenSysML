// A VectorQuantityValue specialization fixing three components refuses a
// two-axis vector quantity, naming the effective multiplicity of num.
package test {
	private import ScalarValues::*;
	private import VectorValues::*;
	private import VectorFunctions::*;
	private import Quantities::*;
	private import SI::*;

	attribute def Velocity3 :> VectorQuantityValue {
		attribute :>> num : Real[3];
	}

	calc def planar {
		return : Velocity3 = VectorOf((1.0, 2.0)) [m];
	}
}
