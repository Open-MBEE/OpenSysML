// Two objects of one type each select a different variant of one variation
// interface. A machine performed by an object routes what it sends over that
// object's own connections, so each object's message arrives at the port its
// selected variant connects the sending port to.
package VariantRouting {
	item def Ping;

	port def PingPort {
		in item ping : Ping;
	}

	part def Router {
		port outPort : PingPort;
		port inPort : PingPort;
		port bypass : PingPort;

		variation interface link {
			variant interface direct connect outPort to inPort;
			variant interface indirect connect outPort to bypass;
		}

		state Route {
			entry; then start;
			state start;

			state sending {
				entry send new Ping() via outPort;
			}
			state arrived;
			state diverted;

			succession first start then sending;

			transition first sending
				accept Ping via inPort
				then arrived;

			transition first sending
				accept Ping via bypass
				then diverted;
		}
	}

	part alpha : Router {
		ref :>> link = link::direct;
	}

	part beta : Router {
		ref :>> link = link::indirect;
	}
}
