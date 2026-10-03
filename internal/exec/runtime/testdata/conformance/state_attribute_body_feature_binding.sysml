package test {
	private import ScalarValues::*;

	state Machine {
		attribute source : Integer := 1;
		attribute observed : Integer := 0;

		entry; then active;
		state active {
			attribute live : Integer = source * 2;
			entry {
				assign source := 5;
				assign observed := live;
			}
		}
		state done;
		transition first active accept when observed > 0 then done;
	}
}
