// A state definition owns `port response`; the part exhibiting it binds that
// port to its own (`bind m.response = response`), whose interface loops back to
// its `command` port. The machine's `send … via response` leaves through the
// bound port and arrives where the interface delivers, as `accept … via command`
// receives there.
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
		port response : ~CommandPort;
		exhibit state m : Responder;
		bind m.response = response;
		interface wire connect response to command;
		perform action kick {
			first start;
			then action p send new Ping() via response;
			then done;
		}
	}
}
