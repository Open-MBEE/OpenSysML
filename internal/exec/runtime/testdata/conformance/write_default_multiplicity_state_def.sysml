// A state definition's own entry behavior writes the features it declares with the same
// count judgment as an action's (write_default_multiplicity_action_def): the subsetting
// feature inherits [0..*] and holds both values, the stated [1..2] holds two.
package test {
	private import ScalarValues::Real;

	state def S {
		attribute many : Real[0..*];
		attribute subset : Real :> many;
		attribute pair : Real[1..2];

		entry; then go;
		state go {
			entry action set {
				assign subset := (3.0, 4.0);
				assign pair := (5.0, 6.0);
			}
		}
	}
}
