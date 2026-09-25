// An index past a dimension's extent is out of range, reported with the
// dimension it exceeds.
package test {
	private import ScalarValues::*;
	private import Collections::*;

	attribute a : Array { :>> dimensions = (2, 3); :>> elements = (1, 2, 3, 4, 5, 6); }

	calc def cell {
		return : Integer = a#(3, 1);
	}
}
