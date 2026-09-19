package test {
	private import ScalarValues::*;

	// A named action body parameter is still the loop's body: the name only scopes
	// the members it declares, so the loop runs instead of failing as unsupported.
	action charger {
		attribute battery : Integer = 97;

		first start;

		then loop action charging {
			attribute step : Integer = 1;
			assign battery := battery + step;
		} until battery >= 100;

		// An empty body parameter runs as an empty body.
		then loop action { } until true;

		then done;
	}
}
