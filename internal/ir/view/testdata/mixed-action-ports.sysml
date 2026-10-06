package MixedActionPorts {
	private import OpenSysMLRenderings::*;

	item def Request;
	port def Signal;

	part def Source {
		port output : Signal;
	}

	part def Sink {
		port input : Signal;
	}

	part def Monitor {
		port unused : Signal;
	}

	part def Plant {
		part source : Source;
		part sink : Sink;
		part monitor : Monitor;
		connection supply : Connection connect source.output to sink.input;
	}

	connection def Connection {
		end part a;
		end part b;
	}

	action def Check {
		in request : Request;
	}

	view mixedView {
		expose Plant;
		expose Check;
		render asMixedDiagram;
	}
}
