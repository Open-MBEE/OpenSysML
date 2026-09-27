// `via tx` in a machine the part declares names the performing part's own port
// tx: the send crosses that port's connector rather than looking elsewhere.
package SendViaThisPort {
	item def Ping;

	port def PingOut {
		out item ping : Ping;
	}
	port def PingIn {
		in item ping : Ping;
	}

	part def Node {
		port tx : PingOut;
		port rx : PingIn;
		connect tx to rx;

		state Machine {
			entry; then begin;
			state begin;

			state sending {
				entry send new Ping() via tx;
			}
			state heard;

			succession first begin then sending;

			transition first sending
				accept Ping via rx
				then heard;
		}
	}

	part solo : Node;
}
