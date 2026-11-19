// An `accept` in a calculation body waits as an action's does; where nothing can
// post the message the wait is reported rather than skipped.
package test {
	private import ScalarValues::*;

	item def Ping;

	calc def Waits {
		in n : Integer = 3;
		accept p : Ping;
		return : Integer = n;
	}
}
