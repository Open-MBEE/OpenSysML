// A result parameter declaring [0..1] holds its one value bare, as any multiplicity of at most one does.
package test {
	private import ScalarValues::*;

	calc def Shaped {
		in x : Real;
		return : Real[0..1] = x;
	}
}
