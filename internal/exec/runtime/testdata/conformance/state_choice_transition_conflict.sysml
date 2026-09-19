package test {
	// Oracle (docs/project/behavior-semantic-oracle.md): two transitions out of
	// `idle` are enabled by one Go, both guards holding for level = 8; the library
	// says one fires, not which. The executor fires the first declared and reports
	// the choice.
	state Dispatcher {
		attribute level : Integer = 8;
		attribute route : Integer = 0;

		entry; then idle;
		state idle;
		state low {
			entry { assign route := 1; }
		}
		state high {
			entry { assign route := 2; }
		}

		transition first idle accept Go if level > 5 then low;
		transition first idle accept Go if level > 7 then high;
	}
}
