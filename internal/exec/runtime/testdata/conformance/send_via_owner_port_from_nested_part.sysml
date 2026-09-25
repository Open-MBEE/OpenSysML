// A behavior running in a part nested inside the sender — here a timeslice of
// the probe — sends through the probe's own port: the via port belongs to an
// ancestor, and the owner's connector still routes the message to the station.
package test {
	private import ScalarValues::*;

	attribute def Telemetry {
		attribute frames : Real;
	}

	port def XBand;

	part def Probe {
		port antenna : XBand;

		timeslice part cruise {
			exhibit state beacon {
				entry; then sending;

				state sending {
					entry action downlink {
						send new Telemetry(frames = 3.0) via antenna;
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

		connect probe.antenna to station.dish;
	}

	part mission : Mission;
}
