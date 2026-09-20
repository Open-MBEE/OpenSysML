// An action's own connector may end at a bound port reference by its bare name:
// `connect src.tx to rx` with `rx` bound to `bound.local` joins the source's port to
// the bound listener's, so a send over it reaches that listener and never the
// performer's same-named port.
package BehaviorConnectionBoundPortEnd {
	item def Go;
	item def Ping;

	port def PingOut {
		out item ping : Ping;
	}

	part def Device {
		port tx : PingOut;
	}

	part def Listener {
		port local : ~PingOut;

		exhibit state life {
			entry; then idle;
			state idle;
			transition first idle accept Ping via local then heard;
			state heard;
		}
	}

	action def Fire {
		in ref src : Device;
		in ref port rx : ~PingOut;
		connect src.tx to rx;
		first start then go;
		action go {
			send new Ping() via src.tx;
		}
		first go then done;
	}

	part def Rig {
		port rx : ~PingOut;
		part device : Device;
		part bound : Listener;

		exhibit state life {
			entry; then idle;
			state idle;
			transition first idle accept Go then firing;
			state firing {
				do action fire : Fire {
					in ref :>> src = device;
					in ref port :>> rx = bound.local;
				}
			}
			state fired;
			state misfired;
			transition first firing accept Ping via rx then misfired;
			transition first firing then fired;
		}
	}
}
