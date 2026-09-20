// A via path that is just a bound reference to a port leaves that port: with `tx`
// bound to `device.tx`, `via tx` sends from the device's port, not from the
// performer's own same-named port, and `accept … via tx` hears at the device's port.
package SendViaBoundPortReference {
	item def Go;
	item def Ping;
	item def Pong;

	port def PingOut {
		out item ping : Ping;
		in item pong : Pong;
	}

	part def Device {
		port tx : PingOut;
	}

	part def Listener {
		port local : ~PingOut;
		attribute heard : Boolean = false;

		exhibit state life {
			entry; then idle;
			state idle;
			transition first idle accept Ping via local do send new Pong() via local then done;
			state done {
				entry assign heard := true;
			}
		}
	}

	action def Fire {
		in ref port tx : PingOut;
		out ref item reply : Pong;
		first start then go;
		action go {
			send new Ping() via tx;
		}
		first go then wait;
		action wait accept msg : Pong via tx;
		first wait then keep;
		action keep {
			assign reply := msg;
		}
		first keep then done;
	}

	part def Rig {
		port tx : PingOut;
		ref item lastReply : Pong[0..1];
		part device : Device;
		part wired : Listener;
		part decoy : Listener;
		connect device.tx to wired.local;
		connect tx to decoy.local;

		exhibit state life {
			entry; then idle;
			state idle;
			transition first idle accept Go then firing;
			state firing {
				do action run {
					first start then fire;
					action fire : Fire {
						in ref port :>> tx = device.tx;
					}
					first fire then keep;
					action keep {
						assign lastReply := fire.reply;
					}
					first keep then done;
				}
			}
			state fired;
			transition first firing then fired;
		}
	}
}
