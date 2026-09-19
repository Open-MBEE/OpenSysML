// No Natural equals a negative Integer, so ToNatural reports a domain error.
package test {
	private import ScalarValues::*;

	calc def narrow {
		return : Natural = IntegerFunctions::ToNatural(-3);
	}
}
