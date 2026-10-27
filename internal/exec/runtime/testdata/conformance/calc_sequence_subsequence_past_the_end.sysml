// subsequence(2, 5) of three elements iterates 2..5 and evaluates seq#(4),
// which is the index error: an endIndex past the end is not clipped.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	calc def slicePastTheEnd {
		return : Integer[0..*] = (1, 2, 3)->subsequence(2, 5);
	}
}
