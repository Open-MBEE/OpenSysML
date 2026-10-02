package test {
	private import ScalarValues::*;

	// A send written directly among an action's members is a subaction like any
	// other: no succession orders it against `counter`, so both start with the
	// action, and `counter`'s accept takes the message whichever runs first.
	action communicator {
		attribute number : Integer := 0;
		send 5 to counter;
		action counter accept n : Integer;
		then action recorder { assign number := n; }
	}
}
