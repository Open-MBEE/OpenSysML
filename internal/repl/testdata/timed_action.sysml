// An action parked on the clock and a state machine accepting the signal it
// sends after the wait, so %advance has both debuggers to move at once.
package Timed {
	private import ScalarValues::*;

	item def Ping;

	action pinger {
		attribute count : Integer = 0;
		first start;
		then action wait accept after 5 [SI::s];
		then action tick assign count := count + 1;
		then send new Ping() to listener;
		then done;
	}

	state listener {
		entry; then idle;
		state idle;
		transition idle_pinged first idle accept Ping then pinged;
		state pinged;
	}
}
