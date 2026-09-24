// A constructor's positional arguments bind the features an object of the type
// carries: a calc written between two attributes holds no value, so
// `new Frame(1, 7)` binds `x` and `y` and leaves the calc where it is.
package test {
	private import ScalarValues::*;

	item def Frame {
		attribute x : Integer;
		calc doubled { x * 2 }
		attribute y : Integer;
	}

	port def Link;

	part def Probe {
		port link : Link;

		exhibit state beacon {
			entry; then sending;

			state sending {
				entry action downlink {
					send new Frame(1, 7) via link;
				}
			}
		}
	}

	part def Station {
		port dish : Link;
		attribute packed : Integer = 0;

		exhibit state listen {
			entry; then standingBy;

			state standingBy;

			state logged;

			transition first standingBy
				accept f : Frame via dish
				do assign packed := f.x + 10 * f.y
				then logged;
		}
	}

	part def Site {
		part probe : Probe;
		part station : Station;

		connect probe.link to station.dish;
	}

	part site : Site;
}
