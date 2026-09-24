// An action's own connector joins the ports of two objects its reference parameters
// are bound to: `connect dev.p to snk.local` keeps its ends as written, so a
// `send … via dev.p` travels it to the bound sink's port. The send leaves the bound
// device's port too, so a connection the performer declares from that port carries a
// copy as well: both listeners hear it, and the sender's own same-named part is never
// consulted (SysML v2 §7.16).
package SendViaBoundReferenceBehaviorConnection {
	item def Go;
	item def Ping;

	port def PingOut {
		out item ping : Ping;
	}

	part def Device {
		port p : PingOut;
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
		in ref dev : Device;
		in ref snk : Listener;
		connect dev.p to snk.local;
		first start then go;
		action go {
			send new Ping() via dev.p;
		}
		first go then done;
	}

	part def Rig {
		part dev : Device;
		part device : Device;
		part bound : Listener;
		part wired : Listener;
		part decoy : Listener;
		connect device.p to wired.local;
		connect dev.p to decoy.local;

		exhibit state life {
			entry; then idle;
			state idle;
			transition first idle accept Go then firing;
			state firing {
				do action fire : Fire {
					in ref :>> dev = device;
					in ref :>> snk = bound;
				}
			}
			state fired;
			transition first firing then fired;
		}
	}
}
