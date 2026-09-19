package test {
	private import ScalarValues::*;

	// The standard action-body spelling: a succession to a loop node written as an
	// action body parameter, whose `until` condition is tested after each
	// iteration, and a succession to the action's final node written `then done`.
	action charger {
		attribute battery : Integer = 97;

		first start;

		then loop action {
			assign battery := battery + 1;
		} until battery >= 100;

		then done;
	}
}
