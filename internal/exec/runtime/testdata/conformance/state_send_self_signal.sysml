package test {
	// A machine that drives itself: the entry action of `waiting` sends Ping,
	// and the only transition out of `waiting` accepts Ping. Nothing is injected
	// from outside, so `done` is reached only if the sent message actually
	// reaches the machine's own event queue.
	item def Ping;

	state Driver {
		entry; then start;
		state start;

		state waiting {
			entry { send Ping to Driver; }
		}


		succession first start then waiting;

		transition first waiting when Ping then done;
	}
}
