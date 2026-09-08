package Test {
	private import ScalarValues::*;
	action addFive {
		attribute result : Integer = 0;
		first start;
		action inner {
			assign result := result + 5;
		}
		done;
		succession first start then inner;
		succession first inner then done;
	}
	action noStart {
		attribute result : Integer = 0;
	}
	action race {
		attribute winner : Integer = 0;
		first start;
		fork split;
		action left {
			assign winner := 1;
		}
		action right {
			assign winner := 2;
		}
		join sync;
		done;
		succession first start then split;
		succession first split then left;
		succession first split then right;
		succession first left then sync;
		succession first right then sync;
		succession first sync then done;
	}
	state Machine {
		entry; then init;
		state init;
		state Running;

		succession first init then Running;
		succession first Running then done;
	}
}
