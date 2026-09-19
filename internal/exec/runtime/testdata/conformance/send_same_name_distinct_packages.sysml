// Two same-named item definitions in different packages are distinct: a message
// carrying one does not satisfy an accept whose name resolves to the other.
package SendSameNameDistinct {
	private import ScalarValues::*;

	package Alpha {
		item def Ping;
	}

	package Beta {
		item def Ping;
	}

	port def PingPort {
		out item issued : Alpha::Ping;
	}

	part def Console {
		port p : PingPort;
		attribute sent : Integer = 0;

		exhibit state dispatch {
			entry; then idle;

			state idle;

			state sending {
				entry action issue {
					send new Alpha::Ping() via p;
					assign sent := sent + 1;
				}
			}

			succession first idle then sending;
		}
	}

	part def Unit {
		private import Beta::*;

		port p : ~PingPort;
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
				accept order : Ping via p
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
