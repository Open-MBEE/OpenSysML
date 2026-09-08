// A transfer addressed to a port is received by that port, not by the machine
// performing the accept. With a via-less accept and an `accept … via inPort` on
// one state, only the `via` transition is enabled for it: the via-less one,
// declared first, is not a competing alternative and no choice is reported.
package AcceptPortAddressed {
	item def Ping;

	port def PingPort {
		in item ping : Ping;
	}

	state Listen {
		port inPort : PingPort;

		entry; then start;
		state start;

		state waiting {
			entry send new Ping() to inPort;
		}
		state strayed;
		state received;

		succession first start then waiting;

		transition first waiting
			accept Ping
			then strayed;

		transition first waiting
			accept Ping via inPort
			then received;
	}
}
