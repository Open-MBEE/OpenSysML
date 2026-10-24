// A state definition owns `port response`; the part exhibiting it connects that
// port of the performance directly (`connect m.response to command`). The
// machine's `send … via response` leaves through the performance's port over
// that connector.
package test {
	item def Ping;
	item def Pong;
	port def CommandPort { in item request : Ping; out item reply : Pong; }

	state def Responder {
		port command : CommandPort;
		port response : ~CommandPort;
		entry; then idle;
		state idle;
		state replied;
		transition first idle accept Ping via command
			do send new Pong() via response
			then replied;
	}

	state def Counter {
		port command : CommandPort;
		attribute heard : Integer = 0;
		entry; then waiting;
		state waiting;
		state counted;
		transition first waiting accept Pong via command
			do assign heard := heard + 1
			then counted;
	}

	part def Tug {
		port command : CommandPort;
		exhibit state m : Responder;
		exhibit state n : Counter;
		interface wire connect m.response to command;
		perform action kick {
			first start;
			then action p send new Ping() via m.response;
			then done;
		}
	}
}
