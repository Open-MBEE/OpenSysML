// Several indexes address an Array; a flat sequence has one dimension, so two
// indexes into it are a type mismatch, not a selection.
package test {
	private import ScalarValues::*;

	calc def cell {
		return : Integer = BaseFunctions::'#'((1, 2, 3, 4), (2, 2));
	}
}
