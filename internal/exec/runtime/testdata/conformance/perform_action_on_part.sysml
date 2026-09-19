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
			first start then 'set azimuth';
			action 'set azimuth' {
				in value;
				assign this.azimuth := value;
			}
			bind 'set azimuth'.value = az;
			first 'set azimuth' then done;
		}

		action point : Point;
	}

	part def Station {
		part tel : Telescope;
		attribute azimuth : Real default = 0.0;

		action def Face {
			in az : Real;
			first start then turn;
			action turn {
				assign this.azimuth := az;
			}
			first turn then done;
		}

		action def Observe {
			first start then pick;
			action pick {
				out result : Real;
				assign result := 45.0;
			}
			first pick then pt;
			perform action pt ::> tel.point;
			flow pick.result to pt.az;
			first pt then own;
			action own : Face {
				in az = 90.0;
			}
			first own then done;
		}

		exhibit state run {
			entry; then observing;

			state observing {
				entry action observe : Observe;
			}
			state pointed;

			transition first observing if tel.azimuth == 45.0 and azimuth == 90.0 then pointed;
		}
	}
}
