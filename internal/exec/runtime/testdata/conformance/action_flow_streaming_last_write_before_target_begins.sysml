package test {
	private import ScalarValues::*;

	// A source that writes its pin several times before any performance of the target is
	// under way leaves the target the pin's value: its latest write. The list is built up
	// element by element and the consumer, begun after the producer, reads all three.
	action stream {
		attribute count : Integer = 0;

		action producer {
			out list : Integer[0..*] nonunique = ();
			action one { assign list := (list, 1); }
			action two { assign list := (list, 2); }
			action three { assign list := (list, 3); }
			succession first start then one;
			succession first one then two;
			succession first two then three;
			succession first three then done;
		}

		action consumer {
			in list : Integer[0..*] nonunique;
			action take { assign count := SequenceFunctions::size(list); }
			succession first start then take;
			succession first take then done;
		}

		succession first start then producer;
		succession first producer then consumer;
		succession first consumer then done;

		flow producer.list to consumer.list;
	}
}
