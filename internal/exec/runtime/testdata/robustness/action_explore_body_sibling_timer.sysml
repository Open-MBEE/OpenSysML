package test {
	private import ScalarValues::*;
	private import SI::*;
	action def Reader {
		out attribute timed : Integer = 0;
		first start;
		then action timer accept after 10 [s];
		then action noteTimer { assign timed := 1; }
		then done;
	}
	action def Main {
		out attribute timed : Integer = 0;
		first start;
		then perform action reader : Reader;
		then action collect { assign timed := reader.timed; }
		then done;
	}
	// gate's timer starts at the same instant as reader's, so mark reads main.timed at 1 s.
	action def SiblingTimer {
		out attribute timed : Integer = 0;
		out attribute gated : Integer = 0;
		first start;
		fork split;
		perform action main : Main;
		action gate accept after 1 [s];
		action mark { assign gated := main.timed + 1; }
		join meet;
		action collect { assign timed := main.timed; }
		done;
		succession first start then split;
		succession first split then main;
		succession first split then gate;
		succession first gate then mark;
		succession first mark then meet;
		succession first main then meet;
		succession first meet then collect;
		succession first collect then done;
	}
}
