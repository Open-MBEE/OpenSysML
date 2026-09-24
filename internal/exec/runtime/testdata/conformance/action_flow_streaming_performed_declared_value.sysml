package test {
	private import ScalarValues::*;

	action def Producer {
		out value : Integer = 5;
		attribute i : Integer = 0;

		action wait { assign i := i + 1; }

		succession first start then wait;
		succession first wait then done;
	}

	// The value a performed action's declaration gives its out pin streams from the
	// performing node as the performance begins, so the consumer forked beside it
	// reads 5 whichever of the two the fork begins first.
	action stream {
		attribute total : Integer = 0;

		fork split;
		action producer : Producer;
		action consumer {
			in value : Integer;
			action take { assign total := total + value; }
			succession first start then take;
			succession first take then done;
		}
		join sync;

		succession first start then split;
		succession first split then producer;
		succession first split then consumer;
		succession first producer then sync;
		succession first consumer then sync;
		succession first sync then done;

		flow producer.value to consumer.value;
	}
}
