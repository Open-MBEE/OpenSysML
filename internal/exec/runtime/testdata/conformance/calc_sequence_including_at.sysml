// includingAt inserts before the 1-based index, shifting the tail right, so the
// result is longer than the input by the values inserted.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	calc def insert {
		in x : Integer;
		return : Integer[0..*] = (1, 2, 3)->includingAt(x, 2);
	}
}
