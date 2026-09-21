package test {
	private import ScalarValues::*;

	// A flow may name a source pin by its inherited name or by the redefining one;
	// both name one pin, so its one write to a target begun afterwards is staged once:
	// the target's first performance takes it and its second finds nothing waiting.
	action def Producer {
		out result : Integer[0..1] = ();
	}

	action stream {
		attribute received : Integer = 0;

		action producer : Producer {
			out value : Integer[0..1] redefines result;
			action write { assign value := 5; }
			succession first start then write;
			succession first write then done;
		}

		action consumer {
			in v : Integer[0..1];
			action count { assign received := received + SequenceFunctions::size(v); }
			succession first start then count;
			succession first count then done;
		}

		fork twice;
		merge again;
		succession first start then producer;
		succession first producer then twice;
		succession first twice then again;
		succession first twice then again;
		succession first again then consumer;
		succession first consumer then done;

		flow producer.result to consumer.v;
		flow producer.value to consumer.v;
	}
}
