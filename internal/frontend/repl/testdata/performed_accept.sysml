// Objects whose performed actions park at an accept, so %send has actions rather
// than machines to reach: a top-level perform, a perform nested in another, one
// parked at two accepts for the same signal, a part performing an action beside
// an exhibited machine, and an idle sibling whose %state session runs the others
// when the clock advances.
package Q {
	private import ScalarValues::*;

	attribute def Go { attribute n : Integer; }
	attribute def Halt;
	attribute def Reset;

	action def Main {
		out total : Integer = 0;
		first start;
		then action w1 accept g : Go;
		then action a1 assign total := total + g.n;
		then done;
	}

	action def Outer {
		out heard : Integer = 0;
		first start;
		then action inner {
			first start;
			then action listen accept h : Halt;
			then action mark assign heard := 1;
			then done;
		}
		then done;
	}

	action def Split {
		out taken : Integer = 0;
		first start;
		then fork split;
			then left;
			then right;
		action left accept l : Halt;
		then action l1 assign taken := taken + 1;
		then meet;
		action right accept r : Halt;
		then action r1 assign taken := taken + 10;
		then meet;
		join meet;
		then done;
	}

	state def Idle {
		entry; then a;
		state a;
		transition a_b first a accept Halt then b;
		state b;
	}

	part def PD {
		perform action main : Main;
	}

	part def Nest {
		perform action outer : Outer;
	}

	part def Fork {
		perform action split : Split;
	}

	part def Both {
		exhibit state idle : Idle;
		perform action main : Main;
	}

	part def Other {
		exhibit state s : Idle;
	}

	part pd : PD;
	part nest : Nest;
	part twin : Fork;
	part both : Both;
	part other : Other;
}
