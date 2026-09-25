// A join completes one compound transition: the effect of every transition
// into it runs once the sources are exited, before the state owning the join
// is exited and the join's outgoing effect runs.
package Test {
	state Machine {
		attribute log : String = "";

		entry; then start;
		state start;
		state Outer {
			exit { assign log := log + "outer(exit) "; }

			state Work parallel {
				state left {
					entry; then l1;
					state l1;
					transition first l1 do assign log := log + "left(effect) " then sync;
				}
				state right {
					entry; then r1;
					state r1;
					transition first r1 do assign log := log + "right(effect) " then sync;
				}
			}
			join sync;
			transition first sync do assign log := log + "sync(effect) " then Rest;
		}
		state Rest {
			entry { assign log := log + "rest(entry)"; }
		}

		transition first start when go then Work;
		transition first Rest then done;
	}
}
