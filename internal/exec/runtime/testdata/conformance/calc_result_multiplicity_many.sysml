// A result parameter declaring [0..*] holds one computed value as a one-element sequence: a feature value is a sequence bounded by its multiplicity (KerML 7.3), held in the shape the declared multiplicity gives it, as an action parameter is.
package test {
	private import ScalarValues::*;

	calc def Shaped {
		in x : Real;
		return : Real[0..*] = x;
	}
}
