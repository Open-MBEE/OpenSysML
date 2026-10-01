// Oracle: A junction with no way through: the transition is not enabled, and the occurrence is handled as unmatched.
package Test {
	private import ScalarValues::*;
	attribute def Continue;

	state Machine {
		attribute log : String = "";

		entry; then a;
		state a;
		state s2;
		state unused;
		junction dead;

		transition first a then dead;
		transition first dead if false then unused;
		transition first a accept Continue do assign log := "T3" then s2;
	}
}
