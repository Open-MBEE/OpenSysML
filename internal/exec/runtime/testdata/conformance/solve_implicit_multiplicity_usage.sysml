// An attribute usage owned by a definition, with no owned subsetting or
// redefinition and no declared multiplicity, has the implicit [1..1] (SysML v2
// §7.6.3), so a constraint reading it reads one value: the evaluator checks the
// value held and the solver encodes one scalar variable for it.
package test {
	private import ScalarValues::Real;

	part def Tank {
		attribute level : Real = 2.0;
		constraint nonNegative { level >= 0.0 }
	}

	part tank : Tank;
}
