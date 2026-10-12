package BodyFeatureBindings {
	private import ScalarValues::*;

	action def Work {
		attribute source : Integer := 1;
		attribute live : Integer = source * 2;
		attribute fallback : Integer default source;

		action nested {
			attribute nestedLive : Integer = source + 1;
		}

		if source > 0 {
			attribute blockLive : Integer = source * 3;
			assign source := 2;
		}
	}

	state def Machine {
		attribute source : Integer := 1;
		attribute live : Integer = source * 2;

		state active {
			attribute stateLive : Integer = source + 1;
			entry {
				attribute entryLive : Integer = source * 3;
			}
			do {
				attribute doLive : Integer = source * 4;
			}
			exit {
				attribute exitLive : Integer = source * 5;
			}
		}
	}
}
