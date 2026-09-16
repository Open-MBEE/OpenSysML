// A signal dispatched at the instant a time-triggered join segment's timer is
// due is not that timer's expiry: `Go`, sent by `c`'s timer at t=1 and so tied
// with `b`'s timer due then, enables `a`'s segment but not `b`'s, whichever of
// the two is dispatched first; the segment dispatched second finds the other's
// occurrence gone. The join never fires; only the order of `c`'s and `d`'s
// timers is open.
package Test {
	private import ScalarValues::*;
	private import SI::*;

	item def Go;

	state Machine {
		attribute log : String = "";

		entry; then Work;
		state Work parallel {
			state a {
				entry; then a1;
				state a1;
				transition first a1 accept Go do assign log := log + "a " then sync;
			}
			state b {
				entry; then b1;
				state b1;
				transition first b1 accept after 1 [s] do assign log := log + "b " then sync;
			}
			state c {
				entry; then c1;
				state c1;
				state c2;
				transition first c1 accept after 1 [s] do action { assign log := log + "c "; send new Go() to Machine; } then c2;
			}
			state d {
				entry; then d1;
				state d1;
				state d2;
				transition first d1 accept after 1 [s] do assign log := log + "d " then d2;
			}
		}
		join sync;
		transition first sync do assign log := log + "sync" then done;
		state done;
	}
}
