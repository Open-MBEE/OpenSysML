// A metadata annotation among a calculation body's members annotates the
// calculation, as a comment does, and is no step of it: the result expression
// after it is still the value answered.
package test {
	private import ScalarValues::*;

	metadata def Note { attribute tag : String; }

	calc def Twice {
		in x : Real;
		@Note { tag = "doubles"; }
		x * 2.0
	}
}
