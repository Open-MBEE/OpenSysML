// A held entry may coexist with a sibling change transition, but the
// transition inside the held scope fires only after its source is entered.
package test {
	private import ScalarValues::*;

	item def Start;

	state Machine parallel {
		attribute flag : Boolean = false;
		attribute order : Integer = 0;
		state left {
			entry; then prep;
			state prep;
			state work {
				ref :>> runToCompletionScope = self;
				state step {
					entry action { assign order := order * 10 + 1; }
				}
				state done {
					entry action { assign order := order * 10 + 3; }
				}
				entry action { assign flag := true; } then step;
				transition first step accept when flag then done;
			}
			transition first prep accept Start then work;
		}
		state right {
			entry; then a;
			state a;
			state b {
				entry action { assign order := order * 10 + 2; }
			}
			transition first a accept when flag then b;
		}
	}
}
