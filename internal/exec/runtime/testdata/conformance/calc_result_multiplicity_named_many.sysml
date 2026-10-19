// A named result parameter declaring [0..*] holds its value as a sequence, as an anonymous one does.
package test {
	private import ScalarValues::*;

	calc def Shaped {
		in x : Real;
		return r : Real[0..*] = x;
	}
}
