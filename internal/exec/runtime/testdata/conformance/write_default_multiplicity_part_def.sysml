// A feature stating no multiplicity is held to the one it inherits by subsetting (KerML 1.1
// §7.4.12, §7.3.4.5), a stated [0..*] holds what it is written, and a stated [1..2] holds two:
// the action a part definition owns writes the part's features with the count judged once,
// by the feature's effective multiplicity, as a write from outside the behavior is.
package test {
	private import ScalarValues::Real;

	part def P {
		attribute many : Real[0..*];
		attribute subset : Real :> many;
		attribute open : Real[0..*];
		attribute pair : Real[1..2];
		action a {
			assign subset := (3.0, 4.0);
			assign open := (1.0, 2.0, 3.0);
			assign pair := (5.0, 6.0);
		}
	}

	part p : P;
}
