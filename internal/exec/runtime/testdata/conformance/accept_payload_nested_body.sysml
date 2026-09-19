package test {
	private import ScalarValues::*;

	// The payload an accept node binds is read from a nested if and loop body of
	// a sibling node: those bodies resolve through the action body the payload
	// belongs to.
	action communicator {
		attribute total : Integer = 0;

		first start;

		action sender {
			send 7 to receiver;
		}

		action receiver accept msg : Integer;

		action processor {
			if msg > 5 { assign total := msg * 2; }
			while total < 20 { assign total := total + msg; }
		}

		done;

		succession first start then sender;
		succession first sender then receiver;
		succession first receiver then processor;
		succession first processor then done;
	}
}
