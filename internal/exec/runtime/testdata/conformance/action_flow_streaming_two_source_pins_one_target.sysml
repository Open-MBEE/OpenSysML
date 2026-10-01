package test {
	private import ScalarValues::*;

	// Two pins of one source stream to one pin of a target begun after it: each pin's
	// latest write waits in its own place, so the target's two performances, one per
	// enabling succession, take the source's low then its high.
	action stream {
		attribute total : Integer = 0;

		action producer {
			out low : Integer[0..1] = ();
			out high : Integer[0..1] = ();
			action low1 { assign low := 1; }
			action high10 { assign high := 10; }
			succession first start then low1;
			succession first low1 then high10;
			succession first high10 then done;
		}

		action consumer {
			in v : Integer[0..1];
			action add { assign total := total + v; }
			succession first start then add;
			succession first add then done;
		}

		fork twice;
		merge again;
		succession first start then producer;
		succession first producer then twice;
		succession first twice then again;
		succession first twice then again;
		succession first again then consumer;
		succession first consumer then done;

		flow producer.low to consumer.v;
		flow producer.high to consumer.v;
	}
}
