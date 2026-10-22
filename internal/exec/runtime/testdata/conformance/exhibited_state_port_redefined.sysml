// A state definition owns `port response`; the exhibit usage redefines it as the
// exhibiting part's port (`:>> response = Tug::response`). The machine's
// `send … via response` leaves through the part's port the redefinition names.
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
		exhibit state m : Responder {
			:>> response = Tug::response;
		}
		interface wire connect response to command;
		perform action kick {
			first start;
			then action p send new Ping() via response;
			then done;
		}
	}
}
