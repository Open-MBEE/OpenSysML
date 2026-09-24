// An addressed send naming a port of the sending object delivers to that port of
// that object, so the accept listening on it takes the message.
package PortIdentity {
	item def Ping;

	port def PingPort {
		in item ping : Ping;
	}

	part def Node {
		port inPort : PingPort;

		state Listen {
			entry; then start;
			state start;

			state waiting {
				entry send new Ping() to inPort;
			}
			state received;

			succession first start then waiting;

			transition first waiting
				accept Ping via inPort
				then received;
		}
	}

	part solo : Node;
}
