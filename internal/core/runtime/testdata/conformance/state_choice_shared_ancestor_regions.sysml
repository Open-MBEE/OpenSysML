package test {
	// Oracle (docs/project/behavior-semantic-oracle.md): two transitions out of
	// the parallel state `work` are enabled by one Go. Both regions select the
	// same transition out of `work`, which the event takes once; the choice among
	// the transitions out of `work` is likewise made once and reported once.
	state Machine {
		attribute level : Integer = 8;
		attribute route : Integer = 0;

		entry; then work;
		state work parallel {
			state a { entry; then a1; state a1; }
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
