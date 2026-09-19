// A behavior nested in a timeslice sends through a dotted port path — a port
// of a part the probe holds — and the owner-level connector attached to that
// nested port still routes the message to the station.
package test {
	private import ScalarValues::*;

	attribute def Telemetry {
		attribute frames : Real;
	}

	port def XBand;

	part def Radio {
		port tx : XBand;
	}

	part def Probe {
		part radio : Radio;

		timeslice part cruise {
			exhibit state beacon {
				entry; then sending;

				state sending {
					entry action downlink {
						send new Telemetry(frames = 5.0) via radio.tx;
					}
				}
			}
		}
	}

	part def Station {
		port dish : XBand;
		attribute frames : Real = 0.0;

		exhibit state listen {
			entry; then standingBy;

			state standingBy;

			state logged;

			transition first standingBy
				accept t : Telemetry via dish
				do assign frames := frames + t.frames
				then logged;
		}
	}

	part def Mission {
		part probe : Probe;
		part station : Station;

		connect probe.radio.tx to station.dish;
	}

	part mission : Mission;
}
