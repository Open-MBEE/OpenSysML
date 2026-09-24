package test {
	private import ScalarValues::*;

	action communicate {
		attribute result : Integer = 0;
		port outPort;
		port inPort;
		connect outPort to inPort;

		first start;
		action sender {
			send 100 via outPort;
		}
		action receiver accept msg : Integer via inPort;
		action processor {
			assign result := msg;
		}
		done;
		succession first start then sender;
		succession first sender then receiver;
		succession first receiver then processor;
		succession first processor then done;
	}
}
