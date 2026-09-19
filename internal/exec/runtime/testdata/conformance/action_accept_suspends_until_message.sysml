package test {
	private import ScalarValues::*;

	// An accept reached before its message suspends the action rather than
	// failing. The fork puts `reader` and the sending branch in parallel, and
	// `gate` delays the send by one step, so `reader` is parked at its accept
	// before `sender` posts anything: `received` is 7 only if the parked token
	// resumed when the message arrived.
	action communicator {
		attribute received : Integer = 0;

		first start;

		fork split;

		action reader accept n : Integer;

		action gate;

		action sender {
			send 7 to reader;
		}

		join sync;

		action recorder {
			assign received := n;
		}

		done;

		succession first start then split;
		succession first split then reader;
		succession first split then gate;
		succession first reader then sync;
		succession first gate then sender;
		succession first sender then sync;
		succession first sync then recorder;
		succession first recorder then done;
	}
}
