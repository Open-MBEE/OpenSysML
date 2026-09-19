// An Array's derived features read out of the value: rank is the number of
// dimensions, flattenedSize their product, elements the row-major sequence.
package test {
	private import ScalarValues::*;
	private import Collections::*;

	attribute a : Array { :>> dimensions = (2, 3); :>> elements = (1, 2, 3, 4, 5, 6); }

	calc def shape {
		return : Integer[*] nonunique = (a.rank, a.flattenedSize, a.dimensions, a.elements#(6));
	}
}
