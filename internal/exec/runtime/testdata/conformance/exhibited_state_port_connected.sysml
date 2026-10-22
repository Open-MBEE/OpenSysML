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
		attribute replies : Integer = 0;
		entry; then idle;
		state idle;
		state replied;
		transition first idle accept Ping via command
			do send new Pong() via response
			then replied;
		transition first replied accept Pong via command
			do assign replies := replies + 1
			then idle;
	}

	part def Tug {
		port command : CommandPort;
		exhibit state m : Responder;
		interface wire connect m.response to command;
		perform action kick {
			first start;
			then action p send new Ping() via m.response;
			then done;
		}
	}
}
