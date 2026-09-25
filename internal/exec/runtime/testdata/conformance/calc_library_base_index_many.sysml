// BaseFunctions::'#' with several indexes addresses an Array by one index per
// dimension; the operator form `a#(i, j)` is the same call.
package test {
	private import ScalarValues::*;
	private import Collections::*;

	attribute a : Array { :>> dimensions = (2, 3); :>> elements = (1, 2, 3, 4, 5, 6); }

	calc def cell {
		return : Integer = a#(1, 3) + BaseFunctions::'#'(a, (2, 3));
	}
}
