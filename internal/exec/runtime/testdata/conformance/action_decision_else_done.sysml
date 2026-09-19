package test {
	private import ScalarValues::*;

	action watchdog {
		attribute battery : Integer = 80;
		attribute alerted : Integer = 0;

		first start;

		action sendBatteryLow {
			assign alerted := 1;
		}

		decide check;
		succession first start then check;
		if battery < 20 then sendBatteryLow;
		else done;
		succession first sendBatteryLow then done;
	}
}
