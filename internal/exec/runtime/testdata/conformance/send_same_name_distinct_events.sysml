// Two same-named event features of different parts are distinct: a message sent
// from one does not satisfy an accept subsetting the other, though both are
// written `alert`.
package SendSameNameDistinctEvents {
	private import ScalarValues::*;

	item def Ping;

	port def PingPort {
		out item issued : Ping;
	}

	part def Console {
		port p : PingPort;
		item alert : Ping;
		attribute sent : Integer = 0;

		exhibit state dispatch {
			entry; then idle;

			state idle;

			state sending {
				entry action issue {
					send alert via p;
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
