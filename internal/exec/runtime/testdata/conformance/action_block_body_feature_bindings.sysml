package test {
	private import ScalarValues::*;

	action run {
		attribute source : Integer := 0;
		out attribute total : Integer := 0;

		first start;
		then action blocks {
			while source < 3 {
				attribute local : Integer = source;
				assign source := source + 1;
				assign total := total + local;
			}
			if source == 3 {
				attribute branch : Integer = source;
				assign source := 6;
				assign total := total + branch;
			}
			for i in 1..2 {
				attribute each : Integer = source + i;
				assign source := source + 1;
				assign total := total + each;
			}
		}
		then done;
	}
}
