// Two nested objects addressing each other reach one another rather than
// materializing a second copy: `a` pings its sibling `b` while `b`'s reply is
// addressed back at the very object still being materialized, so the exchange
// settles instead of duplicating both until the run budget is spent.
package test {
	item def Ping;
	item def Pong;

	part def Node;

	part def Pair {
		part a : Node {
			exhibit state pinging {
				entry; then sending;

				state sending {
					entry send new Ping() to b;
				}
				accept Pong then answered;
				state answered;
			}
		}

		part b : Node {
			exhibit state replying {
				entry; then waiting;

				state waiting;
				accept Ping then replied;
				state replied {
					entry send new Pong() to a;
				}
			}
		}
	}
}
