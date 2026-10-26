// An inverted range removes nothing: subsequence(1, 2) then subsequence(3) is
// the whole sequence, and a wider inversion repeats the overlap, exactly as the
// body concatenates the two subsequences.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	calc def dropInverted {
		return : Integer[0..*] nonunique = ((1, 2, 3)->excludingAt(3, 2), (1, 2, 3)->excludingAt(3, 1));
	}
}
