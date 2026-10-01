// Oracle: A junction with no way through: the transition is not enabled, and the occurrence is handled as unmatched.
package Test {
	attribute def Start;

	state Machine {
		entry; then wait;
		state wait;
		state other;
		state unused;
		junction dead;

		transition first wait accept Start then dead;
		transition first wait accept Start then other;
		transition first dead if false then unused;
	}
}
