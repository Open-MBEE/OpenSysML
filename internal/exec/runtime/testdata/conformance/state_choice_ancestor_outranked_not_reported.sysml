package test {
	// Region `a` reacts to Go from its own substate `a1`, so the transitions out
	// of the enclosing parallel state `work` lose to it (UML orders the innermost
	// enabled transition first) although both of their guards hold. The
	// alternatives out of `work` were never the run's to choose among: no choice
	// point is reported.
	state Machine {
		attribute level : Integer = 8;
		attribute route : Integer = 0;

		entry; then work;
		state work parallel {
			state a {
				entry; then a1;
				state a1;
				state a2 {
					entry { assign route := 3; }
				}
				transition first a1 accept Go then a2;
			}
			state b { entry; then b1; state b1; }
		}
		state low {
			entry { assign route := 1; }
		}
		state high {
			entry { assign route := 2; }
		}

		transition first work accept Go if level > 5 then low;
		transition first work accept Go if level > 7 then high;
	}
}
