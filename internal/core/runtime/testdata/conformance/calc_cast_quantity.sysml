// A quantity is kept by the quantity value type whose measurement reference its
// unit is one of, and by that type's supertypes; a length is not a duration
// (KerML 1.1 §8.3.4.9).
package test {
	private import ScalarValues::*;
	private import SI::*;
	private import ISQ::*;

	calc def QuantityCasts {
		in length : ISQBase::LengthValue = 5 [m];
		out asLength : ISQBase::LengthValue[0..1] = length as ISQBase::LengthValue;
		out asScalarQuantity : Quantities::ScalarQuantityValue[0..1] = length as Quantities::ScalarQuantityValue;
		out asDuration : ISQBase::DurationValue[0..1] = length as ISQBase::DurationValue;
		out lengthsOfMixed : ISQBase::LengthValue[0..*] = (5 [m], 3 [s], 2 [m]) as ISQBase::LengthValue;
	}

	calc c : QuantityCasts;
}
