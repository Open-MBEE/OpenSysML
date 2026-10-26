// excludingAt removes the 1-based positions startIndex..endIndex, endIndex
// defaulting to startIndex; the library body is
// (seq->subsequence(1, startIndex - 1), seq->subsequence(endIndex + 1)).
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	calc def drop {
		in x : Integer;
		return : Integer[0..*] nonunique = ((1, 2, 3, 4)->excludingAt(x), (1, 2, 3, 4)->excludingAt(x, x + 1));
	}
}
