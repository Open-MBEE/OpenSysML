package test {
	private import ScalarValues::*;

	part def Host {
		attribute count : Integer = 0;
		exhibit state life {
			entry; then idle;
			state idle;
		}
		perform action run[2] {
			action step { assign count := count + 1; }
			first step;
		}
	}
}
