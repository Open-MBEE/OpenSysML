package test {
	private import ScalarValues::*;

	// A plain `flow` streams (SysML v2 §7.16): each value the producer writes while the
	// consumer, forked beside it, is being performed reaches it at once, so total = 1+2+3.
	action stream {
		attribute total : Integer = 0;
		attribute i : Integer = 0;
		attribute reads : Integer = 0;

		fork split;

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
