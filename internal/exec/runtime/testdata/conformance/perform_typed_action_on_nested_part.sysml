// `perform action sp : Spin ::> drive.motor.spin` names the action's definition
// and the usage it performs: the typing lets the caller read `sp.result`, while
// the reference makes the motor two parts down the callee's `this`. Without the
// reference the typed nested usage would perform Spin as the rig, which has no
// speed to set.
package test {
	private import ScalarValues::*;

	part def Motor {
		attribute speed : Real default = 0.0;

		action def Spin {
			in rpm : Real;
			out result : Real;
			in ref context : Motor;
			first start then 'set speed';
			action 'set speed' {
				in value;
				assign context.speed := value;
			}
			bind 'set speed'.value = rpm;
			first 'set speed' then 'read speed';
			action 'read speed' {
				out result = context.speed;
			}
			bind result = 'read speed'.result;
			first 'read speed' then done;
		}

		action spin : Spin { in rpm; out result; in ref :>> context = this; }
	}

	part def Drive {
		part motor : Motor;
	}

	part def Rig {
		part drive : Drive;
		attribute got : Real default = 0.0;

		action def Spinup {
			in ref context : Rig;
			first start then sp;
			perform action sp : Motor::Spin ::> context.drive.motor.spin { in rpm = 30.0; }
			first sp then keep;
			action keep {
				assign context.got := sp.result;
			}
			first keep then done;
		}

		exhibit state run {
			entry; then spinning;

			state spinning {
				entry action spinup : Spinup { in ref :>> context = this; }
			}
			state spun;

			transition first spinning if drive.motor.speed == 30.0 and got == 30.0 then spun;
		}
	}
}
