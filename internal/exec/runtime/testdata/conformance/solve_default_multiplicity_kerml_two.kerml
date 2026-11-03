// A KerML feature declaring and inheriting no multiplicity has the usual default
// 0..* (KerML 1.1 §7.4.12, §7.4.5.2): valued with two values it holds both, a
// collection the invariant's comparison does not take, and the solver refuses the
// read as one that may hold more than one value rather than encode one scalar.
package test {
	private import ScalarValues::Real;

	class Tank {
		feature level : Real = (2.0, 3.5);
		inv nonNegative { level >= 0.0 }
	}

	feature tank : Tank;
}
