// A startIndex of size + 1 keeps every position: subsequence(1, size) is the
// whole sequence and subsequence(size + 2) iterates an empty range, so no
// seq#(i) is evaluated and the sequence is answered unchanged, not an error.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	calc def dropPastTheEnd {
		return : Integer[0..*] = (1, 2, 3)->excludingAt(4);
	}
}
