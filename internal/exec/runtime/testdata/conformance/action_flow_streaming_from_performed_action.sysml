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
	// written: each of 1, 2, 3 waits for a consumer performance of its own, so total = 6.
	action stream {
		attribute total : Integer = 0;
		attribute reads : Integer = 0;

		action producer : Producer;
		merge again;
		action consumer {
			in value : Integer;
			assign total := total + value;
			assign reads := reads + 1;
		}
		decide more;

		succession first start then producer;
		succession first producer then again;
		succession first again then consumer;
		succession first consumer then more;
		succession first more if reads < 3 then again;
		succession first more if reads >= 3 then done;

		flow producer.value to consumer.value;
	}
}
