// A state definition owns a receive port (`port inbox`) that the exhibiting part
// connects directly (`connect response to m.inbox`), with no part port of its own
// for it; the reply is likewise connected directly to a second machine's port.
// Both machines accept through the performance's own port.
package test {
	item def Ping;
	item def Pong;
	port def CommandPort { in item request : Ping; out item reply : Pong; }

	state def Responder {
		port inbox : CommandPort;
		port response : ~CommandPort;
		entry; then idle;
		state idle;
		state replied;
		transition first idle accept Ping via inbox
			do send new Pong() via response
			then replied;
	}

	state def Counter {
		port ear : CommandPort;
		attribute heard : Integer = 0;
		entry; then waiting;
		state waiting;
		state counted;
		transition first waiting accept Pong via ear
			do assign heard := heard + 1
			then counted;
	}

	part def Tug {
		port response : ~CommandPort;
		exhibit state m : Responder;
		exhibit state n : Counter;
		interface wire connect response to m.inbox;
		interface back connect m.response to n.ear;
		perform action kick {
			first start;
			then action p send new Ping() via response;
			then done;
		}
	}
}
