package test {
	private import ScalarValues::*;

	state machine {
		attribute source : Integer := 1;
		attribute seen1 : Integer := 0;
		attribute seen2 : Integer := 0;
		attribute visits : Integer := 0;

		entry; then active;
		state active {
			attribute live : Integer = source * 2;
			entry {
				if visits == 0 {
					assign seen1 := live;
				} else {
					assign seen2 := live;
				}
				assign visits := visits + 1;
			}
		}
		state done;
		transition first active accept when visits == 1 do assign source := 5 then active;
		transition first active accept when visits == 2 then done;
	}
}
