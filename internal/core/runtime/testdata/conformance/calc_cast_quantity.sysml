// A quantity is kept by the quantity value type whose measurement reference its
// unit is one of, and by that type's supertypes; a length is not a duration
// (KerML 1.1 §8.3.4.9).
package test {
	private import ScalarValues::*;
	private import SI::*;
	private import ISQ::*;
	private import QuantityCalculations::*;

	// A subtype narrowing lengths by something a magnitude and a unit do not state.
	attribute def RoomLength :> ISQBase::LengthValue;

	// A union of quantity types states no measurement reference of its own.
	attribute def LengthOrDuration unions ISQBase::LengthValue, ISQBase::DurationValue;

	attribute room : RoomLength = 4 [m];

	calc def QuantityCasts {
		in length : ISQBase::LengthValue = 5 [m];
		out asLength : ISQBase::LengthValue[0..1] = length as ISQBase::LengthValue;
		out asScalarQuantity : Quantities::ScalarQuantityValue[0..1] = length as Quantities::ScalarQuantityValue;
		out asDuration : ISQBase::DurationValue[0..1] = length as ISQBase::DurationValue;
		out lengthsOfMixed : ISQBase::LengthValue[0..*] = (5 [m], 3 [s], 2 [m]) as ISQBase::LengthValue;
		// Nothing kept still measures in the source's unit, so an aggregate of it does.
		out sumOfNoDurations = sum((5 [m], 2 [m]) as ISQBase::DurationValue);
		// The declaring type says which of its dimension's values these are.
		out declaredAsSubtype : RoomLength[0..1] = room as RoomLength;
		out declaredAsBaseQuantity : ISQBase::LengthValue[0..1] = room as ISQBase::LengthValue;
		out declaredAsDuration : ISQBase::DurationValue[0..1] = room as ISQBase::DurationValue;
		// A union is read through its operands, each of which fixes a reference.
		out bareAsUnion : LengthOrDuration[0..1] = 5 [m] as LengthOrDuration;
		out bareSecondsAsUnion : LengthOrDuration[0..1] = 3 [s] as LengthOrDuration;
		out bareMassAsUnion : LengthOrDuration[0..1] = 2 [kg] as LengthOrDuration;
	}

	calc c : QuantityCasts;
}
