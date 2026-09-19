// An action sends a signal a state machine reacts to after a delay, on the
// clock the two share: the machine counts down 3 s from the signal, and the
// action, waiting 4 s, reads the count the countdown produced.
package test {
	private import SI::*;
	private import ScalarValues::*;

	attribute def Go;

	part def Listener {
		attribute count : Integer = 0;

		exhibit state reacting {
			entry; then idle;

			state idle;
			accept Go then countdown;
			state countdown;
			accept after 3 [s] then fired;
			state fired {
				entry assign count := count + 1;
			}
		}
	}

	part listener : Listener;

	action commander {
		attribute before : Integer = -1;
		attribute after : Integer = -1;

		first start;
		then action arm assign before := listener.count;
		then action go send new Go() to listener;
		then action wait accept after 4 [s];
		then action read assign after := listener.count;
		then done;
	}
}
