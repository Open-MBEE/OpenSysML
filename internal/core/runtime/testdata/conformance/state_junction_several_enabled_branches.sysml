// Two branches out of a junction are enabled at once: both unguarded, so both
// hold when the compound transition is selected. Exactly one is taken, and
// which one is open: the junction is reported as a choice point, so a sweep
// enumerates both runs, each taking a different branch effect with it.
package Test {
	state Machine {
		attribute route : Integer = 0;

		entry; then idle;
		state idle;
		junction split;
		state left;
		state right;

		transition first idle accept Go then split;
		transition first split do assign route := 1 then left;
		transition first split do assign route := 2 then right;
	}
}
