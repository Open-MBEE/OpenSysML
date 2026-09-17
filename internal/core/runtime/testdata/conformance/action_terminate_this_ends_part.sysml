// `terminate this;` in an action a part performs ends the part (SysML v2 §7.17.10):
// the action's own performance ends at that statement with the writes made so far,
// and so does every other behavior the part performs — the machine it exhibits is
// terminated where it stands, its do behavior abandoned, no state exited.
package test {
	private import ScalarValues::*;

	attribute def Never;

	part def Probe {
		attribute seen : Integer = 0;
		attribute beats : Integer = 0;
		attribute exits : Integer = 0;

		exhibit state life {
			entry; then alive;
			state alive {
				do action beat {
					first start;
					then action one assign beats := 1;
					then action wait accept Never;
					then action two assign beats := 2;
					then done;
				}
				exit action leave assign exits := 1;
			}
		}

		perform action selfDestruct {
			action step {
				assign this.seen := 1;
				terminate this;
				assign this.seen := 2;
			}
			first step;
		}
	}
}
