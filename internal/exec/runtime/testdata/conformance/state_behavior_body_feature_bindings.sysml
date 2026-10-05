package test {
	private import ScalarValues::*;

	state Machine {
		attribute source : Integer := 1;
		attribute log : Integer := 0;
		attribute complete : Boolean := false;

		entry; then active;
		state active {
			entry {
				attribute entryValue : Integer = source;
				assign source := 2;
				then assign log := entryValue;
			}
			do {
				attribute doValue : Integer = source;
				assign source := 3;
				then assign log := log * 10 + doValue;
				then assign complete := true;
			}
			exit {
				attribute exitValue : Integer = source;
				assign source := 4;
				then assign log := log * 10 + exitValue;
			}
		}
		state done;
		transition first active accept when complete then done;
	}
}
