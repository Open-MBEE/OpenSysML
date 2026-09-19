// A transfer addressed to the part itself is received by the part, the
// performer of its machine. The accept naming no port takes it; `accept … via
// inPort`, whose receiver is the port and is declared first, does not.
package AcceptPartAddressed {
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
				entry send new Ping() to alpha;
			}
			state onPort;
			state received;

			succession first start then waiting;

			transition first waiting
				accept Ping via inPort
				then onPort;

			transition first waiting
				accept Ping
				then received;
		}
	}

	part alpha : Node;
}
