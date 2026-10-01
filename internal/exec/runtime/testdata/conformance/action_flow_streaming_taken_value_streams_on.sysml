package test {
	private import ScalarValues::*;

	// A value waiting at a pin when its node's performance begins streams on along the
	// flows out of that pin at once: the relay's input, taken as the relay begins, reaches
	// the consumer beside it before the relay completes.
	action stream {
		attribute total : Integer = 0;

		action producer {
			out value : Integer;
			action write { assign value := 5; }
			succession first start then write;
			succession first write then done;
		}

		fork split;

		action relay {
			in value : Integer;
			action hold { assign total := total + 0; }
			action linger { assign total := total + 0; }
			succession first start then hold;
			succession first hold then linger;
			succession first linger then done;
		}

		action consumer {
			in value : Integer;
			action take { assign total := total + value; }
			succession first start then take;
			succession first take then done;
		}

		join sync;

		succession first start then producer;
		succession first producer then split;
		succession first split then relay;
		succession first split then consumer;
		succession first relay then sync;
		succession first consumer then sync;
		succession first sync then done;

		flow producer.value to relay.value;
		flow relay.value to consumer.value;
	}
}
