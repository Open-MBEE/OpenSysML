// An accept that names a payload binds it to an occurrence of the signal when
// the send constructed the signal rather than a value: `send new Telemetry(frames = 3.0)`
// binds `t` to a Telemetry object whose `frames` the effect reads, and a
// payload-less `send new Ping()` still binds its accept's name.
package test {
	private import ScalarValues::*;

	attribute def Telemetry {
		attribute frames : Real;
	}

	attribute def Ping;

	port def XBand;

	part def Probe {
		port antenna : XBand;

		exhibit state beacon {
			entry; then sending;

			state sending {
				entry action downlink {
					send new Telemetry(frames = 3.0) via antenna;
					send new Ping() via antenna;
				}
			}
		}
	}

	part def Station {
		port dish : XBand;
		attribute frames : Real = 0.0;
		attribute pinged : Integer = 0;

		exhibit state listen {
			entry; then standingBy;

			state standingBy;

			state logged;

			state acknowledged;

			transition first standingBy
				accept t : Telemetry via dish
				do assign frames := frames + t.frames
				then logged;

			transition first logged
				accept p : Ping via dish
				do assign pinged := pinged + 1
				then acknowledged;
		}
	}

	part def Site {
		part probe : Probe;
		part station : Station;

		connect probe.antenna to station.dish;
	}

	part site : Site;
}
