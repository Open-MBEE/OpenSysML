package test {
	private import ScalarValues::*;

	// Two flows may name one source pin by its inherited and by its redefining name;
	// each flow declaration is a transfer of its own, so the pin's one write is staged
	// twice for a target begun afterwards: each of the target's two performances takes one.
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
