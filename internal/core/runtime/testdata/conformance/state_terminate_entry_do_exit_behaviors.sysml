// `terminate` in a state's entry, do or exit behavior ends that behavior at the
// statement: the state stays active and the machine keeps dispatching. The
// transition out is timed so the do body, run one statement a round, reaches
// its `terminate` before anything interrupts it.
package test {
	private import ScalarValues::*;

	state def Machine {
		attribute count : Integer = 0;
		attribute later : Integer = 0;

		entry; then idle;

		state idle {
			entry action begin {
				assign count := 1;
				terminate;
				assign count := 99;
			}
			do action tick {
				assign later := 5;
				terminate;
				assign later := 77;
			}
			exit action leave {
				assign count := count + 10;
				terminate;
				assign count := 0;
			}
		}
		state next;

		transition first idle accept after 1 then next;
	}
}
