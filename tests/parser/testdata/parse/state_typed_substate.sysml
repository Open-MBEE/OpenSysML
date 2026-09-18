// A substate typed by a state definition, with and without a body of its own,
// and a typed usage in a parallel body.
package Test {
	state def Inner {
		entry; then i1;
		state i1;
	}

	state def Machine {
		entry; then nested;
		state nested : Inner;
		state extended : Inner {
			state i2;
		}
		state running parallel {
			state a : Inner;
			state b : Inner;
		}
	}
}
