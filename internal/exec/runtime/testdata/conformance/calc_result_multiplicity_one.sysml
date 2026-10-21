// A result parameter declaring [1] holds its one value bare: a multiplicity of at most one holds the sole element.
package test {
	private import ScalarValues::*;

	calc def Shaped {
		in x : Real;
		return : Real[1] = x;
	}
}
