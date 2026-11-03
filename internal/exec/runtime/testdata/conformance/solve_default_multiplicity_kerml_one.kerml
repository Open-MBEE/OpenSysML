// A KerML feature declaring and inheriting no multiplicity has the usual default
// 0..* (KerML 1.1 §7.4.12, §7.4.5.2): valued with one value it holds one, which
// the evaluator reads, while the solver refuses the read as one that may hold
// more than one value — the one value held is not a bound of one.
package test {
	private import ScalarValues::Real;

	class Tank {
		feature level : Real = 2.0;
		inv nonNegative { level >= 0.0 }
	}

	feature tank : Tank;
}
