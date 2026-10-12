package test {
	private import ScalarValues::*;
	private import SI::*;

	action def Reader {
		out attribute got : Integer;
		first start;
		then action r accept after 5 [s];
		then action keep { assign got := 1; }
		then done;
	}

	action def Mid {
		out attribute got : Integer;
		first start;
		then perform action reader : Reader;
		then action keep { assign got := reader.got; }
		then done;
	}

	action waitsForClock {
		attribute received : Integer = 0;
		attribute instant : Real = 0.0;
		first start;
		then action m : Mid;
		then action take { assign received := m.got; }
		then action record { assign instant := localClock.currentTime; }
		then done;
	}
}
