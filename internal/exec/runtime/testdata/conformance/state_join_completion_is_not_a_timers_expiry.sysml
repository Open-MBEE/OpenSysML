// A completion segment arriving before a timer segment waits at the join; the
// timer's own expiry completes it, with either arrival order observable.
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
