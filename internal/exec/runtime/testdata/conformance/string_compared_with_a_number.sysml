// StringFunctions declares '<' over two String operands, so comparing a string
// with a number is reported rather than either being coerced to the other.
package test {
	private import ScalarValues::*;

	calc def afterA {
		in other;
		return : Boolean = "a" < other;
	}
}
