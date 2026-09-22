package test {
	private import ScalarValues::*;

	// The payload an accept node binds is the node's own output pin: a flow out
	// of `receiver.msg` delivers the accepted value to the pin it names, so a
	// consumer reads it through its input rather than through the shared name.
	attribute def Reading { attribute level : Integer; }
	attribute def Calibrated :> Reading;

	action communicator {
		attribute total : Integer = 0;

		first start;

		action sender {
			send new Calibrated(level = 8) to receiver;
		}

		action receiver accept msg : Reading;

		action consumer {
			in value : Reading;
			assign total := value.level * 2;
		}

		done;

		flow receiver.msg to consumer.value;
		succession first start then sender;
		succession first sender then receiver;
		succession first receiver then consumer;
		succession first consumer then done;
	}
}
