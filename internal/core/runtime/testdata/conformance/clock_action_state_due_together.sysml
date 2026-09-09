// An action token and a state transition due at one instant of the shared clock:
// which runs first is a due-order choice, not an order the library fixes. Under
// the default policy the machine, created after the action, runs first and the
// action reads the lamp lit; under `declared` the action reads it still dark.
package test {
	private import SI::*;
	private import ScalarValues::*;

	part def Beacon {
		attribute lit : Boolean = false;

		exhibit state blinking {
			entry; then dark;

			state dark {
				accept after 5 [s] then shining;
			}
			state shining {
				entry assign lit := true;
			}
		}
	}

	part beacon : Beacon;

	action watcher {
		attribute armed : Boolean = false;
		attribute sawLit : Boolean = false;

		first start;
		then action arm assign armed := beacon.lit == false;
		then action wait accept after 5 [s];
		then action look assign sawLit := beacon.lit;
		then done;
	}
}
