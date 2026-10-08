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
				then assign total := total + local;
			}
			then if source == 3 {
				attribute branch : Integer = source;
				assign source := 6;
				then assign total := total + branch;
			}
			then for i in 1..2 {
				attribute each : Integer = source + i;
				assign source := source + 1;
				then assign total := total + each;
			}
		}
		then done;
	}
}
