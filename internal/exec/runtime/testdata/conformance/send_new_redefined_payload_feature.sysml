// A payload feature redefined under another name is one feature: it takes one
// position in `new Sub(4.0, 6.0)` (`b`, then `k`), binds by its new name in
// `new Sub(b = 5.0)`, and the accepted object reads it back under that name.
package test {
	private import ScalarValues::*;

	attribute def Base {
		attribute a : Real;
		attribute k : Real;
	}

	attribute def Sub :> Base {
		attribute b redefines a;
	}

	port def XBand;

	part def Probe {
		port antenna : XBand;

		exhibit state beacon {
			entry; then sending;

			state sending {
				entry action downlink {
					send new Sub(4.0, 6.0) via antenna;
					send new Sub(b = 5.0) via antenna;
				}
			}
		}
	}

	part def Station {
		port dish : XBand;
		attribute positional : Real = 0.0;
		attribute labelled : Real = 0.0;

		exhibit state listen {
			entry; then standingBy;

			state standingBy;

			state gotPositional;

			state gotLabelled;

			transition first standingBy
				accept t : Sub via dish
				do assign positional := t.b * 10.0 + t.k
				then gotPositional;

			transition first gotPositional
				accept u : Sub via dish
				do assign labelled := u.b
				then gotLabelled;
		}
	}

	part def Site {
		part probe : Probe;
		part station : Station;

		connect probe.antenna to station.dish;
	}

	part site : Site;
}
