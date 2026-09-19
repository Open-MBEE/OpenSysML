// An Array of no dimensions has rank 0 and flattenedSize 1, so one stating
// `dimensions = ()` with no element is malformed, not a bare object.
package test {
	private import ScalarValues::*;
	private import Collections::*;

	attribute a : Array { :>> dimensions = (); :>> elements = (); }

	calc def rank {
		return : Natural = a.rank;
	}
}
