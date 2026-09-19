// A redefinition of a Clock's currentTime is the same feature under another
// name: reading it, bare in the clock's own body or through a part holding
// the clock, is the run's shared clock, and it advances with the timed waits
// of the run rather than holding a stored value.
package test {
	private import ScalarValues::*;
	private import Clocks::*;

	part def Chrono :> Clock {
		attribute now :>> currentTime;
		attribute stamped : Real = -1.0;

		action stamp {
			action mark {
				assign stamped := now;
			}
			first start then mark;
			first mark then done;
		}
	}

	part chrono : Chrono;

	action supervisor {
		attribute before : Real = -1.0;
		attribute later : Real = -1.0;
		attribute stamped : Real = -1.0;

		first start;
		then action read1 {
			assign before := chrono.now;
		}
		then action delay accept after 1.5 [SI::s];
		then perform action run ::> chrono.stamp;
		then action read2 {
			assign later := chrono.now;
			assign stamped := chrono.stamped;
		}
		then done;
	}
}
