package test {
	private import ScalarValues::*;

	// A `succession flow` carries the value its source holds once the source completes, and
	// the target begins no sooner: the consumer reads the last value, 3, three times.
	action handoff {
		attribute total : Integer = 0;
		attribute i : Integer = 0;
		attribute reads : Integer = 0;

		action producer {
			out value : Integer;

			action emit { assign i := i + 1; assign value := i; }
			decide again;

			succession first start then emit;
			succession first emit then again;
			succession first again if i < 3 then emit;
			succession first again if i >= 3 then done;
		}

		action consumer {
			in value : Integer;

			action prime { assign reads := 0; }
			action take { assign total := total + value; assign reads := reads + 1; }
			decide more;

			succession first start then prime;
			succession first prime then take;
			succession first take then more;
			succession first more if reads < 3 then take;
			succession first more if reads >= 3 then done;
		}

		succession first start then producer;
		succession first consumer then done;

		succession flow producer.value to consumer.value;
	}
}
