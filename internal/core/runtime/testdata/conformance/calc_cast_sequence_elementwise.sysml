// Casting a sequence is element-wise: the elements the target classifies are
// kept in their original order, a sequence none of whose elements it
// classifies casts to the empty sequence, and one element the target cannot
// decide refuses the whole cast (KerML 1.1 §8.3.4.9).
package test {
	private import ScalarValues::*;

	calc def SequenceCasts {
		out integersOfMixed : Integer[0..*] = (1, 2.5, 3, 4.0) as Integer;
		out rationalsOfMixed : Rational[0..*] = (1, 2.5, 3, 4.0) as Rational;
		out noneConform : Natural[0..*] = (-1, -2) as Natural;
		out emptyStaysEmpty : Integer[0..*] = () as Integer;
	}

	calc c : SequenceCasts;

	attribute naturalsOfSigned : Natural[0..*] = (-1, 2, -3, 4) as Natural;
}
