// A KerML feature declaring and inheriting no multiplicity has the usual default
// 0..* (KerML 1.1 §7.4.12, §7.4.5.2), so an invariant reading it reads a
// collection: valued with nothing, it holds no value, and the solver refuses the
// read as one that may hold more than one value rather than encode one scalar.
package test {
	private import ScalarValues::Real;

	class Tank {
		feature level : Real = ();
		inv nonNegative { level >= 0.0 }
	}

	feature tank : Tank;
}
