// An attribute usage stating no multiplicity on an action definition has the implicit [1..1]
// (SysML v2 §7.6.3) as one on a part definition does (write_implicit_one_part_def): the
// action's own body cannot write two values into it.
package test {
	private import ScalarValues::Real;

	action def A {
		attribute z : Real;
		assign z := (3.0, 4.0);
	}
}
