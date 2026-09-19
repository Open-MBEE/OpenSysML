package test {
	private import ScalarValues::*;

	// A message is typed by what was sent, and an accept takes only the type it
	// declares. Both messages are addressed to `counter`, and the String is sent
	// first, so a queue that ignored types would bind the String: `number` is 50
	// only if the accept skipped it and took the Integer.
	action communicator {
		attribute number : Integer = 0;

		first start;

		action sender {
			send "high" to counter;
			send 50 to counter;
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
