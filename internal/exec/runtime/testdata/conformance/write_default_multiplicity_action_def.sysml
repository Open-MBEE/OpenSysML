// The same features declared on an action definition and written by its own body take the
// same count judgment as the part definition's (write_default_multiplicity_part_def): the
// subsetting feature inherits [0..*] and holds both values, the stated bounds are judged.
package test {
	private import ScalarValues::Real;

	action def A {
		attribute many : Real[0..*];
		attribute subset : Real :> many;
		attribute open : Real[0..*];
		attribute pair : Real[1..2];
		assign subset := (3.0, 4.0);
		assign open := (1.0, 2.0, 3.0);
		assign pair := (5.0, 6.0);
	}
}
