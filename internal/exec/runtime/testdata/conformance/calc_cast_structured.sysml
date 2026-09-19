// A structured value is kept by a target its own shape, units and frame fit —
// the same reading a write to a feature of that type applies: a vector quantity
// of three axes is no scalar quantity (KerML 1.1 §8.3.4.9).
package test {
	private import ScalarValues::*;
	private import VectorValues::*;
	private import Quantities::*;
	private import SI::*;

	attribute d : VectorQuantityValue = 2 [m] * VectorFunctions::VectorOf((1.0, 2.0, 3.0));

	calc def StructuredCasts {
		out asVectorQuantity : VectorQuantityValue[0..*] = d as VectorQuantityValue;
		out asTensorQuantity : TensorQuantityValue[0..*] = d as TensorQuantityValue;
		out asScalarQuantity : ScalarQuantityValue[0..*] = d as ScalarQuantityValue;
		out asNumber : NumericalValue[0..*] = d as NumericalValue;
	}

	calc c : StructuredCasts;
}
