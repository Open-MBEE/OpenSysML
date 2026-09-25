// A calc body that computes with the non-normative OpenSysML extension library:
// the built-in implementations of exp, ln, log and the two-argument atan2, the
// functions the OMG Kernel Function Library declares nowhere.
package test {
	private import ScalarValues::*;
	private import OpenSysMLMathFunctions::*;

	calc def descentSteering {
		in altitude : Real;
		in vy : Real;
		in vx : Real;
		return : Real = ln(exp(altitude)) + atan2(vy, vx) * 4.0 + log(8.0, 2.0);
	}
}
