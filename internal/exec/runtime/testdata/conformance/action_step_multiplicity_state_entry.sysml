package test {
	private import ScalarValues::*;

	state def Machine {
		attribute entries : Integer = 0;

		entry; then active;
		state active {
			entry action enter[2] { assign entries := entries + 1; }
		}
	}
}
