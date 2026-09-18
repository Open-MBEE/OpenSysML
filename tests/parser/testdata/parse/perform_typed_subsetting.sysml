package PerformTyped {
	part def Motor {
		action def Spin {
			in rpm : Real;
			out result : Real;
		}
		action spin : Spin;
	}
	part def Rig {
		part motor : Motor;
		attribute got : Real;
		action def Spinup {
			perform action sp : Motor::Spin ::> motor.spin { in rpm = 30.0; }
			first start then sp;
			action keep {
				assign this.got := sp.result;
			}
			first sp then keep;
			first keep then done;
		}
	}
}
