package test {
	private import ScalarValues::*;

	action def Reader {
		out attribute got : Integer;
		first start;
		then action r accept n : Integer;
		then action keep { assign got := r.n; }
		then done;
	}

	action def Mid {
		out attribute got : Integer;
		first start;
		then perform action reader : Reader;
		then action keep { assign got := reader.got; }
		then done;
	}

	action nested {
		attribute received : Integer = 0;
		first start;
		fork split;
		action m : Mid { assign received := m.got; }
		action gate1;
		action gate2;
		action gate3;
		action sender { send 7; }
		join sync;
		done;
		succession first start then split;
		succession first split then m;
		succession first split then gate1;
		succession first gate1 then gate2;
		succession first gate2 then gate3;
		succession first gate3 then sender;
		succession first m then sync;
		succession first sender then sync;
		succession first sync then done;
	}
}
