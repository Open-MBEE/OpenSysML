// An operator function's `[1]` operand holding several values is a
// multiplicity violation, judged before the operand's kind.
package test {
	private import ScalarValues::*;

	calc def addAll {
		attribute xs : Integer[0..*] = (1, 2, 3);
		return : Integer = IntegerFunctions::'+'(xs, 1);
	}
}
