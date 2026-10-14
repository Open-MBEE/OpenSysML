package test {
	private import ScalarValues::*;

	// One occurrence of Go fires a transition in each region of the parallel
	// state `work`, and both effects write `last`; the next occurrence, Next,
	// fires a transition whose effect computes `computed` from `last` and writes
	// `fixed` from a literal. The library orders neither region's reaction before
	// the other's, so `last` stands open, and so does `computed`, computed from it
	// after both reactions; `fixed` is the same in every order.
	attribute def Go;
	attribute def Next;
	state Machine {
		attribute last : Integer = 0;
		attribute computed : Integer = 0;
		attribute fixed : Integer = 0;

		entry; then work;
		state work parallel {
			state a {
				entry; then a1;
				state a1;
				state a2;
				transition first a1 accept Go do assign last := 1 then a2;
			}
			state b {
				entry; then b1;
				state b1;
				state b2;
				transition first b1 accept Go do assign last := 2 then b2;
			}
		}
		state settled;
		transition first work accept Next do { assign computed := last + 1; assign fixed := 5; } then settled;
	}
}
