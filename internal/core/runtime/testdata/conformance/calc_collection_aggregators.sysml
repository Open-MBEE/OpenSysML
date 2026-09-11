// The aggregating operations of the library over a sequence: sum and product of
// NumericalFunctions, the element-naming head, last and tail of
// SequenceFunctions, and union and intersection.
package test {
	private import SequenceFunctions::*;
	private import NumericalFunctions::*;
	calc aggregate {
		attribute xs : Integer[*] = (1, 2, 3, 4);
		attribute ys : Integer[*] = (3, 4, 5);
		attribute joined : Integer[*] nonunique = xs->union(ys);
		attribute common : Integer[*] = xs->intersection(ys);
		return : Integer =
			xs->sum() * 1000
			+ xs->product()
			+ xs->head() * 100000
			+ xs->last() * 10
			+ joined->size() * 10000
			+ common->size() * 1000000
			+ xs->tail()->sum() * 100;
	}
}
