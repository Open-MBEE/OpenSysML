// Two unconnected parts of one type each own a port named inPort. A send
// addressing alpha's port reaches that object's port alone, so alpha accepts it
// there while beta — structurally identical, joined to alpha by nothing — never
// sees it and stays where it was.
package SendIdentity {
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
				entry send new Ping() to alpha.inPort;
			}
			state received;
			state strayed;

			succession first start then waiting;

			transition first waiting
				accept Ping via inPort
				then received;

			transition first waiting
				accept Ping
				then strayed;
		}
	}

	part alpha : Node;
	part beta : Node;
}
