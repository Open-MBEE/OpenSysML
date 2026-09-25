// A Collections::Array declared with its dimensions and elements is one value
// that 'array#' indexes in row-major order: the last index varies fastest.
package test {
	private import ScalarValues::*;
	private import Collections::*;

	attribute a : Array { :>> dimensions = (2, 3); :>> elements = (1, 2, 3, 4, 5, 6); }

	calc def cell {
		return : Integer = CollectionFunctions::'array#'(a, (2, 1));
	}
}
