package test {
	private import ScalarValues::*;

	// A value taken as a performance begins streams on while that performance is already
	// ongoing: carried through the echo beside it and back to another pin of the relay,
	// it reaches the relay that just began rather than waiting for a later one.
	action stream {
		attribute total : Integer = 0;

		action producer {
			out value : Integer;
			action write { assign value := 5; }
			succession first start then write;
			succession first write then done;
		}

		fork split;

		action echo {
			in value : Integer;
			action hold { assign total := total + 0; }
			action linger { assign total := total + 0; }
			action wait { assign total := total + 0; }
			action rest { assign total := total + 0; }
			succession first start then hold;
			succession first hold then linger;
			succession first linger then wait;
			succession first wait then rest;
			succession first rest then done;
		}

		action relay {
			in value : Integer;
			in back : Integer;
			action take { assign total := total + back; }
			succession first start then take;
			succession first take then done;
		}

		join sync;

		succession first start then split;
		succession first split then echo;
		succession first split then producer;
		succession first producer then relay;
		succession first echo then sync;
		succession first relay then sync;
		succession first sync then done;

		flow producer.value to relay.value;
		flow relay.value to echo.value;
		flow echo.value to relay.back;
	}
}
