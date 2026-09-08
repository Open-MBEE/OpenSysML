// A complex value that arithmetic left on the real axis is the real number it
// holds, so the ordering scalar types classify it by that number; one off the
// axis is on no ordering bound (KerML 1.1 §8.3.4.9).
package test {
	private import ScalarValues::*;
	private import ComplexFunctions::*;

	calc def ComplexCasts {
		out onAxisAsPositive : Positive[0..1] = (rect(3.0, 1.0) + rect(1.0, -1.0)) as Positive;
		out negativeOnAxisAsPositive : Positive[0..1] = (rect(-3.0, 1.0) + rect(-1.0, -1.0)) as Positive;
		out offAxisAsPositive : Positive[0..1] = (rect(3.0, 1.0) + rect(1.0, 1.0)) as Positive;
	}

	calc c : ComplexCasts;
}
