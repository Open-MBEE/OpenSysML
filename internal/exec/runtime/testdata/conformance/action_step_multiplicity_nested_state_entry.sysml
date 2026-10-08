package test {
	private import ScalarValues::*;

	state def Machine {
		attribute counter : Integer = 0;

		entry; then active;
		state active {
			entry action begin {
				first start then tick;
				action tick[2] { assign counter := counter + 1; }
				then done;
			}
		}
	}
}
