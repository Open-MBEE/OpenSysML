package test {
	private import ScalarValues::*;

	// A value streamed while no performance of its target is under way waits at the
	// target's pin: the consumer begins after the producer is over and reads 5.
	action stream {
		attribute total : Integer = 0;

		action producer {
			out value : Integer;
			assign value := 5;
		}

		action consumer {
			in value : Integer;
			action take { assign total := total + value; }
			succession first start then take;
			succession first take then done;
		}

		succession first start then producer;
		succession first producer then consumer;
		succession first consumer then done;

		flow producer.value to consumer.value;
	}
}
