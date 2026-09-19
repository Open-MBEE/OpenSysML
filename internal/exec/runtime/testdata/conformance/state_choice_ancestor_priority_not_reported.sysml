package test {
	// The substate `fine` and the composite `running` enclosing it both react to
	// Tick with a holding guard. UML orders these: the innermost enabled transition
	// fires, so this is a rule, not a choice, and no choice point is reported.
	state Monitor {
		attribute temp : Integer = 30;
		attribute path : Integer = 0;

		entry; then running;
		state running {
			entry; then fine;
			state fine;
			state warm {
				entry { assign path := 1; }
			}

			transition first fine accept Tick if temp > 20 then warm;
		}
		state halted {
			entry { assign path := 2; }
		}

		transition first running accept Tick if temp > 10 then halted;
	}
}
