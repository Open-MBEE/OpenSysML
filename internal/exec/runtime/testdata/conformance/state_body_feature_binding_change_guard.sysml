// The machine's transition polling observes a change in the live data binding.
package test {
	private import ScalarValues::*;

	state Machine {
		attribute a : Integer := 0;
		attribute s : Integer = a * 2;
		attribute observed : Integer := 0;

		entry; then waiting;
		state waiting {
			do action arm { assign a := 5; }
		}
		state done;

		transition first waiting accept when s > 3 do {
			attribute effectValue : Integer = a * 2;
			assign a := 6;
			assign observed := effectValue;
		} then done;
	}
}
