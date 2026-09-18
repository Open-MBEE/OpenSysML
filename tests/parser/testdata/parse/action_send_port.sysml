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
		action notifier {
			send 100 to reader;
		}
		action reader accept msg : Integer via inPort;
		done;
		succession first start then sender;
		succession first sender then notifier;
		succession first notifier then reader;
		succession first reader then done;
	}
}
