// An instant names a time on the same clock a duration is measured on: a
// TimeInstantValue of 2 [min] is due 120 seconds in.
package test {
	private import SI::*;
	private import Time::*;

	state Machine {
		attribute log : Integer = 0;
		attribute due : TimeInstantValue = 2 [min];

		entry; then start;
		state start;
		state waiting;
		accept at due then done;
		state done {
			entry { assign log := log * 10 + 1; }
		}

		succession first start then waiting;
	}
}
