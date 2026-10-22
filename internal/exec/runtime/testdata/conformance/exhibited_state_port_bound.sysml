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
		port response : ~CommandPort;
		exhibit state m : Responder;
		exhibit state n : Counter;
		bind m.response = response;
		interface wire connect response to command;
		perform action kick {
			first start;
			then action p send new Ping() via response;
			then done;
		}
	}
}
