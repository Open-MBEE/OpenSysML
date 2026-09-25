package test {
	private import ScalarValues::*;

	// Only the body feature that redefines SendAction::payload is the message a
	// send with no argument carries: binding some other input leaves it without one.
	action communicator {
		attribute number : Integer = 0;

		first start;

		action sender {
			send to counter { in delay : Integer = 50; }
		}

		action counter accept n : Integer;

		action recorder {
			assign number := n;
		}

		done;

		succession first start then sender;
		succession first sender then counter;
		succession first counter then recorder;
		succession first recorder then done;
	}
}
