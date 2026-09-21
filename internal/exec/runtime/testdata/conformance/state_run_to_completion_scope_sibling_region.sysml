// A scoped false redefinition on work leaves the sibling region's Go
// transition free while work's entry cascade is held.
package test {
	private import ScalarValues::*;

	item def Ping;
	item def Start;
	item def Go;

	state def Machine parallel {
		attribute order : Integer = 0;
		state left {
			entry; then prep;
			state prep;
			state work {
				ref :>> runToCompletionScope = self;
				state inner {
					entry action { assign order := order * 10 + 1; }
				}
				entry { send Go to Machine; }
				entry; then inner;
			}
			transition first prep accept Start then work;
		}
		state right {
			entry; then a;
			state a;
			state b {
				entry action { assign order := order * 10 + 2; }
			}
			transition first a accept Go then b;
		}
	}
}
