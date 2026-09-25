package test {
	private import ScalarValues::*;

	action def Producer {
		out value : Integer;
		attribute i : Integer = 0;

		action emit { assign i := i + 1; assign value := i; }
		decide again;

		succession first start then emit;
		succession first emit then again;
		succession first again if i < 3 then emit;
		succession first again if i >= 3 then done;
	}

	// The outputs an action performed by a node writes stream from the node as they are
	// written; a consumer begun afterwards reads the pin's value, the last of them.
	action stream {
		attribute total : Integer = 0;

		action producer : Producer;
		action consumer {
			in value : Integer;
			assign total := total + value;
		}

		succession first start then producer;
		succession first producer then consumer;
		succession first consumer then done;

		flow producer.value to consumer.value;
	}
}
