// A library function the library states no convention for reports itself by
// name: no tensor product says which indices contract.
package test {
	private import ScalarValues::*;
	private import Quantities::*;
	private import TensorCalculations::*;

	calc def product {
		return : TensorQuantityValue = tensorTensorMult(1.0 [SI::m], 2.0 [SI::m]);
	}
}
