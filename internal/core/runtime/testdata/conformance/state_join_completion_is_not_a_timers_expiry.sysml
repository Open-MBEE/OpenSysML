// A completion event dispatched at the instant a time-triggered join segment's
// timer is due is not that timer's expiry: a1's do behavior ends at t=1, and
// its completion, dispatched before b1's timer due then, enables a1's segment
// but not b1's, so it fires nothing; b1's expiry then finds a1 completed and
// fires the join.
package Test {
	private import ScalarValues::*;
	private import SI::*;

	state Machine {
		attribute log : String = "";

		entry; then Work;
		state Work parallel {
			state a {
				entry; then a1;
				state a1 {
					do action busy {
						first start;
						then action wait accept after 1 [s];
						then action mark assign log := log + "busy ";
						then done;
					}
				}
				transition first a1 do assign log := log + "a " then sync;
			}
			state b {
				entry; then b1;
				state b1;
				transition first b1 accept after 1 [s] do assign log := log + "b " then sync;
			}
		}
		join sync;
		transition first sync do assign log := log + "sync" then done;
		state done;
	}
}
