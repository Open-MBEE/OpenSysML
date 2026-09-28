// `perform action pt ::> tel.point` performs the telescope's action as the
// telescope: the chain's operand is evaluated in the caller and the callee's
// `this` is the object it denotes, so the telescope's azimuth moves. The plain
// nested call beside it keeps the station as `this`, so the station's own
// same-named attribute takes the other write and the machine's guard sees both.
package test {
	private import ScalarValues::*;

	part def Telescope {
		attribute azimuth : Real default = 0.0;

		action def Point {
			in az : Real;
			in ref context : Telescope;
			first start then 'set azimuth';
			action 'set azimuth' {
				in value;
				assign context.azimuth := value;
			}
			bind 'set azimuth'.value = az;
			first 'set azimuth' then done;
		}

		action point : Point { in ref :>> context = this; }
	}

	part def Station {
		part tel : Telescope;
		attribute azimuth : Real default = 0.0;

		action def Face {
			in az : Real;
			in ref context : Station;
			first start then turn;
			action turn {
				assign context.azimuth := az;
			}
			first turn then done;
		}

		action def Observe {
			in ref context : Station;
			first start then pick;
			action pick {
				out result : Real;
				assign result := 45.0;
			}
			first pick then pt;
			perform action pt : Telescope::Point ::> context.tel.point;
			flow pick.result to pt.az;
			first pt then own;
			action own : Face {
				in az = 90.0; in ref :>> context = Observe::context;
			}
			first own then done;
		}

		exhibit state run {
			entry; then observing;

			state observing {
				entry action observe : Observe { in ref :>> context = this; }
			}
			state pointed;

			transition first observing if tel.azimuth == 45.0 and azimuth == 90.0 then pointed;
		}
	}
}
