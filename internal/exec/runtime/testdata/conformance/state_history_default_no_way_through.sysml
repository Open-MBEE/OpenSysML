package Test {
	private import ScalarValues::*;
	attribute def Go;
	attribute def Continue;

	state Machine {
		attribute log : String = "";

		entry; then idle;
		state idle {
			entry action mark { assign log := "started"; }
		}
		state done;
		state work {
			entry; then w1;
			state w1;
			history resume;
			junction dead;
			transition first resume then dead;
			transition first dead if false then w1;
		}

		transition first idle accept Go then resume;
		transition first idle accept Continue do assign log := log + " continued" then done;
	}
}
