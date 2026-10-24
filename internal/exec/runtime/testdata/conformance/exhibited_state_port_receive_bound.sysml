// A state definition owns a receive port named unlike the part's (`port inbox`); the
// part exhibiting it binds that port to its own `command` (`bind m.inbox = command`).
// The machine's `accept … via inbox` receives what the part's interface delivers to
// `command`, and its reply leaves through the bound `response`.
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
		bind m.inbox = command;
		bind m.response = response;
		interface wire connect response to command;
		perform action kick {
			first start;
			then action p send new Ping() via response;
			then done;
		}
	}
}
