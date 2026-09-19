// An index outside 1..size+1 names no insertion point and is reported, rather
// than appending or dropping the values.
package test {
	private import ScalarValues::*;
	private import SequenceFunctions::*;

	calc def insertPastTheEnd {
		return : Integer[0..*] = (1, 2, 3)->includingAt(9, 5);
	}
}
