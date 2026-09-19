// An index of size+1 names the position after the last element, so includingAt
// appends there; an index past it names no insertion point at all.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	calc def append {
		in x : Integer;
		return : Integer[0..*] = (1, 2, 3)->includingAt(x, 4);
	}
}
