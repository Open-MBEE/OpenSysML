// A send addressing `context.<part>` from an action def owned by a part def
// delivers to that part of the performing object: the def takes the performing
// object as its `context` parameter, so the path is read from the host.
package SendThisPart {
	private import ScalarValues::*;

	item def Go {
		attribute n : Integer;
	}

	part def Worker {
		attribute got : Integer = 0;

		state def Listen {
			in ref context : Worker;
			entry; then idle;
			state idle;
			transition first idle accept g : Go do assign context.got := g.n then idle;
		}

		exhibit state listen : Listen { in ref :>> context = this; }
	}

	part def Host {
		attribute mark : Integer = 0;
		part worker : Worker;

		action def Notify {
			in ref context : Host;
			first start;
			then action poke send new Go(n = 7) to context.worker;
			then action note assign context.mark := 1;
			then done;
		}

		perform action notify : Notify { in ref :>> context = this; }
	}

	part host : Host;
}
