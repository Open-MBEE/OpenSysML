// A startIndex beyond size + 1 makes the body's subsequence(1, startIndex - 1)
// evaluate seq#(size + 1), which is the index error; the result is not the
// sequence unchanged.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	calc def dropBeyondTheEnd {
		return : Integer[0..*] = (1, 2, 3)->excludingAt(5);
	}
}
