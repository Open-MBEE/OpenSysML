package test {
	private import ScalarValues::*;

	// The value a source's declaration gives its out pin (`out value = 5`) streams as the
	// performance begins, so the consumer forked beside it reads 5.
	action stream {
		attribute total : Integer = 0;

		fork split;
		action producer {
			out value : Integer = 5;
			action wait { assign total := total + 0; }
			succession first start then wait;
			succession first wait then done;
		}
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
