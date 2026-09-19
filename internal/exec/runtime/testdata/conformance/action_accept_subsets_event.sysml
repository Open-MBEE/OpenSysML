package test {
	private import ScalarValues::*;

	item def Ping;

	// An accept whose payload subsets a declared event waits for an occurrence of
	// that one event: `send shutDown` sends the event feature, and the accept
	// subsetting it consumes the message it makes.
	action controller {
		attribute stopped : Integer = 0;
		item shutDown : Ping;

		first start;

		action finish {
			send shutDown to interrupt;
		}

		action interrupt accept :> shutDown;

		action record {
			assign stopped := 1;
		}

		done;

		succession first start then finish;
		succession first finish then interrupt;
		succession first interrupt then record;
		succession first record then done;
	}
}
