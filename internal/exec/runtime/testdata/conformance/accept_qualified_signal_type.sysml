// An accept naming its type with a qualifier resolves that exact definition:
// where two same-named definitions are visible, `accept Beta::Ping` takes a
// Beta::Ping message and refuses an Alpha::Ping one.
package AcceptQualified {
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
		private import Alpha::*;
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

			transition wrongOne first standingBy
				accept order : Beta::Ping via p
				then working;
		}
	}

	part def Taker {
		private import Alpha::*;
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

			transition rightOne first standingBy
				accept order : Alpha::Ping via p
				then working;
		}
	}

	part def Site {
		part console : Console;
		part unit : Unit;
		part taker : Taker;

		connect console.p to unit.p;
		connect console.p to taker.p;
	}

	part site : Site;
}
