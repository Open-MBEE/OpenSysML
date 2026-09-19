package test {
	private import ScalarValues::*;

	// `accept when <condition>` waits for a change: the condition is re-tested
	// every step, and the token leaves the accept the step it holds.
	action monitor {
		attribute temp : Integer = 10;
		attribute alerted : Integer = 0;

		first start;

		action heat {
			assign temp := 40;
		}

		action awaitWarm accept when temp > 20;

		action alert {
			assign alerted := 1;
		}

		done;

		succession first start then heat;
		succession first heat then awaitWarm;
		succession first awaitWarm then alert;
		succession first alert then done;
	}
}
