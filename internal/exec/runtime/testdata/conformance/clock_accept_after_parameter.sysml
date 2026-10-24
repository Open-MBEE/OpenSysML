// `accept after howLong` where `howLong` is an `in` parameter of the action
// definition: the parameter is a duration-valued expression like any other, read
// in the performance's scope when the accept is armed, so the bound value times
// the wait — whether bound by the performing node, bound to a literal in the
// definition, or copied into an owned attribute.
package test {
	private import ScalarValues::*;
	private import SI::*;
	private import ISQ::*;

	action def Wait {
		in howLong : DurationValue;
		out finished : Boolean = false;
		first start;
		then action w accept after howLong;
		then assign finished := true;
		then done;
	}

	action def WaitLiteral {
		in howLong : DurationValue = 2.0 [s];
		out finished : Boolean = false;
		first start;
		then action w accept after howLong;
		then assign finished := true;
		then done;
	}

	action def WaitCopy {
		in howLong : DurationValue;
		attribute delay : DurationValue = howLong;
		out finished : Boolean = false;
		first start;
		then action w accept after delay;
		then assign finished := true;
		then done;
	}

	action timed {
		attribute dur : DurationValue = 5.0 [s];
		out at1 : Real = 0.0;
		out at2 : Real = 0.0;
		out at3 : Real = 0.0;
		first start;
		then action bound : Wait { in howLong = dur; }
		then action mark1 assign at1 := localClock.currentTime;
		then action literal : WaitLiteral;
		then action mark2 assign at2 := localClock.currentTime;
		then action copied : WaitCopy { in howLong = dur; }
		then action mark3 assign at3 := localClock.currentTime;
		then done;
	}
}
