// An accept subsetting an event feature waits for a message sent from that
// feature: a message of a type merely spelled like the feature does not satisfy
// it, even where the type resolves and the feature does too.
package SendTypedMessageIsNotEvent {
	private import ScalarValues::*;

	item def Ping;

	package Signals {
		item def alert;
	}

	port def PingPort {
		out item issued : Ping;
		out item typed : Signals::alert;
	}

	part def Console {
		port p : PingPort;
		attribute sent : Integer = 0;

		exhibit state dispatch {
			entry; then idle;

			state idle;

			state sending {
				entry action issue {
					send new Signals::alert() via p;
					assign sent := sent + 1;
				}
			}

			succession first idle then sending;
		}
	}

	part def Unit {
		port p : ~PingPort;
		item alert : Ping;
		attribute accepted : Integer = 0;

		exhibit state duty {
			entry; then standingBy;

			state standingBy;

			state working {
				entry action count {
					assign accepted := accepted + 1;
				}
			}

			transition first standingBy
				accept :> alert via p
				then working;
		}
	}

	part def Site {
		part console : Console;
		part unit : Unit;

		connect console.p to unit.p;
	}

	part site : Site;
}
