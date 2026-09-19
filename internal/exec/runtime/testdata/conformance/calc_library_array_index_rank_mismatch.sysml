// An Array takes one index per dimension: one index into a rank-2 Array is a
// multiplicity violation of `indexes: Positive[n]`, not a flattened lookup.
package test {
	private import ScalarValues::*;
	private import Collections::*;

	attribute a : Array { :>> dimensions = (2, 3); :>> elements = (1, 2, 3, 4, 5, 6); }

	calc def cell {
		return : Integer = CollectionFunctions::'array#'(a, 4);
	}
}
