package P {
	port def CommandPort;

	part def Console {
		port command : CommandPort;
	}

	part def Unit {
		port command : ~CommandPort;
	}

	part def Site {
		part console : Console;
		part units : Unit[2];

		connect console.command to units.command;
	}

	state def Radio {
		port command : CommandPort;

		entry; then start;
		state start;
		state sent;
		transition go first start do send Ping() via command then sent;
	}
}
