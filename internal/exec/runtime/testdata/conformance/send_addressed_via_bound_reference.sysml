// An addressed send over a via path rooted at a bound reference leaves the bound
// object's port and names one of that object's receivers: with `dev` bound to
// `device`, `via dev.tx to listener` reaches the device's `listener` machine alone.
package SendAddressedViaBoundReference {
	item def Go;
	item def Ping;

	port def PingOut {
		out item ping : Ping;
	}

	part def Device {
		port tx : PingOut;
		port rx : ~PingOut;
		connect tx to rx;

		exhibit state listener {
			entry; then idle;
			state idle;
			transition first idle accept Ping via rx then heard;
			state heard;
		}

		exhibit state other {
			entry; then idle;
			state idle;
			transition first idle accept Ping via rx then heard;
			state heard;
		}
	}

	action def Fire {
		in ref dev : Device;
		first start then go;
		action go {
			send new Ping() via dev.tx to listener;
		}
		first go then done;
	}

	part def Rig {
		part dev : Device;
		part device : Device;

		exhibit state life {
			entry; then idle;
			state idle;
			transition first idle accept Go then firing;
			state firing {
				do action fire : Fire {
					in ref :>> dev = device;
				}
			}
			state fired;
			transition first firing then fired;
		}
	}
}
