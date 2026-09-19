// A Complex is what ComplexFunctions return, on the real axis or off it: the
// ordering scalar types classify none of them, whatever real number one may
// equal — `re` yields that number as a Real (KerML 1.1 §8.3.4.9).
package test {
	private import ScalarValues::*;
	private import ComplexFunctions::*;

	calc def ComplexCasts {
		out onAxisAsPositive : Positive[0..1] = (rect(3.0, 1.0) + rect(1.0, -1.0)) as Positive;
		out onAxisAsReal : Real[0..1] = (rect(3.0, 1.0) + rect(1.0, -1.0)) as Real;
		out onAxisAsComplex : Complex[0..1] = (rect(3.0, 1.0) + rect(1.0, -1.0)) as Complex;
		out realPartAsReal : Real[0..1] = re(rect(3.0, 1.0) + rect(1.0, -1.0)) as Real;
		out negativeOnAxisAsPositive : Positive[0..1] = (rect(-3.0, 1.0) + rect(-1.0, -1.0)) as Positive;
		out offAxisAsPositive : Positive[0..1] = (rect(3.0, 1.0) + rect(1.0, 1.0)) as Positive;
	}

	calc c : ComplexCasts;
}
