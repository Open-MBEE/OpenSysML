// KerML's ScalarValues::Integer is the mathematical integers, with no bound:
// a sum past int64 is the next Integer, a power the exact power, a quotient of
// two such Integers the exact Rational, and a remainder the exact remainder.
// A literal past int64 is the Integer it spells.
package test {
	private import ScalarValues::*;

	action def Unbounded {
		attribute sum : Integer = 9223372036854775807 + 1;
		attribute sumIsNext : Boolean = 9223372036854775807 + 1 == 9223372036854775808;
		attribute power : Integer = 2 ** 70;
		attribute ratioIsTwo : Boolean = (2 ** 70) / (2 ** 69) == 2;
		attribute remainder : Integer = (2 ** 70) % 7;
		attribute literal : Integer = 123456789012345678901234567890;
		attribute demoted : Integer = 2 ** 70 - 2 ** 70 + 1;
		attribute product : Integer = 0;

		first start then multiply;
		action multiply {
			assign product := literal * literal;
		}
		first multiply then done;
	}
}
