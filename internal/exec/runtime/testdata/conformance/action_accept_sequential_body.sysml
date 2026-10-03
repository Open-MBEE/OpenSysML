package test {
	private import ScalarValues::*;

	action acceptsInSequentialBody {
		attribute total : Integer = 0;
		attribute seen : Integer = 0;
		first start;
		fork split;
		action reader {
			loop {
				action step {
					accept a : Integer;
					assign total := total + a;
				}
				assign seen := total;
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
		succession first split then reader;
		succession first split then gate1;
		succession first gate1 then gate2;
		succession first gate2 then gate3;
		succession first gate3 then sender;
		succession first reader then sync;
		succession first sender then sync;
		succession first sync then done;
	}
}
