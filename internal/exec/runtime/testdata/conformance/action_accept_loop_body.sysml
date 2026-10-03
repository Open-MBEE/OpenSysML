package test {
	private import ScalarValues::*;

	action acceptsInLoop {
		attribute total : Integer = 0;
		first start;
		fork split;
		action sum {
			loop {
				accept n : Integer;
				assign total := total + n;
			} until total >= 10;
		}
		action gate1;
		action gate2;
		action gate3;
		action sender {
			send 4;
			send 6;
		}
		join sync;
		done;
		succession first start then split;
		succession first split then sum;
		succession first split then gate1;
		succession first gate1 then gate2;
		succession first gate2 then gate3;
		succession first gate3 then sender;
		succession first sum then sync;
		succession first sender then sync;
		succession first sync then done;
	}
}
