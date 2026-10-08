package test {
	private import ScalarValues::*;

	action acceptsInBranch {
		attribute received : Integer = 0;
		first start;
		fork split;
		action reader {
			if true {
				accept thenValue : Integer;
				assign received := thenValue;
			} else {
				accept skippedThen : Integer;
			}
			then if false {
				accept skippedElse : Integer;
			} else {
				accept elseValue : Integer;
				assign received := received + elseValue;
			}
		}
		action gate1;
		action gate2;
		action gate3;
		action sender { send 3; send 4; }
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
