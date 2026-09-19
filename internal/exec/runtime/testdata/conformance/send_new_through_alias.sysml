// A constructor written through an alias sends the definition the alias names:
// `send new Report(frames = 3.0)` with `alias Report for Telemetry` is a
// Telemetry message, so an accept typed either way binds it and reads `frames`.
package SendNewThroughAlias {
	private import ScalarValues::*;

	attribute def Telemetry {
		attribute frames : Real;
	}

	alias Report for Telemetry;

	port def XBand;

	part def Probe {
		port antenna : XBand;

		exhibit state beacon {
			entry; then sending;

			state sending {
				entry action downlink {
					send new Report(frames = 3.0) via antenna;
					send new Report(frames = 4.0) via antenna;
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

			state confirmed;

			transition first standingBy
				accept t : Telemetry via dish
				do assign frames := frames + t.frames
				then logged;

			transition first logged
				accept r : Report via dish
				do assign frames := frames + r.frames
				then confirmed;
		}
	}

	part def Site {
		part probe : Probe;
		part station : Station;

		connect probe.antenna to station.dish;
	}

	part site : Site;
}
