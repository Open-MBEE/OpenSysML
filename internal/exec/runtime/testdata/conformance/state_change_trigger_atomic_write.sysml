package test {
	private import ScalarValues::*;
	attribute def Go;

	part def P {
		attribute a : Integer = 0;
		attribute b : Integer;
		bind b = a;

		exhibit state machine {
			entry; then outer;
			state outer {
				entry; then waiting;
				state waiting;
				state writing {
					entry action update { assign a := 1; }
				}
				transition first waiting accept Go then writing;
				transition first writing then waiting;
			}
			state done;

			transition first outer accept when a != b then done;
		}
	}
}
