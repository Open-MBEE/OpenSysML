package ParameterValues {
	port def PowerPort {
		out attribute power : ScalarValues::Real;
	}

	action def Consume {
		in power : ScalarValues::Real;
		in gain : ScalarValues::Real;
	}

	part def Motor {
		port powerPort : PowerPort;

		perform action consume : Consume {
			in power = powerPort.power;
			in gain = 0.5;
		}

		action boost : Consume {
			in power = powerPort.power * 2.0;
		}
	}
}
