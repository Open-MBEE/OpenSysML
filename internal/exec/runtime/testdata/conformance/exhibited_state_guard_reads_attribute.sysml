// A guard reads the value an earlier entry action wrote to the state
// performance occurrence.
package test {
	private import ScalarValues::*;

	state def Modes {
		attribute attempts : Integer = 0;

		entry; then armed;
		state armed {
			entry action increment {
				assign attempts := attempts + 1;
			}
		}
		state ready;

		transition arm_to_ready first armed if attempts > 0 then ready;
	}

	part def Controller {
		exhibit state modes : Modes;
	}
}
