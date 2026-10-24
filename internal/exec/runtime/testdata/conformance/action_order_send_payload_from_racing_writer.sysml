package test {
	private import ScalarValues::*;

	// Two branches between one fork and one join each write `x` once; a send after
	// the join carries `x` as its payload, an accept binds the message, and `total`
	// is computed from the payload: the transfer orders the accept after the
	// send, so what the accept bound stands on the open order of the writes.
	item def Reading { attribute level : Integer; }

	action relay {
		attribute x : Integer = 0;
		attribute total : Integer = 0;
		attribute count : Integer = 0;

		first start;
		fork split;
		action a { assign x := 1; }
		action b { assign x := 2; }
		join sync;
		action sender { send new Reading(level = x) to receiver; }
		action receiver accept msg : Reading;
		action keep { assign total := msg.level + 10; assign count := 1; }
		done;

		succession first start then split;
		succession first split then a;
		succession first split then b;
		succession first a then sync;
		succession first b then sync;
		succession first sync then sender;
		succession first sender then receiver;
		succession first receiver then keep;
		succession first keep then done;
	}
}
