// The identity of each aggregation is its answer for an empty collection, which
// is what the library's own sum0/product1 compute: 0 for a sum, 1 for a
// product. head of an empty sequence is empty — the `Anything[0..1]` it is
// declared to return — and its size is 0.
package test {
	private import SequenceFunctions::*;
	private import NumericalFunctions::*;
	calc empties {
		attribute none : Integer[*] = ();
		return : Integer =
			none->sum()
			+ none->product()
			+ none->size()
			+ (if none->isEmpty() ? 10 else 0)
			+ (if none->head() == null ? 100 else 0);
	}
}
