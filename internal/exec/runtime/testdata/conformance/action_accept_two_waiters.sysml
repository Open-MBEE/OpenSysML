package test {
	private import ScalarValues::*;

	// Two accepts park at once, in parallel branches, and each resumes on the
	// message of its own type: the String sent first does not resume the
	// Integer accept, and neither branch consumes the other's message.
	action communicator {
		attribute number : Integer = 0;
		attribute message : String = "none";

		first start;

		fork split;

		action listenNumber accept n : Integer;

		action listenText accept t : String;

		action gate;

		action sender {
			send "go" to listenText;
			send 5 to listenNumber;
		}

		join sync;

		action recorder {
			assign number := n;
			assign message := t;
		}

		done;

		succession first start then split;
		succession first split then listenNumber;
		succession first split then listenText;
		succession first split then gate;
		succession first listenNumber then sync;
		succession first listenText then sync;
		succession first gate then sender;
		succession first sender then sync;
		succession first sync then recorder;
		succession first recorder then done;
	}
}
