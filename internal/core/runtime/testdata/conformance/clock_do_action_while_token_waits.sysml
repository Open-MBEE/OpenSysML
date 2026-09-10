// A state's do action runs while an action token waits on the clock: the
// worker enters `resting` at t=2 and its do behavior runs there, all before
// the action's 5 s wait ends and it reads the result.
package test {
	private import SI::*;
	private import ScalarValues::*;

	part def Worker {
		attribute ticks : Integer = 0;

		exhibit state shift {
			entry; then working;

			state working;
			accept after 2 [s] then resting;
			state resting {
				entry assign ticks := ticks + 10;
				do action rest assign ticks := ticks + 1;
			}
		}
	}

	part worker : Worker;

	action supervisor {
		attribute before : Integer = -1;
		attribute after : Integer = -1;

		first start;
		then action arm assign before := worker.ticks;
		then action wait accept after 5 [s];
		then action read assign after := worker.ticks;
		then done;
	}
}
