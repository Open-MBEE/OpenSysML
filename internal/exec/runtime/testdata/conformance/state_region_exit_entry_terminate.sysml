package test {
	private import ScalarValues::*;

	attribute def Go;

	state Machine {
		attribute entered : Integer = 0;
		attribute childEntered : Integer = 0;

		entry; then running;
		state running parallel {
			state left {
				entry; then idle;
				state idle;
				transition first idle accept Go then target;
			}
			state right {
				entry; then parked;
				state parked;
			}
		}
		state target {
			entry action boot { assign entered := entered + 1; }
			transition boot then route;
			junction route;
			transition first route then stop;
			action stop terminate;
			state child { entry { assign childEntered := 1; } }
		}
	}
}
