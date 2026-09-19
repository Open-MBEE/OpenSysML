// A calc body that computes with the KerML function library: the built-in
// implementations of sqrt and sin, and the ** operator.
package test {
	private import ScalarValues::*;
	private import RealFunctions::*;
	private import TrigFunctions::*;

	calc def diagonalGain {
		in x : Real;
		in y : Real;
		return : Real = sqrt(x ** 2 + y ** 2) + sin(0.0) * 10.0;
	}
}
