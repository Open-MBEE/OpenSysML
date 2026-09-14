// Two state machines on one clock: one's transition effect writes a feature the
// other's guard reads, so their dispatches are dependent and both routes are
// reached; a third machine touching only its own data is independent of both.
package test {
	private import ScalarValues::*;

	part def Dial {
		attribute level : Integer = 0;
	}

	part dial : Dial;

	state turner {
		entry; then idle;
		state idle;
		state turned;
		transition first idle do assign dial.level := 9 then turned;
	}

	state router {
		attribute route : Integer = 0;
		entry; then idle;
		state idle;
		choice pick;
		state low { entry { assign route := 1; } }
		state high { entry { assign route := 2; } }
		succession first idle then pick;
		transition first pick if dial.level < 5 then low;
		transition first pick if dial.level >= 5 then high;
	}

	state loner {
		attribute steps : Integer = 0;
		entry; then one;
		state one;
		state two;
		transition first one do assign steps := steps + 1 then two;
		transition first two do assign steps := steps + 1 then done;
	}
}
