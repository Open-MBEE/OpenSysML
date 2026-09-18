// A send addressing `this.<part>` from an action def owned by a part def
// delivers to that part of the performing object: `this` is the context
// occurrence the performance runs for, so the path is read from the host.
package SendThisPart {
	private import ScalarValues::*;

	item def Go {
		attribute n : Integer;
	}

	part def Worker {
		attribute got : Integer = 0;

		state def Listen {
			entry; then idle;
			state idle;
			transition first idle accept g : Go do assign this.got := g.n then idle;
		}

		exhibit state listen : Listen;
	}

	part def Host {
		attribute mark : Integer = 0;
		part worker : Worker;

		action def Notify {
			first start;
			then action poke send new Go(n = 7) to this.worker;
			then action note assign this.mark := 1;
			then done;
		}

		perform action notify : Notify;
	}

	part host : Host;
}
