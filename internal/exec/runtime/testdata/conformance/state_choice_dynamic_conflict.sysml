// Two branches of a choice are enabled only by the value the transition into it
// writes: read after that effect, both hold, and which one is taken is open. The
// choice is reported as a choice point, so a sweep enumerates both runs.
package Test {
	state Machine {
		attribute level : Integer = 0;
		attribute route : Integer = 0;

		entry; then idle;
		state idle;
		choice pick;
		state low { entry { assign route := 1; } }
		state high { entry { assign route := 2; } }

		transition first idle accept Go do assign level := 8 then pick;
		transition first pick if level > 5 then low;
		transition first pick if level > 7 then high;
	}
}
