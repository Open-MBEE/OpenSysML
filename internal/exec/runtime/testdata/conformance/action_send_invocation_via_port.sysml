package test {
	private import ScalarValues::*;

	item def Data {
		attribute value : Integer;
	}

	// `send new Data(reading) via commPort` sends the signal the constructor names,
	// carrying its argument, and the accept on the far end of the port binds that
	// payload to its parameter.
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

		action receive accept msg : Data via groundPort;

		action record {
			assign received := msg.value;
		}

		done;

		succession first start then transmit;
		succession first transmit then receive;
		succession first receive then record;
		succession first record then done;
	}
}
