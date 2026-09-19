// A machine whose active state's do behavior is parked at an accept of Go, a
// signal no transition of the machine accepts; Noise is accepted nowhere.
package Waiting {
	private import ScalarValues::*;
	attribute def Go;
	attribute def Noise;

	state def Waiter {
		attribute total : Integer = 0;
		entry; then active;
		state active {
			do action work {
				first start;
				then action reader accept Go;
				then action count assign total := total + 1;
				then done;
			}
		}
		succession first active then finished;
		state finished;
	}

	part def Box {
		exhibit state w : Waiter;
	}

	part box : Box;
}
