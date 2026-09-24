// Each object holds a distinct occurrence of the same exhibited state machine.
package test {
	private import ScalarValues::*;

	item def Tick;

	state def Counting {
		attribute count : Integer = 0;

		entry; then waiting;
		state waiting;
		accept Tick then counted;
		state counted {
			entry action increment {
				assign count := count + 1;
			}
		}
	}

	part def Counter {
		exhibit state modes : Counting;
	}
}
