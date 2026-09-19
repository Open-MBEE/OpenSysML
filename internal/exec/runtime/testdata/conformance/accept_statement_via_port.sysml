package test {
	private import ScalarValues::*;

	item def Data {
		attribute value : Integer;
	}

	// An accept node standing as a statement of an action body, with no name of
	// its own, still binds the payload that reaches the port it accepts through.
	action telemetry {
		attribute reading : Integer = 7;
		attribute received : Integer = 0;
		port commPort;
		port groundPort;
		connect commPort to groundPort;

		first start;

		action transmit {
			send new Data(reading) via commPort;
		}

		then accept msg : Data via groundPort;

		then action record {
			assign received := msg.value;
		}

		done;

		succession first start then transmit;
		succession first record then done;
	}
}
