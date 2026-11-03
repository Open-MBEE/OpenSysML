// An endIndex past the end drops from startIndex to the end: subsequence(1, 1)
// keeps the head and subsequence(10) iterates the empty range 10..3.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	calc def dropStraddling {
		return : Integer[0..*] = (1, 2, 3)->excludingAt(2, 9);
	}
}
