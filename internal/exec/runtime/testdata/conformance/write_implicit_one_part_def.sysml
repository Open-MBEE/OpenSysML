// An attribute usage stating no multiplicity on a part definition has the implicit [1..1]
// (SysML v2 §7.6.3): the action the part definition owns cannot write two values into it.
package test {
	private import ScalarValues::Real;

	part def P {
		attribute z : Real;
		action a {
			assign z := (3.0, 4.0);
		}
	}

	part p : P;
}
