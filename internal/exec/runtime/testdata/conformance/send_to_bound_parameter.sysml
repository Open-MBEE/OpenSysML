// A send addressing a parameter of the sending action delivers to the object
// the caller bound to it: the target is read from the performance's own frame,
// not walked as a feature of the performing object, so one action notifies
// whichever part it is handed, and a chain from the parameter reaches through it.
package SendToBoundParameter {
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

	part def Crew {
		part lead : Worker;
	}

	part def Host {
		part left : Worker;
		part right : Worker;
		part crew : Crew;

		action def Notify {
			in recipient : Worker;
			in n : Integer;
			first start;
			then action poke send new Go(n = n) to recipient;
			then done;
		}

		action def Brief {
			in team : Crew;
			first start;
			then action poke send new Go(n = 5) to team.lead;
			then done;
		}

		action def Round {
			first start;
			then action toRight : Notify { in recipient = this.right; in n = 7; }
			then action toLeft : Notify { in recipient = this.left; in n = 3; }
			then action toCrew : Brief { in team = this.crew; }
			then done;
		}

		perform action round : Round;
	}

	part host : Host;
}
