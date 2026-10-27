// An attribute usage stating no multiplicity on a state definition has the implicit [1..1]
// (SysML v2 §7.6.3): the state definition's own entry behavior cannot write two values into it.
package test {
	private import ScalarValues::Real;

	state def S {
		attribute z : Real;

		entry; then go;
		state go {
			entry action set {
				assign z := (3.0, 4.0);
			}
		}
	}
}
